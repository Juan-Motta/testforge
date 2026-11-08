package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"testforge/internal/agent"
	"testforge/internal/apikey"
	"testforge/internal/authn"
	"testforge/internal/platform/apperr"
	"testforge/internal/workspace"
)

const linkBody = `{"message":"{{message}}","threadId":"{{run.thread_id}}","userId":"{{var.userId}}"}`

// Link creates a regular text agent that calls this sandbox agent's chat
// endpoint with a new workspace API key, so scenarios can test it. Owners and
// admins with a session (it creates an API key).
func (s *Service) Link(ctx context.Context, p authn.Principal, ws workspace.Workspace, id string) (agent.Agent, error) {
	if p.Method != authn.MethodSession {
		return agent.Agent{}, apperr.Forbidden("Sign in with a session to link sandbox agents; API keys cannot create API keys.")
	}
	if !ws.Role.CanAdminister() {
		return agent.Agent{}, apperr.Forbidden("Only owners and admins can link sandbox agents.")
	}
	sb, err := s.Get(ctx, ws, id)
	if err != nil {
		return agent.Agent{}, err
	}
	name, err := s.freeAgentName(ctx, ws, sb.Name+" (sandbox)")
	if err != nil {
		return agent.Agent{}, err
	}
	key, err := s.keys.Create(ctx, p, ws, apikey.CreateInput{Name: truncate("Sandbox: "+sb.Name, 100)})
	if err != nil {
		return agent.Agent{}, err
	}
	body := linkBody
	created, err := s.agents.Create(ctx, p, ws, agent.Input{
		Name:        name,
		Description: truncate(strings.TrimSpace("Sandbox agent. "+sb.Description), 2000),
		Modality:    agent.ModalityText,
		Transport: agent.Transport{
			URL:          fmt.Sprintf("%s/api/v1/workspaces/%s/sandbox-agents/%s/chat", s.opts.PublicAPIURL, ws.Slug, sb.ID),
			Query:        []agent.KV{},
			Headers:      []agent.KV{{Key: "Authorization", Value: "Bearer {{secret.api_key}}"}},
			Body:         &body,
			InitMessages: []string{},
		},
		Codec:     agent.Codec{Type: "http_json", Options: map[string]string{"replyPath": "message"}},
		Variables: []agent.KV{{Key: "userId", Value: "test-user"}},
		Secrets:   []agent.SecretInput{{Name: "api_key", Value: key.Secret}},
	})
	if err != nil {
		// Do not leave an unused key behind.
		if rerr := s.keys.Revoke(context.WithoutCancel(ctx), p, ws, key.ID); rerr != nil {
			slog.Error("revoke sandbox link key", "key", key.ID, "err", rerr)
		}
		if ae, ok := apperr.As(err); ok && ae.Kind == apperr.KindInvalid {
			if msg, bad := ae.Fields["transport.url"]; bad {
				return agent.Agent{}, apperr.Invalid(map[string]string{"publicApiUrl": "PUBLIC_API_URL is not a valid agent URL here: " + msg})
			}
		}
		return agent.Agent{}, err
	}
	return created, nil
}

// freeAgentName returns base, or base with " 2", " 3"… when taken.
func (s *Service) freeAgentName(ctx context.Context, ws workspace.Workspace, base string) (string, error) {
	base = truncate(base, 95)
	rows, err := s.pool.Query(ctx, `SELECT lower(name) FROM agents WHERE workspace_id = $1 AND lower(name) LIKE lower($2) || '%'`,
		ws.ID, escapeLike(base))
	if err != nil {
		return "", fmt.Errorf("agent names: %w", err)
	}
	defer rows.Close()
	taken := map[string]bool{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return "", err
		}
		taken[n] = true
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	name := base
	for i := 2; taken[strings.ToLower(name)]; i++ {
		name = fmt.Sprintf("%s %d", base, i)
	}
	return name, nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
