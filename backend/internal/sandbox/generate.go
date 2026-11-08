package sandbox

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"testforge/internal/llm"
	"testforge/internal/platform/apperr"
	"testforge/internal/workspace"
)

// GenerateRequest asks a model to draft a sandbox agent or one tool.
type GenerateRequest struct {
	Kind         string        `json:"kind"`
	ConnectionID string        `json:"connectionId"`
	Model        string        `json:"model"`
	Description  string        `json:"description"`
	Agent        *AgentContext `json:"agent"`
}

// AgentContext describes the agent a generated tool belongs to.
type AgentContext struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	SystemPrompt string   `json:"systemPrompt"`
	ToolNames    []string `json:"toolNames"`
}

// AgentDraft is a generated, unsaved sandbox agent (connection and model are the caller's).
type AgentDraft struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	SystemPrompt string `json:"systemPrompt"`
	Memory       Memory `json:"memory"`
	Tools        []Tool `json:"tools"`
}

// Draft is the generation result: Agent for kind agent, Tool for kind tool.
type Draft struct {
	Agent *AgentDraft `json:"agent"`
	Tool  *Tool       `json:"tool"`
}

const (
	generateAgent = "agent"
	generateTool  = "tool"
)

var draftToolSchema = map[string]any{
	"type": "object", "additionalProperties": false,
	"required": []any{"name", "description", "parametersJson", "response", "latencyMs", "errorRate", "errorMessage"},
	"properties": map[string]any{
		"name":           map[string]any{"type": "string", "description": "snake_case, letters, digits, _ or -, at most 64 characters."},
		"description":    map[string]any{"type": "string", "description": "When the agent should call it and what it returns."},
		"parametersJson": map[string]any{"type": "string", "description": `A JSON Schema, serialized as a JSON string, whose "type" is "object".`},
		"response":       map[string]any{"type": "string", "description": "Realistic mock result (usually JSON) the tool returns; may use {{args.<name>}}."},
		"latencyMs":      map[string]any{"type": "integer", "description": "Simulated latency, 0 to 30000."},
		"errorRate":      map[string]any{"type": "number", "description": "Probability from 0 to 1 that a call fails."},
		"errorMessage":   map[string]any{"type": "string", "description": "Error returned on failure; empty for the default."},
	},
}

var draftAgentSchema = map[string]any{
	"type": "object", "additionalProperties": false,
	"required": []any{"name", "description", "systemPrompt", "memory", "tools"},
	"properties": map[string]any{
		"name":         map[string]any{"type": "string"},
		"description":  map[string]any{"type": "string"},
		"systemPrompt": map[string]any{"type": "string"},
		"memory": map[string]any{
			"type": "object", "additionalProperties": false, "required": []any{"conversation", "longTerm"},
			"properties": map[string]any{"conversation": map[string]any{"type": "boolean"}, "longTerm": map[string]any{"type": "boolean"}},
		},
		"tools": map[string]any{"type": "array", "items": draftToolSchema},
	},
}

const generatorSystem = `You design test agents for TestForge, a platform that tests voice and text AI agents.
A sandbox agent is a system prompt plus mock tools; TestForge runs the model and answers tool calls with each tool's canned response.

Rules:
- Write the system prompt, descriptions and mock responses in the language of the user's description.
- The system prompt states the agent's role, goals, tone, what it must not do, and when to use each tool.
- Tools are realistic for the domain. Names are snake_case. parametersJson is a JSON Schema serialized as a string, with "type":"object", "properties" and "required".
- A mock response is a realistic result, usually JSON, and may insert arguments with {{args.<name>}}.
- latencyMs and errorRate are 0 unless the description asks for slowness or failures; errorMessage is empty unless needed.
- Turn on conversation memory unless asked otherwise; turn on long-term memory only when the agent should remember users across conversations.
- Do not create tools named remember or recall; TestForge adds them for long-term memory.
- At most 20 tools.`

type draftToolWire struct {
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	ParametersJSON string  `json:"parametersJson"`
	Response       string  `json:"response"`
	LatencyMs      int     `json:"latencyMs"`
	ErrorRate      float64 `json:"errorRate"`
	ErrorMessage   string  `json:"errorMessage"`
}

type draftAgentWire struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	SystemPrompt string          `json:"systemPrompt"`
	Memory       Memory          `json:"memory"`
	Tools        []draftToolWire `json:"tools"`
}

