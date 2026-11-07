// Package llm asks a language model for a structured JSON answer through the
// workspace's own provider connection (OpenAI, Azure OpenAI or Anthropic).
package llm

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"testforge/internal/connection"
	"testforge/internal/platform/egress"
)

const requestTimeout = 60 * time.Second

// Request is one structured completion.
type Request struct {
	Model  string
	System string
	// Prompt is the single user message (the conversation so far, rendered as text).
	Prompt string
	// SchemaName and Schema describe the JSON object the model must return.
	SchemaName string
	Schema     map[string]any
	MaxTokens  int
}

// Response is the model's JSON text and token usage.
type Response struct {
	JSON         string
	InputTokens  int
	OutputTokens int
}

// Client completes structured requests.
type Client interface {
	Complete(ctx context.Context, req Request) (Response, error)
}

// New builds a client for a connection with the llm capability.
func New(c connection.Credentials, allowPrivate bool) (Client, error) {
	httpClient := egress.NewClient(allowPrivate, requestTimeout)
	switch c.Kind {
	case connection.KindOpenAI:
		return &openAI{http: httpClient, url: c.Config["baseUrl"] + "/chat/completions", key: c.Secret, org: c.Config["organization"]}, nil
	case connection.KindAzureOpenAI:
		return &openAI{http: httpClient, azure: true, key: c.Secret,
			url: c.Config["endpoint"] + "/openai/deployments/%s/chat/completions?api-version=" + strings.ReplaceAll(url.QueryEscape(c.Config["apiVersion"]), "%", "%%")}, nil
	case connection.KindAnthropic:
		return newAnthropic(c, httpClient), nil
	}
	return nil, fmt.Errorf("%s connections cannot simulate users", c.Kind)
}

// ErrTruncated means the model stopped before finishing its JSON answer.
var ErrTruncated = errors.New("the model's answer was cut off")
