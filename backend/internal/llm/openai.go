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

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string  `json:"content"`
			Refusal *string `json:"refusal"`
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
	target := c.url
	if c.azure {
		target = fmt.Sprintf(c.url, url.PathEscape(req.Model))
	} else {
		body["model"] = req.Model
	}
	raw, _ := json.Marshal(body)
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(raw))
	if err != nil {
		return Response{}, err
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
		return Response{}, fmt.Errorf("calling the model: %w", err)
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
		return Response{}, fmt.Errorf("the model provider answered %s", msg)
	}
	if len(out.Choices) == 0 {
		return Response{}, fmt.Errorf("the model returned no answer")
	}
	choice := out.Choices[0]
	if choice.Message.Refusal != nil && *choice.Message.Refusal != "" {
		return Response{}, fmt.Errorf("the model refused: %s", *choice.Message.Refusal)
	}
	if choice.FinishReason == "length" {
		return Response{}, ErrTruncated
	}
	return Response{JSON: choice.Message.Content, InputTokens: out.Usage.PromptTokens, OutputTokens: out.Usage.CompletionTokens}, nil
}
