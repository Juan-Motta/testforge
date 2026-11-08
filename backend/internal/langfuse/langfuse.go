// Package langfuse reads an agent's traces from Langfuse's public API so the
// judge can see tool calls, models, errors and internal latencies.
package langfuse

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"testforge/internal/connection"
	"testforge/internal/platform/egress"
)

const (
	timeout  = 20 * time.Second
	pageSize = 200
	maxPages = 25
	// v2 field groups the judge needs; io is truncated in the digest.
	v2Fields = "core,basic,time,model,usage,io"
)

// Client reads one Langfuse project.
type Client struct {
	http *http.Client
	host string
	auth string
}

// New builds a client for a langfuse connection.
func New(c connection.Credentials, allowPrivate bool) (*Client, error) {
	if c.Kind != connection.KindLangfuse {
		return nil, fmt.Errorf("%s connections do not provide traces", c.Kind)
	}
	return &Client{
		http: egress.NewClient(allowPrivate, timeout),
		host: strings.TrimRight(c.Config["host"], "/"),
		auth: "Basic " + base64.StdEncoding.EncodeToString([]byte(c.Config["publicKey"]+":"+c.Secret)),
	}, nil
}

// Observation is the part of a Langfuse observation the judge uses.
type Observation struct {
	ID                  string
	TraceID             string
	Type                string
	Name                string
	Level               string
	StatusMessage       string
	Model               string
	StartTime           *time.Time
	EndTime             *time.Time
	CompletionStartTime *time.Time
	TotalTokens         int
	Cost                *float64
	Input               string
	Output              string
}

type apiObservation struct {
	ID                  string             `json:"id"`
	TraceID             string             `json:"traceId"`
	Type                string             `json:"type"`
	Name                string             `json:"name"`
	Level               string             `json:"level"`
	StatusMessage       string             `json:"statusMessage"`
	Model               string             `json:"model"`
	StartTime           *time.Time         `json:"startTime"`
	EndTime             *time.Time         `json:"endTime"`
	CompletionStartTime *time.Time         `json:"completionStartTime"`
	UsageDetails        map[string]float64 `json:"usageDetails"`
	Usage               map[string]float64 `json:"usage"` // v1
	TotalUsage          *float64           `json:"totalUsage"`
	TotalCost           *float64           `json:"totalCost"`
	CalculatedTotalCost *float64           `json:"calculatedTotalCost"` // v1
	Input               json.RawMessage    `json:"input"`
	Output              json.RawMessage    `json:"output"`
}

func (a apiObservation) normalize() Observation {
	o := Observation{ID: a.ID, TraceID: a.TraceID, Type: a.Type, Name: a.Name, Level: a.Level, StatusMessage: a.StatusMessage,
		Model: a.Model, StartTime: a.StartTime, EndTime: a.EndTime, CompletionStartTime: a.CompletionStartTime,
		Input: rawText(a.Input), Output: rawText(a.Output)}
	usage := a.UsageDetails
	if usage == nil {
		usage = a.Usage
	}
	switch {
	case a.TotalUsage != nil:
		o.TotalTokens = int(*a.TotalUsage)
	case usage["total"] > 0:
		o.TotalTokens = int(usage["total"])
	default:
		o.TotalTokens = int(usage["input"] + usage["output"])
	}
	o.Cost = a.TotalCost
	if o.Cost == nil {
		o.Cost = a.CalculatedTotalCost
	}
	return o
}

// rawText turns an input/output payload (string or JSON) into plain text.
func rawText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return string(raw)
}

type page struct {
	Data []apiObservation `json:"data"`
	Meta struct {
		Cursor     string `json:"cursor"`
		Page       int    `json:"page"`
		TotalPages int    `json:"totalPages"`
	} `json:"meta"`
}

var errNotFound = errors.New("not found")

// HTTPError is a non-success answer from Langfuse.
type HTTPError struct{ Status int }

func (e *HTTPError) Error() string { return fmt.Sprintf("Langfuse answered HTTP %d", e.Status) }

