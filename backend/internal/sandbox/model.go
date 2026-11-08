// Package sandbox hosts test agents inside TestForge: a system prompt and a
// model on the workspace's own OpenAI or Azure OpenAI connection, optional
// conversation and long-term memory, and mock tools with fault injection.
// Linked to a regular agent, scenarios and runs test them like any other.
package sandbox

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"testforge/internal/connection"
)

const (
	ModalityText  = "text"
	ModalityVoice = "voice"

	maxTools          = 20
	maxParamsBytes    = 20000
	maxResponseChars  = 20000
	maxLatencyMs      = 30000
	defaultToolError  = "The tool failed."
	defaultUserKey    = "anonymous"
	historyLimit      = 50
	maxToolRounds     = 8
	maxMemoriesListed = 500
	maxRecallResults  = 20
)

var (
	uuidPattern     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	toolNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
	reservedTools   = []string{toolRemember, toolRecall}
	// toolKinds are the connection kinds whose clients implement tool calling.
	toolKinds = []connection.Kind{connection.KindOpenAI, connection.KindAzureOpenAI}
)

// Tool is a mock tool: the model calls it and gets a canned response.
type Tool struct {
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Parameters   map[string]any `json:"parameters"`
	Response     string         `json:"response"`
	LatencyMs    int            `json:"latencyMs"`
	ErrorRate    float64        `json:"errorRate"`
	ErrorMessage string         `json:"errorMessage"`
}

// Memory says which memories the agent keeps.
type Memory struct {
	Conversation bool `json:"conversation"`
	LongTerm     bool `json:"longTerm"`
}

// Input is the create and update request.
type Input struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Modality     string   `json:"modality"`
	ConnectionID string   `json:"connectionId"`
	Model        string   `json:"model"`
	SystemPrompt string   `json:"systemPrompt"`
	Temperature  *float64 `json:"temperature"`
	Memory       Memory   `json:"memory"`
	Tools        []Tool   `json:"tools"`
}

// Agent is a stored sandbox agent. ConnectionID is empty once its connection was deleted.
type Agent struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Modality     string    `json:"modality"`
	ConnectionID string    `json:"connectionId"`
	Model        string    `json:"model"`
	SystemPrompt string    `json:"systemPrompt"`
	Temperature  *float64  `json:"temperature"`
	Memory       Memory    `json:"memory"`
	Tools        []Tool    `json:"tools"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// ChatRequest is one user message.
type ChatRequest struct {
	Message  string  `json:"message"`
	ThreadID *string `json:"threadId"`
	UserID   *string `json:"userId"`
}

// ToolCallLog reports one tool call made while answering.
type ToolCallLog struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result"`
	Error     bool   `json:"error"`
	LatencyMs int    `json:"latencyMs"`
}

// Usage is the tokens spent answering one message.
type Usage struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
}

// ChatReply is the agent's answer.
type ChatReply struct {
	Message   string        `json:"message"`
	ThreadID  string        `json:"threadId"`
	ToolCalls []ToolCallLog `json:"toolCalls"`
	Usage     Usage         `json:"usage"`
}

// StoredMemory is one long-term memory.
type StoredMemory struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

func setOnce(fields map[string]string, k, msg string) {
	if _, ok := fields[k]; !ok {
		fields[k] = msg
	}
}

func textLen(fields map[string]string, key, value string, min, max int) string {
	value = strings.TrimSpace(value)
	n := len([]rune(value))
	switch {
	case n < min && min == 1:
		setOnce(fields, key, "Required.")
	case n < min || n > max:
		setOnce(fields, key, fmt.Sprintf("Use %d to %d characters.", min, max))
	}
	return value
}

// validateDraft checks everything but the connection, which needs the database.
func validateDraft(in *Input, fields map[string]string) {
	in.Name = textLen(fields, "name", in.Name, 1, 100)
	if n := len([]rune(in.Description)); n > 2000 {
		setOnce(fields, "description", "Use at most 2000 characters.")
	}
	in.Description = strings.TrimSpace(in.Description)
	switch in.Modality {
	case ModalityText:
	case ModalityVoice:
		setOnce(fields, "modality", "Voice sandbox agents are not available yet.")
	default:
		setOnce(fields, "modality", "Choose text or voice.")
	}
	in.Model = textLen(fields, "model", in.Model, 1, 200)
	if strings.TrimSpace(in.SystemPrompt) == "" {
		setOnce(fields, "systemPrompt", "Required.")
	} else if len([]rune(in.SystemPrompt)) > 20000 {
		setOnce(fields, "systemPrompt", "Use 1 to 20000 characters.")
	}
	if t := in.Temperature; t != nil && (*t < 0 || *t > 2) {
		setOnce(fields, "temperature", "Use a value from 0 to 2.")
	}
	if in.Tools == nil {
		in.Tools = []Tool{}
	}
	if len(in.Tools) > maxTools {
		setOnce(fields, "tools", fmt.Sprintf("Add at most %d tools.", maxTools))
	}
	seen := map[string]bool{}
	for i := range in.Tools {
		validateTool(&in.Tools[i], fmt.Sprintf("tools.%d.", i), fields)
		name := strings.ToLower(in.Tools[i].Name)
		if seen[name] {
			setOnce(fields, fmt.Sprintf("tools.%d.name", i), "Another tool already uses this name.")
		}
		seen[name] = true
	}
}

// validateTool checks one tool; prefix is the field path, e.g. "tools.0.".
func validateTool(t *Tool, prefix string, fields map[string]string) {
	t.Name = strings.TrimSpace(t.Name)
	switch {
	case !toolNamePattern.MatchString(t.Name):
		setOnce(fields, prefix+"name", "Use 1 to 64 letters, digits, _ or -.")
	case slices.Contains(reservedTools, strings.ToLower(t.Name)):
		setOnce(fields, prefix+"name", fmt.Sprintf("%q is reserved for long-term memory.", t.Name))
	}
	t.Description = textLen(fields, prefix+"description", t.Description, 1, 1000)
	if t.Parameters == nil {
		setOnce(fields, prefix+"parameters", "Required: a JSON Schema whose type is object.")
	} else if typ, _ := t.Parameters["type"].(string); typ != "object" {
		setOnce(fields, prefix+"parameters", `The schema's "type" must be "object".`)
	} else if raw, err := json.Marshal(t.Parameters); err != nil || len(raw) > maxParamsBytes {
		setOnce(fields, prefix+"parameters", fmt.Sprintf("Keep the schema under %d bytes.", maxParamsBytes))
	}
	if len([]rune(t.Response)) > maxResponseChars {
		setOnce(fields, prefix+"response", fmt.Sprintf("Use at most %d characters.", maxResponseChars))
	}
	if t.LatencyMs < 0 || t.LatencyMs > maxLatencyMs {
		setOnce(fields, prefix+"latencyMs", fmt.Sprintf("Use 0 to %d milliseconds.", maxLatencyMs))
	}
	if t.ErrorRate < 0 || t.ErrorRate > 1 {
		setOnce(fields, prefix+"errorRate", "Use a value from 0 to 1.")
	}
	if len([]rune(t.ErrorMessage)) > 500 {
		setOnce(fields, prefix+"errorMessage", "Use at most 500 characters.")
	}
	t.ErrorMessage = strings.TrimSpace(t.ErrorMessage)
}
