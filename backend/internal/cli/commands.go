package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"testforge/internal/client"
)

func cmdLogin(ctx context.Context, e *Env, args []string) int {
	fs, g := newFlags(e, "login", "--url <api-url>", "Store an API key for an API URL; the key comes from TESTFORGE_API_KEY or stdin")
	if _, code, ok := parseExact(e, fs, args, 0); !ok {
		return code
	}
	url := firstNonEmpty(g.url, e.Getenv("TESTFORGE_URL"))
	if url == "" {
		fmt.Fprintln(e.Stderr, "testforge login: --url is required")
		fs.Usage()
		return ExitError
	}
	if _, err := client.ParseURL(url); err != nil {
		return fail(e, err)
	}
	key, err := readKey(e)
	if err != nil {
		return fail(e, err)
	}
	if key == "" {
		return fail(e, errors.New("no API key: pass it on stdin or in TESTFORGE_API_KEY"))
	}
	c, err := client.New(url, key)
	if err != nil {
		return fail(e, err)
	}
	c.UserAgent = "testforge-cli/" + Version
	raw, err := c.Me(ctx)
	if err != nil {
		return fail(e, err)
	}
	var me meView
	if err := json.Unmarshal(raw, &me); err != nil {
		return fail(e, err)
	}
	if me.AuthMethod != "api_key" || len(me.Workspaces) == 0 {
		return fail(e, errors.New("this credential is not a workspace API key"))
	}
	ws := firstNonEmpty(g.workspace, me.Workspaces[0].Slug)
	path, err := e.saveConfig(Config{URL: c.BaseURL(), APIKey: key, Workspace: ws})
	if err != nil {
		return fail(e, err)
	}
	fmt.Fprintf(e.Stdout, "Logged in to %s as %s (workspace %s).\nSaved to %s\n", c.BaseURL(), me.User.Name, ws, path)
	return ExitOK
}

func cmdLogout(_ context.Context, e *Env, args []string) int {
	fs, _ := newFlags(e, "logout", "", "Remove the stored API key")
	if _, code, ok := parseExact(e, fs, args, 0); !ok {
		return code
	}
	removed, err := e.removeConfig()
	if err != nil {
		return fail(e, err)
	}
	if removed {
		fmt.Fprintln(e.Stdout, "Logged out. The key still works until you revoke it in the web app.")
	} else {
		fmt.Fprintln(e.Stdout, "Not logged in.")
	}
	return ExitOK
}

func cmdWhoami(ctx context.Context, e *Env, args []string) int {
	fs, g := newFlags(e, "whoami", "", "Show the key's user, workspace and API URL")
	if _, code, ok := parseExact(e, fs, args, 0); !ok {
		return code
	}
	t, err := e.resolve(g)
	if err != nil {
		return fail(e, err)
	}
	raw, err := t.client.Me(ctx)
	if err != nil {
		return fail(e, err)
	}
	if g.json {
		printJSON(e.Stdout, raw)
		return ExitOK
	}
	var me meView
	if err := json.Unmarshal(raw, &me); err != nil {
		return fail(e, err)
	}
	fmt.Fprintf(e.Stdout, "User       %s <%s>\n", me.User.Name, me.User.Email)
	for _, w := range me.Workspaces {
		fmt.Fprintf(e.Stdout, "Workspace  %s (%s, %s)\n", w.Slug, w.Name, w.Role)
	}
	fmt.Fprintf(e.Stdout, "API        %s\nAuth       %s\n", t.client.BaseURL(), me.AuthMethod)
	return ExitOK
}

// simple runs a read that prints JSON, or a table via render.
func simple(ctx context.Context, e *Env, g *globals, call func(context.Context, *client.Client, string) (json.RawMessage, error), render func(json.RawMessage) error) int {
	t, ws, err := connect(ctx, e, g)
	if err != nil {
		return fail(e, err)
	}
	raw, err := call(ctx, t.client, ws)
	if err != nil {
		return fail(e, err)
	}
	if g.json || render == nil {
		printJSON(e.Stdout, raw)
		return ExitOK
	}
	if err := render(raw); err != nil {
		return fail(e, err)
	}
	return ExitOK
}