// IsPermanent reports errors that waiting will not fix (bad credentials or request).
func IsPermanent(err error) bool {
	var h *HTTPError
	return errors.As(err, &h) && h.Status >= 400 && h.Status < 500 && h.Status != http.StatusRequestTimeout && h.Status != http.StatusTooManyRequests
}

func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.host+path+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return fmt.Errorf("could not reach Langfuse: %w", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 20<<20))
	switch {
	case res.StatusCode == http.StatusNotFound:
		return errNotFound
	case res.StatusCode >= 300:
		return &HTTPError{Status: res.StatusCode}
	}
	if err := json.Unmarshal(body, out); err != nil {
		return errors.New("Langfuse returned an unexpected response")
	}
	return nil
}

// SessionObservations returns every observation of a session between from and to.
func (c *Client) SessionObservations(ctx context.Context, sessionID string, from, to time.Time) ([]Observation, error) {
	var out []Observation
	q := url.Values{
		"sessionId": {sessionID}, "fields": {v2Fields}, "limit": {strconv.Itoa(pageSize)},
		"fromStartTime": {from.UTC().Format(time.RFC3339)}, "toStartTime": {to.UTC().Format(time.RFC3339)},
	}
	for range maxPages {
		var p page
		err := c.get(ctx, "/api/public/v2/observations", q, &p)
		if errors.Is(err, errNotFound) && out == nil {
			return c.sessionObservationsV1(ctx, sessionID, from, to)
		}
		if errors.Is(err, errNotFound) {
			err = &HTTPError{Status: http.StatusNotFound}
		}
		if err != nil {
			return nil, err
		}
		for _, a := range p.Data {
			out = append(out, a.normalize())
		}
		if p.Meta.Cursor == "" || p.Meta.Cursor == q.Get("cursor") {
			break // a repeated cursor would loop over the same page
		}
		q.Set("cursor", p.Meta.Cursor)
	}
	sortByStart(out)
	return out, nil
}

// sessionObservationsV1 serves self-hosted Langfuse v3: traces of the session, then their observations.
func (c *Client) sessionObservationsV1(ctx context.Context, sessionID string, from, to time.Time) ([]Observation, error) {
	var traceIDs []string
	for pageNo := 1; pageNo <= maxPages; pageNo++ {
		var p struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
			Meta struct {
				TotalPages int `json:"totalPages"`
			} `json:"meta"`
		}
		q := url.Values{"sessionId": {sessionID}, "page": {strconv.Itoa(pageNo)}, "limit": {"100"},
			"fromTimestamp": {from.UTC().Format(time.RFC3339)}, "toTimestamp": {to.UTC().Format(time.RFC3339)}}
		if err := c.get(ctx, "/api/public/traces", q, &p); err != nil {
			return nil, err
		}
		for _, t := range p.Data {
			traceIDs = append(traceIDs, t.ID)
		}
		if pageNo >= p.Meta.TotalPages {
			break
		}
	}
	var out []Observation
	for _, id := range traceIDs {
		for pageNo := 1; pageNo <= maxPages; pageNo++ {
			var p page
			q := url.Values{"traceId": {id}, "page": {strconv.Itoa(pageNo)}, "limit": {"100"}}
			if err := c.get(ctx, "/api/public/observations", q, &p); err != nil {
				return nil, err
			}
			for _, a := range p.Data {
				out = append(out, a.normalize())
			}
			if pageNo >= p.Meta.TotalPages {
				break
			}
		}
	}
	sortByStart(out)
	return out, nil
}

func sortByStart(obs []Observation) {
	sort.SliceStable(obs, func(i, j int) bool {
		a, b := obs[i].StartTime, obs[j].StartTime
		switch {
		case a == nil:
			return false
		case b == nil:
			return true
		}
		return a.Before(*b)
	})
}

