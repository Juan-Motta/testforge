// Package mcpserver exposes a TestForge workspace to AI assistants over the
// Model Context Protocol. Every tool calls the public API with the workspace
// API key and returns the API's JSON.
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"testforge/internal/client"
)

// WorkspaceFunc returns the workspace slug the tools act on.
type WorkspaceFunc func(context.Context) (string, error)

const instructions = `TestForge tests voice and text AI agents by running scenarios (a simulated user with a goal, persona and criteria) against them and evaluating each run with an LLM judge.
Typical flow: list_scenarios → launch_runs → list_runs with the batchId until every run is completed/failed and its judgementStatus is completed/failed/skipped → get_judgement for scores, failed criteria and suggestions. reports_overview summarizes recent results.
Launching runs calls the agent under test and the configured LLM providers, which can cost money: confirm with the user before launching many runs.`

// New returns an MCP server for one API client.
func New(c *client.Client, workspace WorkspaceFunc, version string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "testforge", Title: "TestForge", Version: version}, &mcp.ServerOptions{Instructions: instructions})
	t := tools{c: c, ws: workspace}

	read := func(title string) *mcp.ToolAnnotations {
		return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, OpenWorldHint: ptr(false)}
	}
	mcp.AddTool(s, &mcp.Tool{Name: "whoami", Description: "The API key's user and workspace.", Annotations: read("Who am I")},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
			return result(c.Me(ctx))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "list_agents", Description: "List the workspace's agents under test (id, name, modality, transport, codec).", Annotations: read("List agents")},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
			return t.call(ctx, func(ws string) (json.RawMessage, error) { return c.ListAgents(ctx, ws) })
		})
	mcp.AddTool(s, &mcp.Tool{Name: "list_scenarios", Description: "List test scenarios, optionally filtered.", Annotations: read("List scenarios")},
		func(ctx context.Context, _ *mcp.CallToolRequest, in scenarioFilter) (*mcp.CallToolResult, any, error) {
			return t.call(ctx, func(ws string) (json.RawMessage, error) {
				return c.ListScenarios(ctx, ws, client.ScenarioFilter{AgentID: in.AgentID, Tag: in.Tag, GroupID: in.GroupID, Q: in.Query})
			})
		})
	mcp.AddTool(s, &mcp.Tool{Name: "get_scenario", Description: "Read one scenario: goal, persona, facts, criteria and end conditions.", Annotations: read("Get scenario")},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, any, error) {
			return t.call(ctx, func(ws string) (json.RawMessage, error) { return c.GetScenario(ctx, ws, in.ID) })
		})
	mcp.AddTool(s, &mcp.Tool{Name: "launch_runs", Description: "Queue a batch with one run per scenario × repetition. Returns the batch; list_runs with its id follows progress.",
		Annotations: &mcp.ToolAnnotations{Title: "Launch runs", DestructiveHint: ptr(false), OpenWorldHint: ptr(true)}},
		func(ctx context.Context, _ *mcp.CallToolRequest, in launchInput) (*mcp.CallToolResult, any, error) {
			return t.call(ctx, func(ws string) (json.RawMessage, error) {
				return c.LaunchRuns(ctx, ws, client.LaunchRequest{ScenarioIDs: in.ScenarioIDs, Repetitions: in.Repetitions, Concurrency: in.Concurrency, Label: in.Label})
			})
		})
	mcp.AddTool(s, &mcp.Tool{Name: "list_runs", Description: "List runs newest first with status, evaluation status, score and passed. nextCursor pages back.", Annotations: read("List runs")},
		func(ctx context.Context, _ *mcp.CallToolRequest, in runFilter) (*mcp.CallToolResult, any, error) {
			return t.call(ctx, func(ws string) (json.RawMessage, error) {
				return c.ListRuns(ctx, ws, client.RunFilter{BatchID: in.BatchID, ScenarioID: in.ScenarioID, AgentID: in.AgentID, Status: in.Status, Limit: in.Limit, Before: in.Before})
			})
		})
	mcp.AddTool(s, &mcp.Tool{Name: "get_run", Description: "Read a run with its transcript, timings and metrics.", Annotations: read("Get run")},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, any, error) {
			return t.call(ctx, func(ws string) (json.RawMessage, error) { return c.GetRun(ctx, ws, in.ID) })
		})
	mcp.AddTool(s, &mcp.Tool{Name: "get_judgement", Description: "Read a run's evaluation: score, passed, per-criterion verdicts with evidence, suggestions and trace stats.", Annotations: read("Get evaluation")},
		func(ctx context.Context, _ *mcp.CallToolRequest, in runIDInput) (*mcp.CallToolResult, any, error) {
			return t.call(ctx, func(ws string) (json.RawMessage, error) { return c.GetJudgement(ctx, ws, in.RunID) })
		})
	mcp.AddTool(s, &mcp.Tool{Name: "rejudge_run", Description: "Evaluate a completed run again with the scenario's current judge.",
		Annotations: &mcp.ToolAnnotations{Title: "Re-evaluate run", DestructiveHint: ptr(true), IdempotentHint: true, OpenWorldHint: ptr(true)}},
		func(ctx context.Context, _ *mcp.CallToolRequest, in runIDInput) (*mcp.CallToolResult, any, error) {
			return t.call(ctx, func(ws string) (json.RawMessage, error) { return c.Rejudge(ctx, ws, in.RunID) })
		})
	mcp.AddTool(s, &mcp.Tool{Name: "cancel_run", Description: "Cancel a queued or running run.",
		Annotations: &mcp.ToolAnnotations{Title: "Cancel run", DestructiveHint: ptr(true), IdempotentHint: true, OpenWorldHint: ptr(false)}},
		func(ctx context.Context, _ *mcp.CallToolRequest, in idInput) (*mcp.CallToolResult, any, error) {
			return t.call(ctx, func(ws string) (json.RawMessage, error) { return c.CancelRun(ctx, ws, in.ID) })
		})
	mcp.AddTool(s, &mcp.Tool{Name: "reports_overview", Description: "Evaluation report for a period: totals, average score, pass rate, latency, trend, weakest criteria, per-scenario results and failure tags.", Annotations: read("Reports overview")},
		func(ctx context.Context, _ *mcp.CallToolRequest, in reportInput) (*mcp.CallToolResult, any, error) {
			return t.call(ctx, func(ws string) (json.RawMessage, error) {
				return c.ReportsOverview(ctx, ws, client.ReportFilter{Days: in.Days, AgentID: in.AgentID, ScenarioID: in.ScenarioID})
			})
		})
	return s
}