func cmdAgentsList(ctx context.Context, e *Env, args []string) int {
	fs, g := newFlags(e, "agents list", "", "List agents")
	if _, code, ok := parseExact(e, fs, args, 0); !ok {
		return code
	}
	return simple(ctx, e, g, func(ctx context.Context, c *client.Client, ws string) (json.RawMessage, error) {
		return c.ListAgents(ctx, ws)
	}, func(raw json.RawMessage) error {
		var page struct {
			Items []agentView `json:"items"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return err
		}
		rows := [][]string{}
		for _, a := range page.Items {
			rows = append(rows, []string{a.ID, clip(a.Name, 40), a.Modality, a.Codec.Type, clip(a.Transport.URL, 60)})
		}
		table(e.Stdout, "ID\tNAME\tMODALITY\tCODEC\tURL", rows)
		return nil
	})
}

func cmdAgentsGet(ctx context.Context, e *Env, args []string) int {
	fs, g := newFlags(e, "agents get", "<agent-id>", "Show an agent")
	pos, code, ok := parseExact(e, fs, args, 1)
	if !ok {
		return code
	}
	return simple(ctx, e, g, func(ctx context.Context, c *client.Client, ws string) (json.RawMessage, error) {
		return c.GetAgent(ctx, ws, pos[0])
	}, nil)
}

func cmdScenariosList(ctx context.Context, e *Env, args []string) int {
	fs, g := newFlags(e, "scenarios list", "[--agent id] [--tag t] [--group id|none] [--q text]", "List scenarios")
	var f client.ScenarioFilter
	fs.StringVar(&f.AgentID, "agent", "", "only scenarios of this agent id")
	fs.StringVar(&f.Tag, "tag", "", "only scenarios with this tag")
	fs.StringVar(&f.GroupID, "group", "", "only scenarios of this group id (`none` for ungrouped)")
	fs.StringVar(&f.Q, "q", "", "match name or goal")
	if _, code, ok := parseExact(e, fs, args, 0); !ok {
		return code
	}
	return simple(ctx, e, g, func(ctx context.Context, c *client.Client, ws string) (json.RawMessage, error) {
		return c.ListScenarios(ctx, ws, f)
	}, func(raw json.RawMessage) error {
		var page struct {
			Items []scenarioView `json:"items"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return err
		}
		rows := [][]string{}
		for _, s := range page.Items {
			rows = append(rows, []string{s.ID, clip(s.Name, 40), s.Modality, orDash(strings.Join(s.Tags, ",")), clip(s.Goal, 60)})
		}
		table(e.Stdout, "ID\tNAME\tMODALITY\tTAGS\tGOAL", rows)
		return nil
	})
}

func cmdScenariosGet(ctx context.Context, e *Env, args []string) int {
	fs, g := newFlags(e, "scenarios get", "<scenario-id>", "Show a scenario")
	pos, code, ok := parseExact(e, fs, args, 1)
	if !ok {
		return code
	}
	return simple(ctx, e, g, func(ctx context.Context, c *client.Client, ws string) (json.RawMessage, error) {
		return c.GetScenario(ctx, ws, pos[0])
	}, nil)
}

func cmdRunsList(ctx context.Context, e *Env, args []string) int {
	fs, g := newFlags(e, "runs list", "[--batch id] [--scenario id] [--agent id] [--status s] [--limit n]", "List runs, newest first")
	var f client.RunFilter
	fs.StringVar(&f.BatchID, "batch", "", "only runs of this batch id")
	fs.StringVar(&f.ScenarioID, "scenario", "", "only runs of this scenario id")
	fs.StringVar(&f.AgentID, "agent", "", "only runs of this agent id")
	fs.StringVar(&f.Status, "status", "", "queued|running|completed|failed|cancelled|interrupted")
	fs.IntVar(&f.Limit, "limit", 20, "how many runs (1-200)")
	if _, code, ok := parseExact(e, fs, args, 0); !ok {
		return code
	}
	return simple(ctx, e, g, func(ctx context.Context, c *client.Client, ws string) (json.RawMessage, error) {
		return c.ListRuns(ctx, ws, f)
	}, func(raw json.RawMessage) error {
		var page runPage
		if err := json.Unmarshal(raw, &page); err != nil {
			return err
		}
		table(e.Stdout, runHeader, runRows(page.Items))
		if page.NextCursor != nil {
			fmt.Fprintln(e.Stderr, "More runs exist: raise --limit or narrow the filters.")
		}
		return nil
	})
}

