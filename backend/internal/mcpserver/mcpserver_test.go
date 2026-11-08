package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"testforge/internal/client"
)

func connect(t *testing.T, h http.HandlerFunc, ws WorkspaceFunc) *mcp.ClientSession {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := client.New(srv.URL, "tf_key")
	if err != nil {
		t.Fatal(err)
	}
	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := New(c, ws, "test").Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func acme(context.Context) (string, error) { return "acme", nil }

func text(t *testing.T, r *mcp.CallToolResult) string {
	t.Helper()
	if len(r.Content) != 1 {
		t.Fatalf("content = %#v", r.Content)
	}
	return r.Content[0].(*mcp.TextContent).Text
}

func TestTools(t *testing.T) {
	var gotPath, gotQuery, gotBody, gotAuth string
	cs := connect(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery, gotAuth = r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization")
		b := make([]byte, 1024)
		n, _ := r.Body.Read(b)
		gotBody = string(b[:n])
		switch r.URL.Path {
		case "/api/v1/workspaces/acme/runs/nope/judgement":
			w.WriteHeader(404)
			fmt.Fprint(w, `{"error":{"code":"not_found","message":"Run not found."}}`)
		case "/api/v1/workspaces/acme/runs":
			if r.Method == http.MethodPost {
				w.WriteHeader(201)
				fmt.Fprint(w, `{"id":"b1","total":2}`)
				return
			}
			fmt.Fprint(w, `{"items":[{"id":"r1","score":91}],"nextCursor":null}`)
		default:
			fmt.Fprint(w, `{"ok":true}`)
		}
	}, acme)
	ctx := context.Background()

	list, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var names, readOnly []string
	for _, tool := range list.Tools {
		names = append(names, tool.Name)
		if tool.Annotations != nil && tool.Annotations.ReadOnlyHint {
			readOnly = append(readOnly, tool.Name)
		}
	}
	sort.Strings(names)
	sort.Strings(readOnly)
	if strings.Join(names, ",") != "cancel_run,get_judgement,get_run,get_scenario,launch_runs,list_agents,list_runs,list_scenarios,rejudge_run,reports_overview,whoami" {
		t.Fatalf("tools = %v", names)
	}
	if strings.Join(readOnly, ",") != "get_judgement,get_run,get_scenario,list_agents,list_runs,list_scenarios,reports_overview,whoami" {
		t.Fatalf("read-only tools = %v", readOnly)
	}

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_runs", Arguments: map[string]any{"batchId": "b1", "status": "completed", "limit": 5}})
	if err != nil || res.IsError {
		t.Fatalf("list_runs = %v %v", res, err)
	}
	if text(t, res) != `{"items":[{"id":"r1","score":91}],"nextCursor":null}` || gotQuery != "batchId=b1&limit=5&status=completed" || gotAuth != "Bearer tf_key" {
		t.Fatalf("list_runs: %s ?%s %s", text(t, res), gotQuery, gotAuth)
	}

	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "launch_runs", Arguments: map[string]any{"scenarioIds": []string{"s1", "s2"}, "repetitions": 2}})
	if err != nil || res.IsError || text(t, res) != `{"id":"b1","total":2}` {
		t.Fatalf("launch_runs = %v %v", res, err)
	}
	var body map[string]any
	_ = json.Unmarshal([]byte(gotBody), &body)
	if fmt.Sprint(body) != "map[repetitions:2 scenarioIds:[s1 s2]]" {
		t.Fatalf("launch body = %s", gotBody)
	}

	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_judgement", Arguments: map[string]any{"runId": "nope"}})
	if err != nil || !res.IsError || text(t, res) != "Run not found." {
		t.Fatalf("get_judgement error = %v %v", res, err)
	}

	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "reports_overview", Arguments: map[string]any{"days": 7}})
	if err != nil || res.IsError || gotPath != "/api/v1/workspaces/acme/reports/overview" || gotQuery != "days=7" {
		t.Fatalf("reports_overview = %v %v %s?%s", res, err, gotPath, gotQuery)
	}

	// Required arguments are validated before the API is called.
	gotPath = ""
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_run", Arguments: map[string]any{}})
	if gotPath != "" || (err == nil && !res.IsError) {
		t.Fatalf("get_run without id reached %q: %v %v", gotPath, res, err)
	}
}

func TestWorkspaceAndAuthErrors(t *testing.T) {
	cs := connect(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		fmt.Fprint(w, `{"error":{"code":"unauthorized","message":"Authentication required."}}`)
	}, func(context.Context) (string, error) { return "", fmt.Errorf("the API key reaches no workspace") })
	ctx := context.Background()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_agents", Arguments: map[string]any{}})
	if err != nil || !res.IsError || text(t, res) != "the API key reaches no workspace" {
		t.Fatalf("workspace error = %v %v", res, err)
	}
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "whoami", Arguments: map[string]any{}})
	if err != nil || !res.IsError || !strings.Contains(text(t, res), "invalid, expired or revoked") {
		t.Fatalf("whoami 401 = %v %v", res, err)
	}
}