type scenarioFilter struct {
	AgentID string `json:"agentId,omitempty" jsonschema:"only scenarios of this agent id"`
	Tag     string `json:"tag,omitempty" jsonschema:"only scenarios with this tag"`
	GroupID string `json:"groupId,omitempty" jsonschema:"only scenarios of this group id, or none for ungrouped"`
	Query   string `json:"query,omitempty" jsonschema:"case-insensitive match on name or goal"`
}

type idInput struct {
	ID string `json:"id" jsonschema:"the id"`
}

type runIDInput struct {
	RunID string `json:"runId" jsonschema:"the run id"`
}

type launchInput struct {
	ScenarioIDs []string `json:"scenarioIds" jsonschema:"scenario ids (1-100)"`
	Repetitions *int     `json:"repetitions,omitempty" jsonschema:"runs per scenario, 1-10 (default 1)"`
	Concurrency *int     `json:"concurrency,omitempty" jsonschema:"runs of this batch at the same time, 1-10 (default 3)"`
	Label       string   `json:"label,omitempty" jsonschema:"batch label"`
}

type runFilter struct {
	BatchID    string `json:"batchId,omitempty" jsonschema:"only runs of this batch id"`
	ScenarioID string `json:"scenarioId,omitempty" jsonschema:"only runs of this scenario id"`
	AgentID    string `json:"agentId,omitempty" jsonschema:"only runs of this agent id"`
	Status     string `json:"status,omitempty" jsonschema:"queued, running, completed, failed, cancelled or interrupted"`
	Limit      int    `json:"limit,omitempty" jsonschema:"page size 1-200 (default 50)"`
	Before     string `json:"before,omitempty" jsonschema:"nextCursor of the previous page"`
}

type reportInput struct {
	Days       int    `json:"days,omitempty" jsonschema:"7, 30 or 90 (default 30)"`
	AgentID    string `json:"agentId,omitempty" jsonschema:"only runs of this agent id"`
	ScenarioID string `json:"scenarioId,omitempty" jsonschema:"only runs of this scenario id"`
}

type tools struct {
	c  *client.Client
	ws WorkspaceFunc
}

func (t tools) call(ctx context.Context, fn func(ws string) (json.RawMessage, error)) (*mcp.CallToolResult, any, error) {
	ws, err := t.ws(ctx)
	if err != nil {
		return toolError(err), nil, nil
	}
	return result(fn(ws))
}

// result turns an API answer into a tool result: the JSON as text, or the
// API's message as a tool error the model can read and act on.
func result(raw json.RawMessage, err error) (*mcp.CallToolResult, any, error) {
	if err != nil {
		return toolError(err), nil, nil
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(raw)}}}, nil, nil
}

func toolError(err error) *mcp.CallToolResult {
	msg := err.Error()
	var apiErr *client.Error
	if errors.As(err, &apiErr) && apiErr.Status == 401 {
		msg += " (the TestForge API key is invalid, expired or revoked)"
	}
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: msg}}}
}

func ptr[T any](v T) *T { return &v }

// Serve runs the server over stdio until the client disconnects.
func Serve(ctx context.Context, c *client.Client, workspace WorkspaceFunc, version string) error {
	return New(c, workspace, version).Run(ctx, &mcp.StdioTransport{})
}
