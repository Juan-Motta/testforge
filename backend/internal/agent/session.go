package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"testforge/internal/agent/codec"
	"testforge/internal/agent/tmpl"
	"testforge/internal/platform/egress"
)

// Session holds an agent's resolved templates for one conversation: a thread
// id, its secrets and the pre-session captures. The runner uses it to talk to
// the agent the same way the probe does.
type Session struct {
	agent          Agent
	vals           tmpl.Values
	allowPrivate   bool
	Redactor       *tmpl.Redactor
	TraceSessionID string
}

// OpenSession runs the pre-session call (if any) and resolves the trace session id.
func OpenSession(ctx context.Context, a Agent, secrets map[string]string, threadID string, allowPrivate bool, log func(kind, detail string)) (*Session, error) {
	vals := tmpl.Values{"run.thread_id": threadID}
	values := make([]string, 0, len(secrets))
	for k, v := range secrets {
		vals["secret."+k] = v
		values = append(values, v)
	}
	for _, v := range a.Variables {
		vals["var."+v.Key] = v.Value
	}
	s := &Session{agent: a, vals: vals, allowPrivate: allowPrivate, Redactor: tmpl.NewRedactor(values)}
	if log == nil {
		log = func(string, string) {}
	}
	if ps := a.Transport.PreSession; ps != nil {
		redacted := func(kind, detail string) { log(kind, s.Redactor.Redact(detail)) }
		if err := runPreSession(ctx, egress.NewClient(allowPrivate, connectTimeout), ps, vals, redacted); err != nil {
			return nil, fmt.Errorf("%s", s.Redactor.Redact(describeIfTransport(err)))
		}
	}
	trace, err := render(a.TraceSessionID, vals, tmpl.Raw)
	if err != nil {
		return nil, fmt.Errorf("resolving the trace session id: %w", err)
	}
	s.TraceSessionID = trace
	return s, nil
}

// SendText posts one message to a text agent and returns its reply.
func (s *Session) SendText(ctx context.Context, message string, timeout time.Duration, onEvent func(detail string)) (string, error) {
	tc, err := codec.NewText(s.agent.Codec.Type, s.agent.Codec.Options)
	if err != nil {
		return "", err
	}
	target, err := buildURL(s.agent.Transport.URL, s.agent.Transport.Query, s.vals)
	if err != nil {
		return "", err
	}
	vals := tmpl.Values{"message": message}
	for k, v := range s.vals {
		vals[k] = v
	}
	body := ""
	if s.agent.Transport.Body != nil {
		if body, err = render(*s.agent.Transport.Body, vals, tmpl.JSON); err != nil {
			return "", err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader([]byte(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", tc.Accept())
	if err := setHeaders(req.Header, s.agent.Transport.Headers, s.vals); err != nil {
		return "", err
	}
	res, err := egress.NewClient(s.allowPrivate, timeout).Do(req)
	if err != nil {
		return "", fmt.Errorf("%s", s.Redactor.Redact(describe(err)))
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(res.Body, 8192))
		return "", fmt.Errorf("the agent answered HTTP %d: %s", res.StatusCode,
			truncate(s.Redactor.Redact(strings.TrimSpace(string(snippet))), 300))
	}
	reply, err := tc.ReadReply(res, func(_ codec.Kind, detail string) {
		if onEvent != nil {
			onEvent(truncate(s.Redactor.Redact(detail), 200))
		}
	})
	if err != nil {
		return "", fmt.Errorf("reading the reply: %w", err)
	}
	return s.Redactor.Redact(reply), nil
}

// describeIfTransport rewords network failures; protocol errors keep their message.
func describeIfTransport(err error) string {
	var urlErr *url.Error
	if errors.Is(err, egress.ErrBlocked) || errors.Is(err, context.DeadlineExceeded) || errors.As(err, &urlErr) {
		return describe(err)
	}
	return err.Error()
}
