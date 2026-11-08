// Package client is a typed HTTP client of the TestForge public API for
// programmatic callers (the CLI and the MCP server). It authenticates with a
// workspace API key and returns the API's JSON so callers can print it as is.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
)

// maxResponse bounds how much of a response body is read.
const maxResponse = 32 << 20

// Client calls one TestForge API with one API key.
type Client struct {
	base      *url.URL
	key       string
	hc        *http.Client
	UserAgent string
}

// New validates the API URL and returns a client. The key is only ever sent
// over https, or over http to a loopback host (local development).
func New(rawURL, key string) (*Client, error) {
	u, err := ParseURL(rawURL)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(key) == "" {
		return nil, errors.New("missing API key")
	}
	return &Client{
		base: u,
		key:  strings.TrimSpace(key),
		hc: &http.Client{
			Timeout: 60 * time.Second,
			// The API never redirects; following one could hand the key to another host.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		UserAgent: "testforge-cli",
	}, nil
}

// ParseURL normalizes an API base URL and enforces the transport rule.
func ParseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, fmt.Errorf("invalid API URL %q: use https://host[:port]", raw)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("invalid API URL %q: no credentials, query or fragment", raw)
	}
	if u.Scheme == "http" && !isLoopback(u.Hostname()) {
		return nil, fmt.Errorf("refusing to send an API key over plain http to %s: use https", u.Host)
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u, nil
}

func isLoopback(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// BaseURL is the normalized API URL.
func (c *Client) BaseURL() string { return c.base.String() }

// Error is an API error response.
type Error struct {
	Status  int
	Code    string
	Message string
	Fields  map[string]string
}

func (e *Error) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.Status)
	}
	if len(e.Fields) > 0 {
		keys := make([]string, 0, len(e.Fields))
		for k := range e.Fields {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+": "+e.Fields[k])
		}
		msg += " (" + strings.Join(parts, "; ") + ")"
	}
	return msg
}

func (c *Client) do(ctx context.Context, method, path string, q url.Values, body any) (json.RawMessage, error) {
	// path is already escaped segment by segment.
	target := c.base.String() + path
	if len(q) > 0 {
		target += "?" + q.Encode()
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach %s: %w", c.base.Host, unwrapURLError(err))
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if err != nil {
		return nil, fmt.Errorf("reading the response: %w", err)
	}
	if resp.StatusCode >= 300 {
		e := &Error{Status: resp.StatusCode}
		var env struct {
			Error struct {
				Code    string            `json:"code"`
				Message string            `json:"message"`
				Fields  map[string]string `json:"fields"`
			} `json:"error"`
		}
		if json.Unmarshal(data, &env) == nil {
			e.Code, e.Message, e.Fields = env.Error.Code, env.Error.Message, env.Error.Fields
		}
		return nil, e
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return json.RawMessage("null"), nil
	}
	if !json.Valid(data) {
		return nil, fmt.Errorf("the server at %s did not answer JSON (is it the TestForge API?)", c.base.Host)
	}
	return data, nil
}

func unwrapURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

func ws(slug string, parts ...string) string {
	p := "/api/v1/workspaces/" + url.PathEscape(slug)
	for _, s := range parts {
		p += "/" + url.PathEscape(s)
	}
	return p
}

func set(q url.Values, k, v string) {
	if v != "" {
		q.Set(k, v)
	}
}

// Me returns the key's identity and workspace.
func (c *Client) Me(ctx context.Context) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, "/api/v1/auth/me", nil, nil)
}

// ListAgents lists the workspace's agents.
func (c *Client) ListAgents(ctx context.Context, slug string) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, ws(slug, "agents"), nil, nil)
}

// GetAgent reads one agent.
func (c *Client) GetAgent(ctx context.Context, slug, id string) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, ws(slug, "agents", id), nil, nil)
}

// ScenarioFilter narrows ListScenarios.
type ScenarioFilter struct{ AgentID, GroupID, Tag, Q string }

// ListScenarios lists scenarios.
func (c *Client) ListScenarios(ctx context.Context, slug string, f ScenarioFilter) (json.RawMessage, error) {
	q := url.Values{}
	set(q, "agentId", f.AgentID)
	set(q, "groupId", f.GroupID)
	set(q, "tag", f.Tag)
	set(q, "q", f.Q)
	return c.do(ctx, http.MethodGet, ws(slug, "scenarios"), q, nil)
}

// GetScenario reads one scenario.
func (c *Client) GetScenario(ctx context.Context, slug, id string) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, ws(slug, "scenarios", id), nil, nil)
}

// LaunchRequest is the launch body; zero values use the API defaults.
type LaunchRequest struct {
	ScenarioIDs []string `json:"scenarioIds"`
	Repetitions *int     `json:"repetitions,omitempty"`
	Concurrency *int     `json:"concurrency,omitempty"`
	Label       string   `json:"label,omitempty"`
}

// LaunchRuns queues a batch.
func (c *Client) LaunchRuns(ctx context.Context, slug string, r LaunchRequest) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, ws(slug, "runs"), nil, r)
}

// RunFilter narrows ListRuns.
type RunFilter struct {
	BatchID, ScenarioID, AgentID, Status, Before string
	Limit                                        int
}

// ListRuns lists runs, newest first, one page.
func (c *Client) ListRuns(ctx context.Context, slug string, f RunFilter) (json.RawMessage, error) {
	q := url.Values{}
	set(q, "batchId", f.BatchID)
	set(q, "scenarioId", f.ScenarioID)
	set(q, "agentId", f.AgentID)
	set(q, "status", f.Status)
	set(q, "before", f.Before)
	if f.Limit > 0 {
		q.Set("limit", strconv.Itoa(f.Limit))
	}
	return c.do(ctx, http.MethodGet, ws(slug, "runs"), q, nil)
}

// GetRun reads a run with its transcript.
func (c *Client) GetRun(ctx context.Context, slug, id string) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, ws(slug, "runs", id), nil, nil)
}

// CancelRun cancels a queued or running run.
func (c *Client) CancelRun(ctx context.Context, slug, id string) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, ws(slug, "runs", id, "cancel"), nil, nil)
}

// GetJudgement reads a run's judgement.
func (c *Client) GetJudgement(ctx context.Context, slug, runID string) (json.RawMessage, error) {
	return c.do(ctx, http.MethodGet, ws(slug, "runs", runID, "judgement"), nil, nil)
}

// Rejudge queues the run's judgement again.
func (c *Client) Rejudge(ctx context.Context, slug, runID string) (json.RawMessage, error) {
	return c.do(ctx, http.MethodPost, ws(slug, "runs", runID, "judgement"), nil, nil)
}

// ReportFilter narrows ReportsOverview.
type ReportFilter struct {
	Days                int
	AgentID, ScenarioID string
}

// ReportsOverview returns the evaluation report.
func (c *Client) ReportsOverview(ctx context.Context, slug string, f ReportFilter) (json.RawMessage, error) {
	q := url.Values{}
	if f.Days > 0 {
		q.Set("days", strconv.Itoa(f.Days))
	}
	set(q, "agentId", f.AgentID)
	set(q, "scenarioId", f.ScenarioID)
	return c.do(ctx, http.MethodGet, ws(slug, "reports", "overview"), q, nil)
}