// Generate drafts a sandbox agent or one tool; nothing is saved. Any role but viewer.
func (s *Service) Generate(ctx context.Context, ws workspace.Workspace, req GenerateRequest) (Draft, error) {
	if err := canAuthor(ws); err != nil {
		return Draft{}, err
	}
	fields := map[string]string{}
	if req.Kind != generateAgent && req.Kind != generateTool {
		fields["kind"] = "Choose agent or tool."
	}
	req.Model = textLen(fields, "model", req.Model, 1, 200)
	req.Description = textLen(fields, "description", req.Description, 1, 4000)
	if !uuidPattern.MatchString(req.ConnectionID) {
		fields["connectionId"] = "Choose a connection of this workspace."
	}
	if err := apperr.Invalid(fields); err != nil {
		return Draft{}, err
	}
	creds, err := s.conns.Credentials(ctx, ws.ID, req.ConnectionID)
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return Draft{}, apperr.Invalid(map[string]string{"connectionId": "Choose a connection of this workspace."})
		}
		return Draft{}, err
	}
	client, err := llm.New(creds, s.opts.AllowPrivate)
	if err != nil {
		return Draft{}, apperr.Invalid(map[string]string{"connectionId": "Choose a connection with the llm capability."})
	}

	prompt, schema, name := "Design a sandbox agent for this description:\n\n"+req.Description, draftAgentSchema, "sandbox_agent"
	if req.Kind == generateTool {
		prompt, schema, name = toolPrompt(req), draftToolSchema, "sandbox_tool"
	}
	res, err := client.Complete(ctx, llm.Request{
		Model: req.Model, System: generatorSystem, Prompt: prompt, SchemaName: name, Schema: schema, MaxTokens: 8000,
	})
	if err != nil {
		if ctx.Err() != nil {
			return Draft{}, ctx.Err()
		}
		return Draft{}, apperr.Upstream(err.Error())
	}
	if req.Kind == generateTool {
		return decodeToolDraft(res.JSON, req.Agent)
	}
	return decodeAgentDraft(res.JSON)
}

func toolPrompt(req GenerateRequest) string {
	var b strings.Builder
	b.WriteString("Design one mock tool for this description:\n\n")
	b.WriteString(req.Description)
	if a := req.Agent; a != nil {
		fmt.Fprintf(&b, "\n\nThe tool belongs to this agent.\nName: %s\nDescription: %s\nSystem prompt:\n%s\n",
			truncate(a.Name, 100), truncate(a.Description, 2000), truncate(a.SystemPrompt, 8000))
		names := a.ToolNames[:min(len(a.ToolNames), maxTools)]
		for i, n := range names {
			names[i] = truncate(n, 64)
		}
		if len(names) > 0 {
			fmt.Fprintf(&b, "Existing tools (use a different name): %s\n", strings.Join(names, ", "))
		}
	}
	return b.String()
}

func invalidDraft(fields map[string]string) error {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + ": " + fields[k]
	}
	return apperr.Upstream("The model returned an invalid draft: " + strings.Join(parts, "; "))
}

func (w draftToolWire) tool(prefix string, fields map[string]string) Tool {
	t := Tool{Name: w.Name, Description: w.Description, Response: w.Response, LatencyMs: w.LatencyMs,
		ErrorRate: w.ErrorRate, ErrorMessage: w.ErrorMessage}
	if err := json.Unmarshal([]byte(w.ParametersJSON), &t.Parameters); err != nil {
		setOnce(fields, prefix+"parameters", "Not valid JSON.")
		t.Parameters = map[string]any{"type": "object"}
	}
	return t
}

func decodeToolDraft(raw string, ctxAgent *AgentContext) (Draft, error) {
	var w draftToolWire
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		return Draft{}, apperr.Upstream("The model returned an invalid draft: not valid JSON.")
	}
	fields := map[string]string{}
	t := w.tool("", fields)
	validateTool(&t, "", fields)
	if ctxAgent != nil && slices.ContainsFunc(ctxAgent.ToolNames, func(n string) bool { return strings.EqualFold(n, t.Name) }) {
		setOnce(fields, "name", "Another tool already uses this name.")
	}
	if len(fields) > 0 {
		return Draft{}, invalidDraft(fields)
	}
	return Draft{Tool: &t}, nil
}

func decodeAgentDraft(raw string) (Draft, error) {
	var w draftAgentWire
	if err := json.Unmarshal([]byte(raw), &w); err != nil {
		return Draft{}, apperr.Upstream("The model returned an invalid draft: not valid JSON.")
	}
	fields := map[string]string{}
	in := Input{Name: w.Name, Description: w.Description, Modality: ModalityText, Model: "draft",
		SystemPrompt: w.SystemPrompt, Memory: w.Memory, Tools: make([]Tool, len(w.Tools))}
	for i, tw := range w.Tools {
		in.Tools[i] = tw.tool(fmt.Sprintf("tools.%d.", i), fields)
	}
	validateDraft(&in, fields)
	if len(fields) > 0 {
		return Draft{}, invalidDraft(fields)
	}
	return Draft{Agent: &AgentDraft{Name: in.Name, Description: in.Description, SystemPrompt: in.SystemPrompt,
		Memory: in.Memory, Tools: in.Tools}}, nil
}
