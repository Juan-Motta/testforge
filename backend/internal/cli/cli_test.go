package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeAPI is a minimal TestForge API for one workspace "acme".
type fakeAPI struct {
	mu       sync.Mutex
	key      string
	auth     string // authMethod returned by /me
	runs     []map[string]any
	polls    int
	settleAt int // polls after which runs are settled
	launched map[string]any
	lastAuth string
}

func newFake(t *testing.T) (*fakeAPI, *httptest.Server) {
	f := &fakeAPI{key: "tf_good", auth: "api_key"}
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	return f, srv
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastAuth = r.Header.Get("Authorization")
	w.Header().Set("Content-Type", "application/json")
	if f.lastAuth != "Bearer "+f.key {
		w.WriteHeader(401)
		fmt.Fprint(w, `{"error":{"code":"unauthorized","message":"Authentication required."}}`)
		return
	}
	switch p := r.URL.Path; {
	case p == "/api/v1/auth/me":
		fmt.Fprintf(w, `{"user":{"id":"u1","name":"Ada","email":"ada@example.com"},"workspaces":[{"id":"w1","name":"Acme","slug":"acme","role":"member"}],"authMethod":%q}`, f.auth)
	case p == "/api/v1/workspaces/acme/scenarios":
		if r.URL.Query().Get("tag") == "smoke" {
			fmt.Fprint(w, `{"items":[{"id":"s1","name":"Book a table","modality":"text","tags":["smoke"],"goal":"Book"},{"id":"s2","name":"Cancel","modality":"text","tags":["smoke"],"goal":"Cancel"}]}`)
		} else {
			fmt.Fprint(w, `{"items":[]}`)
		}
	case p == "/api/v1/workspaces/acme/runs" && r.Method == http.MethodPost:
		_ = json.NewDecoder(r.Body).Decode(&f.launched)
		w.WriteHeader(201)
		fmt.Fprint(w, `{"id":"b1","label":"","concurrency":3,"total":2,"counts":{"queued":2},"createdAt":"2026-11-07T10:00:00Z","createdBy":null}`)
	case p == "/api/v1/workspaces/acme/runs":
		f.polls++
		items := f.runs
		if f.polls < f.settleAt {
			items = []map[string]any{run("r1", "running", nil, nil, nil), run("r2", "queued", nil, nil, nil)}
		}
		b, _ := json.Marshal(map[string]any{"items": items, "nextCursor": nil})
		_, _ = w.Write(b)
	case p == "/api/v1/workspaces/acme/runs/r1":
		b, _ := json.Marshal(f.runs[0])
		_, _ = w.Write(b)
	case p == "/api/v1/workspaces/acme/runs/missing":
		w.WriteHeader(404)
		fmt.Fprint(w, `{"error":{"code":"not_found","message":"Run not found."}}`)
	default:
		w.WriteHeader(404)
		fmt.Fprint(w, `{"error":{"code":"not_found","message":"Not found."}}`)
	}
}

func run(id, status string, judgement *string, score *int, passed *bool) map[string]any {
	return map[string]any{"id": id, "batchId": "b1", "scenarioName": "Book a table", "agentName": "Bot", "personaName": "Ana",
		"modality": "text", "status": status, "endReason": nil, "error": nil, "createdAt": "2026-11-07T10:00:00Z",
		"metrics": nil, "judgementStatus": judgement, "score": score, "passed": passed, "transcript": []any{}}
}

func ptr[T any](v T) *T { return &v }

type harness struct {
	env            *Env
	stdout, stderr *bytes.Buffer
	vars           map[string]string
}

func newHarness(t *testing.T) *harness {
	h := &harness{stdout: &bytes.Buffer{}, stderr: &bytes.Buffer{}, vars: map[string]string{"XDG_CONFIG_HOME": t.TempDir()}}
	h.env = &Env{Stdin: strings.NewReader(""), Stdout: h.stdout, Stderr: h.stderr,
		Getenv: func(k string) string { return h.vars[k] }, PollEvery: time.Millisecond}
	return h
}

func (h *harness) run(args ...string) int {
	h.stdout.Reset()
	h.stderr.Reset()
	return Main(context.Background(), args, h.env)
}

