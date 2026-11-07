package codec

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const maxReplyBytes = 2 << 20

// Text reads a text agent's HTTP response.
type Text interface {
	// Accept is the Accept header to send when the agent does not set one.
	Accept() string
	// ReadReply returns the reply text; onEvent receives notable events for logs.
	ReadReply(res *http.Response, onEvent func(kind Kind, detail string)) (string, error)
}

// NewText builds a text codec.
func NewText(typ string, opts map[string]string) (Text, error) {
	spec, ok := SpecFor(typ)
	if !ok || spec.Modality != "text" {
		return nil, fmt.Errorf("%q is not a text codec", typ)
	}
	o, err := Options(typ, opts)
	if err != nil {
		return nil, err
	}
	if typ == "http_json" {
		return &httpJSON{replyPath: o["replyPath"]}, nil
	}
	s := &httpStream{deltaPath: o["deltaPath"]}
	for _, c := range []struct {
		raw string
		dst *condition
	}{{o["deltaWhen"], &s.deltaWhen}, {o["skipWhen"], &s.skipWhen}, {o["endWhen"], &s.endWhen}} {
		cond, err := parseCondition(c.raw)
		if err != nil {
			return nil, err
		}
		*c.dst = cond
	}
	return s, nil
}

type httpJSON struct{ replyPath string }

func (c *httpJSON) Accept() string { return "application/json" }

func (c *httpJSON) ReadReply(res *http.Response, _ func(Kind, string)) (string, error) {
	body, err := io.ReadAll(io.LimitReader(res.Body, maxReplyBytes))
	if err != nil {
		return "", err
	}
	var m any
	if err := json.Unmarshal(body, &m); err != nil {
		return "", errors.New("the response is not JSON")
	}
	v, ok := Lookup(m, c.replyPath)
	s, isString := v.(string)
	if !ok || !isString {
		return "", fmt.Errorf("the response has no text at %q", c.replyPath)
	}
	return s, nil
}

// condition matches lines where a dot path equals a value ("path=value").
type condition struct {
	path, value string
	set         bool
}

func parseCondition(raw string) (condition, error) {
	if raw == "" {
		return condition{}, nil
	}
	path, value, ok := strings.Cut(raw, "=")
	if !ok || strings.TrimSpace(path) == "" {
		return condition{}, fmt.Errorf("conditions look like path=value, got %q", raw)
	}
	return condition{path: strings.TrimSpace(path), value: strings.TrimSpace(value), set: true}, nil
}

func (c condition) match(m any) bool {
	v, ok := Lookup(m, c.path)
	if !ok {
		return false
	}
	switch t := v.(type) {
	case string:
		return t == c.value
	case bool:
		return strconv.FormatBool(t) == c.value
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64) == c.value
	}
	return false
}

type httpStream struct {
	deltaPath                    string
	deltaWhen, skipWhen, endWhen condition
}

func (c *httpStream) Accept() string { return "text/event-stream" }

func (c *httpStream) ReadReply(res *http.Response, onEvent func(Kind, string)) (string, error) {
	scanner := bufio.NewScanner(io.LimitReader(res.Body, maxReplyBytes))
	scanner.Buffer(make([]byte, 64*1024), maxReplyBytes)
	var reply strings.Builder
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if line == "" || strings.HasPrefix(line, ":") || strings.HasPrefix(line, "event:") || strings.HasPrefix(line, "id:") {
			continue
		}
		if line == "[DONE]" {
			return reply.String(), nil
		}
		var m any
		if json.Unmarshal([]byte(line), &m) != nil {
			continue
		}
		if c.endWhen.set && c.endWhen.match(m) {
			return reply.String(), nil
		}
		if c.skipWhen.set && c.skipWhen.match(m) {
			continue
		}
		if c.deltaWhen.set && !c.deltaWhen.match(m) {
			if onEvent != nil {
				onEvent(KindOther, summarize(m))
			}
			continue
		}
		if v, ok := Lookup(m, c.deltaPath); ok {
			if s, ok := v.(string); ok {
				reply.WriteString(s)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return reply.String(), fmt.Errorf("reading the stream: %w", err)
	}
	if c.endWhen.set {
		return reply.String(), errors.New("the stream ended before the end event")
	}
	return reply.String(), nil
}

// summarize names a stream event for logs without dumping its content.
func summarize(m any) string {
	obj, ok := m.(map[string]any)
	if !ok {
		return "event"
	}
	for _, k := range []string{"event", "type", "object"} {
		if s, ok := obj[k].(string); ok {
			return s
		}
	}
	return "event"
}

// Lookup follows a dot path through JSON objects and arrays ("a.0.b").
func Lookup(v any, path string) (any, bool) {
	if path == "" {
		return nil, false
	}
	for _, key := range strings.Split(path, ".") {
		switch node := v.(type) {
		case map[string]any:
			next, ok := node[key]
			if !ok {
				return nil, false
			}
			v = next
		case []any:
			i, err := strconv.Atoi(key)
			if err != nil || i < 0 || i >= len(node) {
				return nil, false
			}
			v = node[i]
		default:
			return nil, false
		}
	}
	return v, v != nil
}
