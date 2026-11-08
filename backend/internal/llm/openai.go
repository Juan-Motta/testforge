package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// openAI talks to the OpenAI Chat Completions API, or Azure OpenAI's
// deployment-scoped variant of it.
type openAI struct {
	http  *http.Client
	url   string
	key   string
	org   string
	azure bool
}

type wireToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content   string         `json:"content"`
			Refusal   *string        `json:"refusal"`
			ToolCalls []wireToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *openAI) Complete(ctx context.Context, req Request) (Response, error) {
	body := map[string]any{
		"messages": []map[string]string{{"role": "system", "content": req.System}, {"role": "user", "content": req.Prompt}},
		"response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{
			"name": req.SchemaName, "strict": true, "schema": req.Schema,
		}},
	}
	out, err := c.post(ctx, req.Model, body)
	if err != nil {
		return Response{}, err
	}
	return Response{JSON: out.Choices[0].Message.Content, InputTokens: out.Usage.PromptTokens, OutputTokens: out.Usage.CompletionTokens}, nil
}

// Chat runs one Chat Completions turn with tools.
func (c *openAI) Chat(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	messages := []map[string]any{{"role": "system", "content": req.System}}
	for _, m := range req.Messages {
		wire := map[string]any{"role": m.Role, "content": m.Content}
		switch {
		case m.Role == RoleTool:
			wire["tool_call_id"] = m.ToolCallID
		case len(m.ToolCalls) > 0:
			calls := make([]map[string]any, len(m.ToolCalls))
			for i, tc := range m.ToolCalls {
				calls[i] = map[string]any{"id": tc.ID, "type": "function",
					"function": map[string]any{"name": tc.Name, "arguments": tc.Arguments}}
			}
			wire["tool_calls"] = calls
			if m.Content == "" {
				wire["content"] = nil
			}
		}
		messages = append(messages, wire)
	}
	body := map[string]any{"messages": messages}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, len(req.Tools))
		for i, t := range req.Tools {
			tools[i] = map[string]any{"type": "function", "function": map[string]any{
				"name": t.Name, "description": t.Description, "parameters": t.Parameters,
			}}
		}
		body["tools"] = tools
	}
	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	out, err := c.post(ctx, req.Model, body)
	if err != nil {
		return ChatResponse{}, err
	}
	msg := out.Choices[0].Message
	res := ChatResponse{Text: msg.Content, InputTokens: out.Usage.PromptTokens, OutputTokens: out.Usage.CompletionTokens}
	for _, tc := range msg.ToolCalls {
		res.ToolCalls = append(res.ToolCalls, ToolCall{ID: tc.ID, Name: tc.Function.Name, Arguments: tc.Function.Arguments})
	}
	return res, nil
}

// post sends a Chat Completions request and returns a response with at least
// one choice that was neither refused nor cut off.
func (c *openAI) post(ctx context.Context, model string, body map[string]any) (chatResponse, error) {
	target := c.url
	if c.azure {
		target = fmt.Sprintf(c.url, url.PathEscape(model))
	} else {
		body["model"] = model
	}
	raw, _ := json.Marshal(body)
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(raw))
	if err != nil {
		return chatResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.azure {
		httpReq.Header.Set("Api-Key", c.key)
	} else {
		httpReq.Header.Set("Authorization", "Bearer "+c.key)
		if c.org != "" {
			httpReq.Header.Set("OpenAI-Organization", c.org)
		}
	}
	res, err := c.http.Do(httpReq)
	if err != nil {
		return chatResponse{}, fmt.Errorf("calling the model: %w", err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	var out chatResponse
	_ = json.Unmarshal(data, &out)
	if res.StatusCode >= 300 {
		msg := fmt.Sprintf("HTTP %d", res.StatusCode)
		if out.Error != nil && out.Error.Message != "" {
			msg += ": " + strings.ReplaceAll(out.Error.Message, c.key, "••••")
		}
		return chatResponse{}, fmt.Errorf("the model provider answered %s", msg)
	}
	if len(out.Choices) == 0 {
		return chatResponse{}, fmt.Errorf("the model returned no answer")
	}
	choice := out.Choices[0]
	if choice.Message.Refusal != nil && *choice.Message.Refusal != "" {
		return chatResponse{}, fmt.Errorf("the model refused: %s", *choice.Message.Refusal)
	}
	if choice.FinishReason == "length" {
		return chatResponse{}, ErrTruncated
	}
	return out, nil
}