func TestLoginWhoamiLogout(t *testing.T) {
	f, srv := newFake(t)
	h := newHarness(t)

	h.env.Stdin = strings.NewReader("tf_good\n")
	if code := h.run("login", "--url", srv.URL+"/"); code != 0 {
		t.Fatalf("login = %d: %s", code, h.stderr)
	}
	if strings.Contains(h.stdout.String(), "tf_good") || !strings.Contains(h.stdout.String(), "as Ada (workspace acme)") {
		t.Fatalf("login output: %s", h.stdout)
	}
	path := filepath.Join(h.vars["XDG_CONFIG_HOME"], "testforge", "config.json")
	fi, err := os.Stat(path)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %v, %v", fi, err)
	}
	di, _ := os.Stat(filepath.Dir(path))
	if di.Mode().Perm() != 0o700 {
		t.Fatalf("config dir mode = %v", di.Mode())
	}
	var cfg Config
	data, _ := os.ReadFile(path)
	_ = json.Unmarshal(data, &cfg)
	if cfg != (Config{URL: srv.URL, APIKey: "tf_good", Workspace: "acme"}) {
		t.Fatalf("config = %+v", cfg)
	}

	if code := h.run("whoami"); code != 0 || !strings.Contains(h.stdout.String(), "Ada <ada@example.com>") ||
		!strings.Contains(h.stdout.String(), "acme (Acme, member)") || strings.Contains(h.stdout.String(), "tf_good") {
		t.Fatalf("whoami = %d: %s %s", code, h.stdout, h.stderr)
	}

	// The stored key is never sent to another URL.
	if code := h.run("whoami", "--url", "http://127.0.0.1:1"); code != 1 || !strings.Contains(h.stderr.String(), "no API key for http://127.0.0.1:1") {
		t.Fatalf("other url = %d: %s", code, h.stderr)
	}
	// An environment key goes with an environment URL.
	h.vars["TESTFORGE_URL"], h.vars["TESTFORGE_API_KEY"] = srv.URL, "tf_good"
	if code := h.run("whoami", "--json"); code != 0 || !strings.Contains(h.stdout.String(), `"authMethod": "api_key"`) {
		t.Fatalf("env whoami = %d: %s %s", code, h.stdout, h.stderr)
	}
	delete(h.vars, "TESTFORGE_URL")
	delete(h.vars, "TESTFORGE_API_KEY")

	if code := h.run("logout"); code != 0 || !strings.Contains(h.stdout.String(), "Logged out") {
		t.Fatalf("logout = %d", code)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("config still exists: %v", err)
	}
	if code := h.run("whoami"); code != 1 || !strings.Contains(h.stderr.String(), "not logged in") {
		t.Fatalf("whoami after logout = %d: %s", code, h.stderr)
	}

	// A session credential is not stored; a bad key reports why.
	f.auth = "session"
	h.env.Stdin = strings.NewReader("tf_good\n")
	if code := h.run("login", "--url", srv.URL); code != 1 || !strings.Contains(h.stderr.String(), "not a workspace API key") {
		t.Fatalf("session login = %d: %s", code, h.stderr)
	}
	h.env.Stdin = strings.NewReader("tf_bad\n")
	if code := h.run("login", "--url", srv.URL); code != 1 || !strings.Contains(h.stderr.String(), "invalid, expired or revoked") {
		t.Fatalf("bad key login = %d: %s", code, h.stderr)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("a failed login wrote the config")
	}
	// Plain http to a remote host is refused before the key is read.
	if code := h.run("login", "--url", "http://testforge.example.com"); code != 1 || !strings.Contains(h.stderr.String(), "plain http") {
		t.Fatalf("http login = %d: %s", code, h.stderr)
	}
}

func TestUsageAndErrors(t *testing.T) {
	_, srv := newFake(t)
	h := newHarness(t)
	h.vars["TESTFORGE_URL"], h.vars["TESTFORGE_API_KEY"] = srv.URL, "tf_good"

	if code := h.run(); code != 0 || !strings.Contains(h.stdout.String(), "runs launch") {
		t.Fatalf("help = %d", code)
	}
	if code := h.run("runs", "get", "-h"); code != 0 || !strings.Contains(h.stderr.String(), "Usage: testforge runs get <run-id>") {
		t.Fatalf("runs get -h = %d: %s", code, h.stderr)
	}
	if code := h.run("nope"); code != 1 || !strings.Contains(h.stderr.String(), `unknown command "nope"`) {
		t.Fatalf("unknown = %d", code)
	}
	if code := h.run("runs"); code != 1 || !strings.Contains(h.stderr.String(), "missing subcommand") {
		t.Fatalf("runs = %d", code)
	}
	if code := h.run("runs", "get"); code != 1 || !strings.Contains(h.stderr.String(), "expected 1 argument(s), got 0") {
		t.Fatalf("runs get = %d: %s", code, h.stderr)
	}
	if code := h.run("runs", "get", "missing"); code != 1 || h.stderr.String() != "error: Run not found.\n" {
		t.Fatalf("missing run = %d: %q", code, h.stderr)
	}
	if code := h.run("runs", "launch"); code != 1 || !strings.Contains(h.stderr.String(), "pass --scenario") {
		t.Fatalf("launch without selection = %d: %s", code, h.stderr)
	}
	if code := h.run("runs", "launch", "--tag", "none"); code != 1 || !strings.Contains(h.stderr.String(), "no scenarios match") {
		t.Fatalf("launch empty selection = %d: %s", code, h.stderr)
	}
}