// SessionURL builds the Langfuse page of a session.
func (c *Client) SessionURL(ctx context.Context, sessionID string) (string, error) {
	var p struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/api/public/projects", url.Values{}, &p); err != nil {
		return "", err
	}
	if len(p.Data) == 0 {
		return "", errors.New("the Langfuse key has no project")
	}
	return c.host + "/project/" + url.PathEscape(p.Data[0].ID) + "/sessions/" + url.PathEscape(sessionID), nil
}

// Stats summarize a session's observations.
type Stats struct {
	Traces              int      `json:"traces"`
	Observations        int      `json:"observations"`
	Errors              int      `json:"errors"`
	Models              []string `json:"models"`
	ToolCalls           int      `json:"toolCalls"`
	TotalTokens         int      `json:"totalTokens"`
	TotalCost           *float64 `json:"totalCost"`
	SlowestGenerationMs *int     `json:"slowestGenerationMs"`
	TimeToFirstTokenMs  *int     `json:"timeToFirstTokenMs"`
}

const (
	maxDigestRows = 150
	maxIOChars    = 300
)

// Digest computes stats and bounded, numbered rows (R1, R2…) for the judge.
func Digest(obs []Observation) (Stats, []string) {
	st := Stats{Observations: len(obs), Models: []string{}}
	traces, models := map[string]bool{}, map[string]bool{}
	var ttfts []int
	var rows []string
	for i, o := range obs {
		traces[o.TraceID] = true
		if o.Model != "" && !models[o.Model] {
			models[o.Model] = true
			st.Models = append(st.Models, o.Model)
		}
		if strings.EqualFold(o.Level, "ERROR") {
			st.Errors++
		}
		if strings.EqualFold(o.Type, "TOOL") {
			st.ToolCalls++
		}
		st.TotalTokens += o.TotalTokens
		if o.Cost != nil {
			total := *o.Cost
			if st.TotalCost != nil {
				total += *st.TotalCost
			}
			st.TotalCost = &total
		}
		dur := -1
		if o.StartTime != nil && o.EndTime != nil {
			dur = int(o.EndTime.Sub(*o.StartTime).Milliseconds())
		}
		ttft := -1
		if o.StartTime != nil && o.CompletionStartTime != nil {
			ttft = int(o.CompletionStartTime.Sub(*o.StartTime).Milliseconds())
		}
		if strings.EqualFold(o.Type, "GENERATION") {
			if dur >= 0 && (st.SlowestGenerationMs == nil || dur > *st.SlowestGenerationMs) {
				d := dur
				st.SlowestGenerationMs = &d
			}
			if ttft >= 0 {
				ttfts = append(ttfts, ttft)
			}
		}
		if len(rows) >= maxDigestRows {
			continue
		}
		// Every field is clipped to one line so a value cannot start a fake row.
		parts := []string{fmt.Sprintf("R%d %s %s", i+1, clip(o.Type, 40), clip(o.Name, 120))}
		if o.Model != "" {
			parts = append(parts, "model="+clip(o.Model, 80))
		}
		if dur >= 0 {
			parts = append(parts, fmt.Sprintf("duration=%dms", dur))
		}
		if ttft >= 0 {
			parts = append(parts, fmt.Sprintf("ttft=%dms", ttft))
		}
		if o.TotalTokens > 0 {
			parts = append(parts, fmt.Sprintf("tokens=%d", o.TotalTokens))
		}
		if o.Level != "" && !strings.EqualFold(o.Level, "DEFAULT") {
			parts = append(parts, "level="+clip(o.Level, 20))
		}
		if o.StatusMessage != "" {
			parts = append(parts, "status="+clip(o.StatusMessage, 200))
		}
		if o.Input != "" {
			parts = append(parts, "input="+clip(o.Input, maxIOChars))
		}
		if o.Output != "" {
			parts = append(parts, "output="+clip(o.Output, maxIOChars))
		}
		rows = append(rows, strings.Join(parts, " "))
	}
	st.Traces = len(traces)
	if len(ttfts) > 0 {
		sort.Ints(ttfts)
		m := ttfts[len(ttfts)/2]
		st.TimeToFirstTokenMs = &m
	}
	return st, rows
}

func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}
