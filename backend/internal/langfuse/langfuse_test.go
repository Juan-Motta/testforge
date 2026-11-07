package langfuse

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"testforge/internal/connection"
)

func creds(url string) connection.Credentials {
	return connection.Credentials{Kind: connection.KindLangfuse, Config: map[string]string{"host": url, "publicKey": "pk-lf-1"}, Secret: "sk-lf-secret"}
}

func TestSessionObservationsV2Pagination(t *testing.T) {
	var auth, firstQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		switch r.URL.Path {
		case "/api/public/v2/observations":
			q := r.URL.Query()
			if q.Get("cursor") == "" {
				firstQuery = r.URL.RawQuery
				_ = json.NewEncoder(w).Encode(map[string]any{
					"data": []any{map[string]any{"id": "o1", "traceId": "t1", "type": "GENERATION", "name": "llm", "model": "gpt-x",
						"startTime": "2026-11-07T10:00:00.000Z", "endTime": "2026-11-07T10:00:01.200Z", "completionStartTime": "2026-11-07T10:00:00.400Z",
						"usageDetails": map[string]any{"input": 100, "output": 20, "total": 120}, "totalCost": 0.002, "output": "hola"}},
					"meta": map[string]any{"cursor": "next"},
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []any{map[string]any{"id": "o2", "traceId": "t1", "type": "TOOL", "name": "calendar", "level": "ERROR", "statusMessage": "timeout",
					"startTime": "2026-11-07T10:00:00.500Z", "endTime": "2026-11-07T10:00:00.900Z"}},
				"meta": map[string]any{},
			})
		case "/api/public/projects":
			_, _ = w.Write([]byte(`{"data":[{"id":"proj-1","name":"Prod"}]}`))
		}
	}))
	defer srv.Close()

	c, err := New(creds(srv.URL), true)
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 11, 7, 9, 59, 0, 0, time.UTC)
	obs, err := c.SessionObservations(context.Background(), "sess 1", from, from.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 2 || obs[0].ID != "o1" {
		t.Fatalf("observations = %+v", obs)
	}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("pk-lf-1:sk-lf-secret"))
	if auth != want {
		t.Fatalf("auth = %q", auth)
	}
	for _, part := range []string{"sessionId=sess+1", "fromStartTime=", "toStartTime=", "fields=", "limit="} {
		if !strings.Contains(firstQuery, part) {
			t.Fatalf("query %q misses %q", firstQuery, part)
		}
	}

	stats, rows := Digest(obs)
	if stats.Traces != 1 || stats.Observations != 2 || stats.Errors != 1 || stats.ToolCalls != 1 || stats.TotalTokens != 120 ||
		stats.TotalCost == nil || *stats.TotalCost != 0.002 || *stats.SlowestGenerationMs != 1200 || *stats.TimeToFirstTokenMs != 400 || stats.Models[0] != "gpt-x" {
		t.Fatalf("stats = %+v", stats)
	}
	if len(rows) != 2 || !strings.HasPrefix(rows[0], "R1 GENERATION llm") || !strings.Contains(rows[1], "ERROR") || !strings.Contains(rows[1], "timeout") {
		t.Fatalf("rows = %v", rows)
	}
	url, err := c.SessionURL(context.Background(), "sess 1")
	if err != nil || url != srv.URL+"/project/proj-1/sessions/sess%201" {
		t.Fatalf("url = %q, %v", url, err)
	}
}

func TestSessionObservationsFallsBackToV1(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/public/v2/observations":
			w.WriteHeader(http.StatusNotFound)
		case "/api/public/traces":
			if r.URL.Query().Get("sessionId") != "s1" {
				w.WriteHeader(400)
				return
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"t1"},{"id":"t2"}],"meta":{"page":1,"totalPages":1}}`))
		case "/api/public/observations":
			tid := r.URL.Query().Get("traceId")
			_, _ = w.Write([]byte(`{"data":[{"id":"o-` + tid + `","traceId":"` + tid + `","type":"GENERATION","name":"llm",
				"startTime":"2026-11-07T10:00:00Z","usage":{"input":10,"output":5,"total":15},"calculatedTotalCost":0.001}],"meta":{"page":1,"totalPages":1}}`))
		}
	}))
	defer srv.Close()
	c, _ := New(creds(srv.URL), true)
	obs, err := c.SessionObservations(context.Background(), "s1", time.Now().Add(-time.Hour), time.Now())
	if err != nil || len(obs) != 2 {
		t.Fatalf("obs = %+v, %v", obs, err)
	}
	stats, _ := Digest(obs)
	if stats.TotalTokens != 30 || stats.Traces != 2 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestRejectedCredentialsAndEgress(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(401) }))
	defer srv.Close()
	c, _ := New(creds(srv.URL), true)
	_, err := c.SessionObservations(context.Background(), "s", time.Now(), time.Now())
	if err == nil || !strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "sk-lf-secret") {
		t.Fatalf("err = %v", err)
	}
	locked, _ := New(creds(srv.URL), false)
	if _, err := locked.SessionObservations(context.Background(), "s", time.Now(), time.Now()); err == nil || !strings.Contains(err.Error(), "private network") {
		t.Fatalf("egress err = %v", err)
	}
	if _, err := New(connection.Credentials{Kind: connection.KindOpenAI}, true); err == nil {
		t.Fatal("non-langfuse connection accepted")
	}
}