func TestRunsGetFlagsAfterArgument(t *testing.T) {
	f, srv := newFake(t)
	f.runs = []map[string]any{run("r1", "running", nil, nil, nil)}
	h := newHarness(t)
	h.vars["TESTFORGE_URL"], h.vars["TESTFORGE_API_KEY"] = srv.URL, "tf_good"
	if code := h.run("runs", "get", "r1", "--json"); code != 0 || !strings.Contains(h.stdout.String(), `"id": "r1"`) {
		t.Fatalf("runs get --json = %d: %s %s", code, h.stdout, h.stderr)
	}
	if code := h.run("runs", "get", "r1"); code != 0 || !strings.Contains(h.stdout.String(), "Status     running") {
		t.Fatalf("runs get = %d: %s %s", code, h.stdout, h.stderr)
	}
}

func TestLaunchWaitExitCodes(t *testing.T) {
	cases := []struct {
		name string
		runs []map[string]any
		code int
		want string
	}{
		{"all pass", []map[string]any{
			run("r1", "completed", ptr("completed"), ptr(91), ptr(true)),
			run("r2", "completed", ptr("skipped"), nil, nil),
		}, 0, "2 of 2 runs passed."},
		{"judged fail", []map[string]any{
			run("r1", "completed", ptr("completed"), ptr(91), ptr(true)),
			run("r2", "completed", ptr("completed"), ptr(40), ptr(false)),
		}, 2, "1 of 2 runs passed."},
		{"run failed", []map[string]any{
			run("r1", "completed", ptr("completed"), ptr(91), ptr(true)),
			run("r2", "failed", nil, nil, nil),
		}, 2, "1 of 2 runs passed."},
		{"judgement failed", []map[string]any{
			run("r1", "completed", ptr("failed"), nil, nil),
			run("r2", "completed", ptr("completed"), ptr(91), ptr(true)),
		}, 2, "1 of 2 runs passed."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, srv := newFake(t)
			f.runs, f.settleAt = tc.runs, 3
			h := newHarness(t)
			h.vars["TESTFORGE_URL"], h.vars["TESTFORGE_API_KEY"] = srv.URL, "tf_good"
			code := h.run("runs", "launch", "--tag", "smoke", "--repetitions", "2", "--label", "ci-42", "--wait")
			if code != tc.code || !strings.Contains(h.stdout.String(), tc.want) {
				t.Fatalf("code %d (want %d)\nstdout: %s\nstderr: %s", code, tc.code, h.stdout, h.stderr)
			}
			if !strings.Contains(h.stderr.String(), "0/2 runs settled") || !strings.Contains(h.stderr.String(), "2/2 runs settled") {
				t.Fatalf("progress: %s", h.stderr)
			}
			want := map[string]any{"scenarioIds": []any{"s1", "s2"}, "repetitions": float64(2), "label": "ci-42"}
			if fmt.Sprint(f.launched) != fmt.Sprint(want) {
				t.Fatalf("launch body = %v", f.launched)
			}
		})
	}
}

func TestWaitJSONAndTimeout(t *testing.T) {
	f, srv := newFake(t)
	f.runs = []map[string]any{run("r1", "completed", ptr("completed"), ptr(40), ptr(false))}
	h := newHarness(t)
	h.vars["TESTFORGE_URL"], h.vars["TESTFORGE_API_KEY"] = srv.URL, "tf_good"
	if code := h.run("runs", "wait", "--batch", "b1", "--json"); code != 2 {
		t.Fatalf("wait --json = %d: %s", code, h.stderr)
	}
	var out struct {
		BatchID string           `json:"batchId"`
		Passed  bool             `json:"passed"`
		Runs    []map[string]any `json:"runs"`
	}
	if err := json.Unmarshal(h.stdout.Bytes(), &out); err != nil || out.BatchID != "b1" || out.Passed || len(out.Runs) != 1 {
		t.Fatalf("wait json = %s (%v)", h.stdout, err)
	}

	f.runs, f.settleAt, f.polls = nil, 1<<30, 0
	if code := h.run("runs", "wait", "--batch", "b1", "--timeout", "30ms"); code != 1 || !strings.Contains(h.stderr.String(), "timed out after 30ms; batch b1 is still running") {
		t.Fatalf("timeout = %d: %s", code, h.stderr)
	}
	if code := h.run("runs", "wait"); code != 1 || !strings.Contains(h.stderr.String(), "--batch is required") {
		t.Fatalf("wait without batch = %d", code)
	}
}
