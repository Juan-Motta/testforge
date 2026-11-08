package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseURL(t *testing.T) {
	ok := map[string]string{
		"https://api.example.com/":   "https://api.example.com",
		"https://api.example.com/tf": "https://api.example.com/tf",
		"http://localhost:8080":      "http://localhost:8080",
		"http://127.0.0.1:8080/":     "http://127.0.0.1:8080",
		"http://[::1]:8080":          "http://[::1]:8080",
	}
	for in, want := range ok {
		u, err := ParseURL(in)
		if err != nil || u.String() != want {
			t.Errorf("ParseURL(%q) = %v, %v; want %s", in, u, err, want)
		}
	}
	for _, in := range []string{"", "api.example.com", "ftp://x", "http://api.example.com", "http://10.0.0.5:8080", "http://app.localhost:8080", "http://localhost.evil.com",
		"https://user:pw@api.example.com", "https://api.example.com/?a=1", "https://api.example.com/#x"} {
		if _, err := ParseURL(in); err == nil {
			t.Errorf("ParseURL(%q) accepted", in)
		}
	}
	if _, err := New("https://api.example.com", "  "); err == nil {
		t.Error("empty key accepted")
	}
}

func TestRequestsAndErrors(t *testing.T) {
	var got *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		switch r.URL.Path {
		case "/api/v1/workspaces/acme/runs":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[],"nextCursor":null}`))
		case "/api/v1/workspaces/acme/runs/x/judgement":
			w.WriteHeader(422)
			_, _ = w.Write([]byte(`{"error":{"code":"invalid","message":"Invalid request.","fields":{"b":"bad","a":"worse"}}}`))
		case "/redirect":
			http.Redirect(w, r, "https://elsewhere.example.com/", http.StatusFound)
		default:
			_, _ = w.Write([]byte(`<html>`))
		}
	}))
	defer srv.Close()
	c, err := New(srv.URL, "tf_secret")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	out, err := c.ListRuns(ctx, "acme", RunFilter{BatchID: "b1", Status: "completed", Limit: 200})
	if err != nil || string(out) != `{"items":[],"nextCursor":null}` {
		t.Fatalf("ListRuns = %s, %v", out, err)
	}
	if got.Header.Get("Authorization") != "Bearer tf_secret" || got.URL.Query().Get("batchId") != "b1" ||
		got.URL.Query().Get("status") != "completed" || got.URL.Query().Get("limit") != "200" || got.URL.Query().Has("agentId") {
		t.Fatalf("request = %v %v", got.Header, got.URL)
	}

	_, err = c.Rejudge(ctx, "acme", "x")
	var apiErr *Error
	if !errors.As(err, &apiErr) || apiErr.Status != 422 || apiErr.Code != "invalid" || err.Error() != "Invalid request. (a: worse; b: bad)" {
		t.Fatalf("Rejudge error = %#v %v", apiErr, err)
	}
	if got.Method != http.MethodPost {
		t.Fatalf("method = %s", got.Method)
	}

	// Path segments are escaped, so an id cannot reach another route.
	_, _ = c.GetRun(ctx, "acme", "../agents")
	if got.URL.EscapedPath() != "/api/v1/workspaces/acme/runs/..%2Fagents" {
		t.Fatalf("path = %s", got.URL.EscapedPath())
	}

	if _, err := c.do(ctx, http.MethodGet, "/other", nil, nil); err == nil || !strings.Contains(err.Error(), "did not answer JSON") {
		t.Fatalf("non-JSON = %v", err)
	}
	_, err = c.do(ctx, http.MethodGet, "/redirect", nil, nil)
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusFound || !strings.Contains(err.Error(), "unexpected redirect (302)") {
		t.Fatalf("redirect = %v", err)
	}
}