func cmdRunsGet(ctx context.Context, e *Env, args []string) int {
	fs, g := newFlags(e, "runs get", "<run-id>", "Show a run: metrics, evaluation and transcript")
	pos, code, ok := parseExact(e, fs, args, 1)
	if !ok {
		return code
	}
	t, ws, err := connect(ctx, e, g)
	if err != nil {
		return fail(e, err)
	}
	raw, err := t.client.GetRun(ctx, ws, pos[0])
	if err != nil {
		return fail(e, err)
	}
	if g.json {
		printJSON(e.Stdout, raw)
		return ExitOK
	}
	var r runView
	if err := json.Unmarshal(raw, &r); err != nil {
		return fail(e, err)
	}
	printRun(e.Stdout, r)
	if r.JudgementStatus != nil {
		jraw, err := t.client.GetJudgement(ctx, ws, r.ID)
		if err != nil {
			return fail(e, err)
		}
		var j judgementView
		if err := json.Unmarshal(jraw, &j); err != nil {
			return fail(e, err)
		}
		printJudgement(e.Stdout, j)
	}
	if len(r.Transcript) > 0 {
		fmt.Fprintln(e.Stdout, "\nTranscript")
		user, agent := 0, 0
		for _, turn := range r.Transcript {
			ref := ""
			if turn.Role == "agent" {
				agent++
				ref = fmt.Sprintf("A%d", agent)
			} else {
				user++
				ref = fmt.Sprintf("U%d", user)
			}
			meta := fmt.Sprintf("%.1fs", float64(turn.StartMs)/1000)
			if turn.LatencyMs != nil {
				meta += fmt.Sprintf(", %dms", *turn.LatencyMs)
			}
			if turn.Interrupted {
				meta += ", interrupted"
			}
			fmt.Fprintf(e.Stdout, "  %-4s %-6s %s  (%s)\n", ref, turn.Role, strings.Join(strings.Fields(turn.Text), " "), meta)
		}
	}
	return ExitOK
}

func cmdRunsCancel(ctx context.Context, e *Env, args []string) int {
	fs, g := newFlags(e, "runs cancel", "<run-id>", "Cancel a queued or running run")
	pos, code, ok := parseExact(e, fs, args, 1)
	if !ok {
		return code
	}
	return simple(ctx, e, g, func(ctx context.Context, c *client.Client, ws string) (json.RawMessage, error) {
		return c.CancelRun(ctx, ws, pos[0])
	}, func(raw json.RawMessage) error {
		var r runView
		if err := json.Unmarshal(raw, &r); err != nil {
			return err
		}
		fmt.Fprintf(e.Stdout, "Run %s is %s.\n", r.ID, r.Status)
		return nil
	})
}

func cmdRunsJudgement(ctx context.Context, e *Env, args []string) int {
	fs, g := newFlags(e, "runs judgement", "<run-id>", "Show a run's evaluation")
	pos, code, ok := parseExact(e, fs, args, 1)
	if !ok {
		return code
	}
	return simple(ctx, e, g, func(ctx context.Context, c *client.Client, ws string) (json.RawMessage, error) {
		return c.GetJudgement(ctx, ws, pos[0])
	}, func(raw json.RawMessage) error {
		var j judgementView
		if err := json.Unmarshal(raw, &j); err != nil {
			return err
		}
		printJudgement(e.Stdout, j)
		return nil
	})
}

func cmdRunsRejudge(ctx context.Context, e *Env, args []string) int {
	fs, g := newFlags(e, "runs rejudge", "<run-id>", "Evaluate a run again")
	pos, code, ok := parseExact(e, fs, args, 1)
	if !ok {
		return code
	}
	return simple(ctx, e, g, func(ctx context.Context, c *client.Client, ws string) (json.RawMessage, error) {
		return c.Rejudge(ctx, ws, pos[0])
	}, func(raw json.RawMessage) error {
		var j judgementView
		if err := json.Unmarshal(raw, &j); err != nil {
			return err
		}
		fmt.Fprintf(e.Stdout, "Evaluation %s. Follow it with: testforge runs judgement %s\n", j.Status, pos[0])
		return nil
	})
}

