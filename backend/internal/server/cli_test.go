package server_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"testforge/internal/cli"
	tfclient "testforge/internal/client"
	"testforge/internal/mcpserver"
	"testforge/internal/run"
)

// TestCLIAndMCPAgainstTheAPI drives the real API with a workspace API key:
// login, launch --wait (pass → 0, fail → 2), reports, and MCP tools.
func TestCLIAndMCPAgainstTheAPI(t *testing.T) {
	f := newRunFixture(t)
	judgeLLM := newFakeLLM(t, "placeholder")
	sc := judgedScenario(t, f, false, judgeLLM)
	judgeLLM.decisions = []string{judgeAnswer(sc["criteria"].([]any), 5)}
	startWorkerWith(t, f.e, &fakeEngine{}, 2, &run.JudgeOptions{Concurrency: 1, TraceWait: 0, AllowPrivate: true})
	secret := createKey(t, f.owner, f.slug)["secret"].(string)

	var stdout, stderr bytes.Buffer
	vars := map[string]string{"XDG_CONFIG_HOME": t.TempDir()}
	env := &cli.Env{Stdout: &stdout, Stderr: &stderr, Getenv: func(k string) string { return vars[k] }, PollEvery: 100 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := func(stdin string, args ...string) int {
		stdout.Reset()
		stderr.Reset()
		env.Stdin = strings.NewReader(stdin)
		return cli.Main(ctx, args, env)
	}

	if code := cmd(secret+"\n", "login", "--url", f.e.srv.URL); code != 0 || !strings.Contains(stdout.String(), "(workspace "+f.slug+")") {
		t.Fatalf("login = %d: %s %s", code, stdout.String(), stderr.String())
	}
	if code := cmd("", "scenarios", "list"); code != 0 || !strings.Contains(stdout.String(), "Judged hours") || !strings.Contains(stdout.String(), sc["id"].(string)) {
		t.Fatalf("scenarios list = %d: %s %s", code, stdout.String(), stderr.String())
	}
	if code := cmd("", "runs", "launch", "--scenario", sc["id"].(string), "--label", "ci-1", "--wait"); code != 0 || !strings.Contains(stdout.String(), "1 of 1 runs passed.") {
		t.Fatalf("passing launch = %d: %s %s", code, stdout.String(), stderr.String())
	}

	judgeLLM.decisions = []string{judgeAnswer(sc["criteria"].([]any), 1)}
	if code := cmd("", "runs", "launch", "--scenario", sc["id"].(string), "--wait"); code != 2 || !strings.Contains(stdout.String(), "0 of 1 runs passed.") {
		t.Fatalf("failing launch = %d: %s %s", code, stdout.String(), stderr.String())
	}
	failedRun := strings.Fields(strings.Split(stdout.String(), "\n")[1])[1]
	if code := cmd("", "runs", "get", failedRun); code != 0 || !strings.Contains(stdout.String(), "Evaluation completed (judge judge-model)") ||
		!strings.Contains(stdout.String(), "Score      25, passed no (a must criterion failed)") || !strings.Contains(stdout.String(), "Transcript") {
		t.Fatalf("runs get = %d: %s %s", code, stdout.String(), stderr.String())
	}
	if code := cmd("", "reports", "--days", "7"); code != 0 || !strings.Contains(stdout.String(), "Evaluated  2, average score 62, pass rate 50%") {
		t.Fatalf("reports = %d: %s %s", code, stdout.String(), stderr.String())
	}
	// An invalid request prints the API message and exits 1.
	if code := cmd("", "reports", "--days", "5"); code != 1 || !strings.HasPrefix(stderr.String(), "error: ") {
		t.Fatalf("invalid report = %d: %s", code, stderr.String())
	}

	// MCP over the same key.
	c, err := tfclient.New(f.e.srv.URL, secret)
	if err != nil {
		t.Fatal(err)
	}
	ct, st := mcp.NewInMemoryTransports()
	if _, err := mcpserver.New(c, func(context.Context) (string, error) { return f.slug, nil }, "test").Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_judgement", Arguments: map[string]any{"runId": failedRun}})
	if err != nil || res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, `"passed":false`) {
		t.Fatalf("get_judgement = %v %v", res, err)
	}
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_runs", Arguments: map[string]any{"scenarioId": sc["id"]}})
	if err != nil || res.IsError || strings.Count(res.Content[0].(*mcp.TextContent).Text, `"batchId"`) != 2 {
		t.Fatalf("list_runs = %v %v", res, err)
	}

	// Revoking the key stops both.
	keys := f.owner.do("GET", f.path("/api-keys"), nil).body["items"].([]any)
	f.owner.do("DELETE", f.path("/api-keys/"+keys[0].(map[string]any)["id"].(string)), nil)
	if code := cmd("", "whoami"); code != 1 || !strings.Contains(stderr.String(), "invalid, expired or revoked") {
		t.Fatalf("whoami after revoke = %d: %s", code, stderr.String())
	}
}
