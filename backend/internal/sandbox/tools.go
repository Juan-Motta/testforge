package sandbox

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"regexp"
	"strings"
	"time"

	"testforge/internal/llm"
)

const (
	toolRemember = "remember"
	toolRecall   = "recall"
)

func defaultRand() float64 { return rand.Float64() }

var memoryTools = []llm.ToolDef{
	{
		Name:        toolRemember,
		Description: "Save a fact about the current user to long-term memory, to use in later conversations.",
		Parameters: map[string]any{"type": "object", "required": []any{"fact"}, "additionalProperties": false,
			"properties": map[string]any{"fact": map[string]any{"type": "string", "description": "One self-contained fact."}}},
	},
	{
		Name:        toolRecall,
		Description: "Search the current user's long-term memory. An empty query returns the most recent facts.",
		Parameters: map[string]any{"type": "object", "required": []any{"query"}, "additionalProperties": false,
			"properties": map[string]any{"query": map[string]any{"type": "string", "description": "Words to look for."}}},
	},
}

func toolDefs(a Agent) []llm.ToolDef {
	defs := make([]llm.ToolDef, 0, len(a.Tools)+len(memoryTools))
	for _, t := range a.Tools {
		defs = append(defs, llm.ToolDef{Name: t.Name, Description: t.Description, Parameters: t.Parameters})
	}
	if a.Memory.LongTerm {
		defs = append(defs, memoryTools...)
	}
	return defs
}

// toolResult is what a call returns to the model.
type toolResult struct {
	text  string
	error bool
}

func errorResult(msg string) toolResult {
	b, _ := json.Marshal(map[string]string{"error": msg})
	return toolResult{text: string(b), error: true}
}

// runMock executes a mock tool: latency, then a failure with probability
// ErrorRate, else the rendered response.
func runMock(ctx context.Context, t Tool, args map[string]any, random func() float64) (toolResult, error) {
	if t.LatencyMs > 0 {
		timer := time.NewTimer(time.Duration(t.LatencyMs) * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return toolResult{}, ctx.Err()
		case <-timer.C:
		}
	}
	if t.ErrorRate > 0 && random() < t.ErrorRate {
		msg := t.ErrorMessage
		if msg == "" {
			msg = defaultToolError
		}
		return errorResult(msg), nil
	}
	return toolResult{text: render(t.Response, args)}, nil
}

var argRef = regexp.MustCompile(`\{\{\s*args\.([a-zA-Z0-9_\-]+(?:\.[a-zA-Z0-9_\-]+)*)\s*\}\}`)

// render replaces {{args.<path>}} with the argument: strings as is, other
// values as JSON, missing or null as empty.
func render(tmpl string, args map[string]any) string {
	return argRef.ReplaceAllStringFunc(tmpl, func(ref string) string {
		path := argRef.FindStringSubmatch(ref)[1]
		var v any = args
		for _, part := range strings.Split(path, ".") {
			m, ok := v.(map[string]any)
			if !ok {
				return ""
			}
			if v, ok = m[part]; !ok {
				return ""
			}
		}
		switch x := v.(type) {
		case nil:
			return ""
		case string:
			return x
		default:
			b, _ := json.Marshal(x)
			return string(b)
		}
	})
}

// parseArgs decodes the model's arguments; empty means no arguments.
func parseArgs(raw string) (map[string]any, bool) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, true
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, false
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, true
}
