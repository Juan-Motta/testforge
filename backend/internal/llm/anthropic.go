package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"

	"testforge/internal/connection"
)

// claude uses the official Anthropic Go SDK with the egress-guarded HTTP client.
type claude struct {
	client anthropic.Client
}

func newAnthropic(c connection.Credentials, httpClient *http.Client) *claude {
	return &claude{client: anthropic.NewClient(
		option.WithAPIKey(c.Secret),
		option.WithBaseURL(c.Config["baseUrl"]),
		option.WithHTTPClient(httpClient),
		option.WithMaxRetries(2),
	)}
}

func (c *claude) Complete(ctx context.Context, req Request) (Response, error) {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
	}
	msg, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     req.Model,
		MaxTokens: int64(maxTokens),
		System:    []anthropic.TextBlockParam{{Text: req.System}},
		Messages:  []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock(req.Prompt))},
		// Structured outputs constrain the answer to the decision schema.
		OutputConfig: anthropic.OutputConfigParam{Format: anthropic.JSONOutputFormatParam{Schema: req.Schema}},
	})
	if err != nil {
		var apiErr *anthropic.Error
		if errors.As(err, &apiErr) {
			return Response{}, fmt.Errorf("the model provider answered HTTP %d", apiErr.StatusCode)
		}
		return Response{}, fmt.Errorf("calling the model: %w", err)
	}
	switch msg.StopReason {
	case anthropic.StopReasonMaxTokens:
		return Response{}, ErrTruncated
	case anthropic.StopReasonRefusal:
		return Response{}, errors.New("the model refused to continue the conversation")
	}
	text := ""
	for _, block := range msg.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			text += t.Text
		}
	}
	return Response{JSON: text, InputTokens: int(msg.Usage.InputTokens), OutputTokens: int(msg.Usage.OutputTokens)}, nil
}