func cmdReports(ctx context.Context, e *Env, args []string) int {
	fs, g := newFlags(e, "reports", "[--days 7|30|90] [--agent id] [--scenario id]", "Evaluation report")
	var f client.ReportFilter
	fs.IntVar(&f.Days, "days", 30, "period: 7, 30 or 90 days")
	fs.StringVar(&f.AgentID, "agent", "", "only runs of this agent id")
	fs.StringVar(&f.ScenarioID, "scenario", "", "only runs of this scenario id")
	if _, code, ok := parseExact(e, fs, args, 0); !ok {
		return code
	}
	return simple(ctx, e, g, func(ctx context.Context, c *client.Client, ws string) (json.RawMessage, error) {
		return c.ReportsOverview(ctx, ws, f)
	}, func(raw json.RawMessage) error {
		var r reportView
		if err := json.Unmarshal(raw, &r); err != nil {
			return err
		}
		total := 0
		for _, n := range r.Totals {
			total += n
		}
		w := e.Stdout
		fmt.Fprintf(w, "Last %d days\n", r.Days)
		fmt.Fprintf(w, "Runs       %d", total)
		for _, k := range sortedKeys(r.Totals) {
			fmt.Fprintf(w, ", %s %d", k, r.Totals[k])
		}
		fmt.Fprintln(w)
		fmt.Fprintf(w, "Evaluated  %d, average score %s, pass rate %s\n", r.Judged, num(r.AvgScore, ""), pct(r.PassRate))
		fmt.Fprintf(w, "Latency    p50 %s, p95 %s\nLLM tokens %d\n", num(r.LatencyP50Ms, "ms"), num(r.LatencyP95Ms, "ms"), r.LLMTokens)
		if len(r.WeakestCriteria) > 0 {
			fmt.Fprintln(w, "\nWeakest criteria")
			rows := [][]string{}
			for _, c := range r.WeakestCriteria {
				rows = append(rows, []string{fmt.Sprintf("%.0f", c.AvgScore), fmt.Sprintf("%.0f%%", c.FailRate*100), fmt.Sprint(c.Judged), clip(c.ScenarioName, 30), clip(c.Text, 60)})
			}
			table(w, "AVG\tFAIL\tJUDGED\tSCENARIO\tCRITERION", rows)
		}
		if len(r.Scenarios) > 0 {
			fmt.Fprintln(w, "\nScenarios")
			rows := [][]string{}
			for _, s := range r.Scenarios {
				rows = append(rows, []string{clip(s.ScenarioName, 40), clip(s.AgentName, 24), fmt.Sprint(s.Runs), num(s.AvgScore, ""), pct(s.PassRate), s.LastRunAt.Local().Format("2006-01-02 15:04")})
			}
			table(w, "SCENARIO\tAGENT\tRUNS\tAVG\tPASS\tLAST RUN", rows)
		}
		if len(r.FailureTags) > 0 {
			fmt.Fprintln(w, "\nFailure tags")
			for _, k := range sortedKeys(r.FailureTags) {
				fmt.Fprintf(w, "  %-34s %d\n", k, r.FailureTags[k])
			}
		}
		return nil
	})
}

func cmdVersion(_ context.Context, e *Env, args []string) int {
	fs, _ := newFlags(e, "version", "", "Print the version")
	if _, code, ok := parseExact(e, fs, args, 0); !ok {
		return code
	}
	fmt.Fprintln(e.Stdout, "testforge "+Version)
	return ExitOK
}

func cmdMCP(ctx context.Context, e *Env, args []string) int {
	fs, g := newFlags(e, "mcp", "", "Serve the Model Context Protocol over stdio for AI assistants")
	if _, code, ok := parseExact(e, fs, args, 0); !ok {
		return code
	}
	if e.ServeMCP == nil {
		return fail(e, errors.New("MCP is not available in this build"))
	}
	t, err := e.resolve(g)
	if err != nil {
		return fail(e, err)
	}
	// The workspace is resolved on first use, so a client can start the
	// server while the API is still coming up.
	if err := e.ServeMCP(ctx, t.client, t.workspaceOf); err != nil && ctx.Err() == nil {
		return fail(e, err)
	}
	return ExitOK
}
