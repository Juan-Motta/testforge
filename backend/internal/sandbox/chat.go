package sandbox

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"testforge/internal/authn"
	"testforge/internal/llm"
	"testforge/internal/platform/apperr"
	"testforge/internal/workspace"
)

// chatter builds the tool-calling client of a connection; tests may replace it.
func (s *Service) chatter(ctx context.Context, ws workspace.Workspace, connID string) (llm.ToolChatter, error) {
	noConn := apperr.Invalid(map[string]string{"connectionId": "The sandbox agent needs an OpenAI or Azure OpenAI connection; choose one in its settings."})
	if connID == "" {
		return nil, noConn
	}
	creds, err := s.conns.Credentials(ctx, ws.ID, connID)
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return nil, noConn
		}
		return nil, err
	}
	client, err := llm.New(creds, s.opts.AllowPrivate)
	if err != nil {
		return nil, noConn
	}
	tc, ok := client.(llm.ToolChatter)
	if !ok {
		return nil, noConn
	}
	return tc, nil
}

// Chat answers one user message. Any role but viewer.
func (s *Service) Chat(ctx context.Context, p authn.Principal, ws workspace.Workspace, id string, req ChatRequest) (ChatReply, error) {
	if err := canAuthor(ws); err != nil {
		return ChatReply{}, err
	}
	fields := map[string]string{}
	if n := len([]rune(req.Message)); strings.TrimSpace(req.Message) == "" {
		fields["message"] = "Required."
	} else if n > 8000 {
		fields["message"] = "Use 1 to 8000 characters."
	}
	threadID := strings.TrimSpace(deref(req.ThreadID))
	userKey := strings.TrimSpace(deref(req.UserID))
	if len([]rune(threadID)) > 200 {
		fields["threadId"] = "Use at most 200 characters."
	}
	if len([]rune(userKey)) > 200 {
		fields["userId"] = "Use at most 200 characters."
	}
	if err := apperr.Invalid(fields); err != nil {
		return ChatReply{}, err
	}
	a, err := s.Get(ctx, ws, id)
	if err != nil {
		return ChatReply{}, err
	}
	client, err := s.chatter(ctx, ws, a.ConnectionID)
	if err != nil {
		return ChatReply{}, err
	}
	if threadID == "" {
		threadID = newThreadID()
	}
	if userKey == "" {
		userKey = defaultUserKey
	}

	var history []llm.Message
	if a.Memory.Conversation {
		if history, err = s.history(ctx, a.ID, threadID); err != nil {
			return ChatReply{}, err
		}
	}
	turn := []llm.Message{{Role: llm.RoleUser, Content: req.Message}}
	reply := ChatReply{ThreadID: threadID, ToolCalls: []ToolCallLog{}}
	defs := toolDefs(a)
	for round := 0; ; round++ {
		if round == maxToolRounds {
			return ChatReply{}, apperr.Upstream("The agent called tools too many times without answering.")
		}
		res, err := client.Chat(ctx, llm.ChatRequest{
			Model: a.Model, System: a.SystemPrompt, Messages: append(slices.Clip(history), turn...),
			Tools: defs, Temperature: a.Temperature,
		})
		if err != nil {
			if ctx.Err() != nil {
				return ChatReply{}, ctx.Err()
			}
			return ChatReply{}, apperr.Upstream(err.Error())
		}
		reply.Usage.InputTokens += res.InputTokens
		reply.Usage.OutputTokens += res.OutputTokens
		turn = append(turn, llm.Message{Role: llm.RoleAssistant, Content: res.Text, ToolCalls: res.ToolCalls})
		if len(res.ToolCalls) == 0 {
			reply.Message = res.Text
			break
		}
		for _, call := range res.ToolCalls {
			started := time.Now()
			out, err := s.runTool(ctx, a, userKey, call)
			if err != nil {
				return ChatReply{}, err
			}
			reply.ToolCalls = append(reply.ToolCalls, ToolCallLog{
				Name: call.Name, Arguments: call.Arguments, Result: out.text, Error: out.error,
				LatencyMs: int(time.Since(started).Milliseconds()),
			})
			turn = append(turn, llm.Message{Role: llm.RoleTool, Content: out.text, ToolCallID: call.ID})
		}
	}
	if a.Memory.Conversation {
		if err := s.saveTurn(ctx, a.ID, threadID, turn); err != nil {
			return ChatReply{}, err
		}
	}
	return reply, nil
}

func (s *Service) runTool(ctx context.Context, a Agent, userKey string, call llm.ToolCall) (toolResult, error) {
	args, ok := parseArgs(call.Arguments)
	if !ok {
		return errorResult("The arguments are not valid JSON."), nil
	}
	if a.Memory.LongTerm {
		switch call.Name {
		case toolRemember:
			return s.remember(ctx, a.ID, userKey, args)
		case toolRecall:
			return s.recall(ctx, a.ID, userKey, args)
		}
	}
	for _, t := range a.Tools {
		if t.Name == call.Name {
			return runMock(ctx, t, args, s.opts.Rand)
		}
	}
	return errorResult(fmt.Sprintf("Unknown tool %q.", call.Name)), nil
}

// history returns the thread's last messages, starting at a user message so
// no tool result is left without the call that asked for it.
func (s *Service) history(ctx context.Context, agentID, threadID string) ([]llm.Message, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT message FROM (SELECT id, message FROM sandbox_messages WHERE agent_id = $1 AND thread_id = $2
		   ORDER BY id DESC LIMIT $3) last ORDER BY id`, agentID, threadID, historyLimit)
	if err != nil {
		return nil, fmt.Errorf("load history: %w", err)
	}
	defer rows.Close()
	var out []llm.Message
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan history: %w", err)
		}
		var m llm.Message
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("decode history: %w", err)
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for len(out) > 0 && out[0].Role != llm.RoleUser {
		out = out[1:]
	}
	return out, nil
}

func (s *Service) saveTurn(ctx context.Context, agentID, threadID string, turn []llm.Message) error {
	batch := &pgx.Batch{}
	for _, m := range turn {
		raw, _ := json.Marshal(m)
		batch.Queue(`INSERT INTO sandbox_messages (agent_id, thread_id, message) VALUES ($1, $2, $3)`, agentID, threadID, raw)
	}
	// The agent may have been deleted meanwhile; its messages go with it.
	err := s.pool.SendBatch(context.WithoutCancel(ctx), batch).Close()
	if err != nil && !isForeignKeyViolation(err) {
		return fmt.Errorf("save conversation: %w", err)
	}
	return nil
}

func (s *Service) remember(ctx context.Context, agentID, userKey string, args map[string]any) (toolResult, error) {
	fact, _ := args["fact"].(string)
	fact = strings.TrimSpace(fact)
	if fact == "" || len([]rune(fact)) > 2000 {
		return errorResult("Send a fact of 1 to 2000 characters."), nil
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO sandbox_memories (agent_id, user_key, content) VALUES ($1, $2, $3)`,
		agentID, userKey, fact); err != nil && !isForeignKeyViolation(err) {
		return toolResult{}, fmt.Errorf("save memory: %w", err)
	}
	return toolResult{text: `{"saved":true}`}, nil
}

// recall returns the user's newest memories containing any query word.
func (s *Service) recall(ctx context.Context, agentID, userKey string, args map[string]any) (toolResult, error) {
	query, _ := args["query"].(string)
	var patterns []string
	for _, w := range strings.Fields(strings.ToLower(query)) {
		if len([]rune(w)) >= 2 {
			patterns = append(patterns, "%"+escapeLike(w)+"%")
		}
	}
	rows, err := s.pool.Query(ctx,
		`SELECT content FROM sandbox_memories WHERE agent_id = $1 AND user_key = $2
		   AND (cardinality($3::text[]) = 0 OR lower(content) LIKE ANY ($3::text[]))
		 ORDER BY created_at DESC LIMIT $4`, agentID, userKey, patterns, maxRecallResults)
	if err != nil {
		return toolResult{}, fmt.Errorf("recall: %w", err)
	}
	defer rows.Close()
	found := []string{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return toolResult{}, fmt.Errorf("recall: %w", err)
		}
		found = append(found, c)
	}
	if err := rows.Err(); err != nil {
		return toolResult{}, err
	}
	b, _ := json.Marshal(map[string]any{"memories": found})
	return toolResult{text: string(b)}, nil
}

// Memories lists long-term memories, newest first; userID filters by user.
func (s *Service) Memories(ctx context.Context, ws workspace.Workspace, id, userID string) ([]StoredMemory, error) {
	if _, err := s.Get(ctx, ws, id); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_key, content, created_at FROM sandbox_memories
		 WHERE agent_id = $1 AND ($2 = '' OR user_key = $2) ORDER BY created_at DESC LIMIT $3`, id, userID, maxMemoriesListed)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()
	out := []StoredMemory{}
	for rows.Next() {
		var m StoredMemory
		if err := rows.Scan(&m.ID, &m.UserID, &m.Content, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// DeleteMemory removes one long-term memory. Member or above.
func (s *Service) DeleteMemory(ctx context.Context, ws workspace.Workspace, id, memoryID string) error {
	if err := canAuthor(ws); err != nil {
		return err
	}
	if _, err := s.Get(ctx, ws, id); err != nil {
		return err
	}
	if !uuidPattern.MatchString(memoryID) {
		return apperr.NotFound("Memory not found.")
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM sandbox_memories WHERE agent_id = $1 AND id = $2`, id, memoryID)
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("Memory not found.")
	}
	return nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func newThreadID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return fmt.Sprintf("sbx-%x", b)
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func isForeignKeyViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	return errors.As(err, &pgErr) && pgErr.SQLState() == "23503"
}
