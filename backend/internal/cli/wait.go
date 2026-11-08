package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"time"

	"testforge/internal/client"
)

type idsFlag []string

func (s *idsFlag) String() string     { return fmt.Sprint(*s) }
func (s *idsFlag) Set(v string) error { *s = append(*s, v); return nil }

func cmdRunsLaunch(ctx context.Context, e *Env, args []string) int {
	fs, g := newFlags(e, "runs launch", "(--scenario id ... | --agent id | --tag t | --group id) [--wait]",
		"Launch a batch of runs: the given scenarios, or every scenario matching --agent/--tag/--group")
	var ids idsFlag
	var sel client.ScenarioFilter
	fs.Var(&ids, "scenario", "scenario id (repeatable)")
	fs.StringVar(&sel.AgentID, "agent", "", "every scenario of this agent id")
	fs.StringVar(&sel.Tag, "tag", "", "every scenario with this tag")
	fs.StringVar(&sel.GroupID, "group", "", "every scenario of this group id")
	reps := fs.Int("repetitions", 0, "runs per scenario, 1-10 (default 1)")
	conc := fs.Int("concurrency", 0, "runs of this batch at the same time, 1-10 (default 3)")
	label := fs.String("label", "", "batch label, e.g. a commit or build number")
	wait := waitFlags(fs)
	if _, code, ok := parseExact(e, fs, args, 0); !ok {
		return code
	}
	filtered := sel.AgentID != "" || sel.Tag != "" || sel.GroupID != ""
	if len(ids) == 0 && !filtered {
		fmt.Fprintln(e.Stderr, "testforge runs launch: pass --scenario, or select scenarios with --agent, --tag or --group")
		fs.Usage()
		return ExitError
	}
	if len(ids) > 0 && filtered {
		fmt.Fprintln(e.Stderr, "testforge runs launch: use either --scenario or --agent/--tag/--group, not both")
		return ExitError
	}
	t, ws, err := connect(ctx, e, g)
	if err != nil {
		return fail(e, err)
	}
	if filtered {
		raw, err := t.client.ListScenarios(ctx, ws, sel)
		if err != nil {
			return fail(e, err)
		}
		var page struct {
			Items []scenarioView `json:"items"`
		}
		if err := json.Unmarshal(raw, &page); err != nil {
			return fail(e, err)
		}
		for _, s := range page.Items {
			ids = append(ids, s.ID)
		}
		if len(ids) == 0 {
			return fail(e, errors.New("no scenarios match the selection"))
		}
	}
	req := client.LaunchRequest{ScenarioIDs: ids, Label: *label}
	if *reps != 0 {
		req.Repetitions = reps
	}
	if *conc != 0 {
		req.Concurrency = conc
	}
	raw, err := t.client.LaunchRuns(ctx, ws, req)
	if err != nil {
		return fail(e, err)
	}
	var batch struct {
		ID    string `json:"id"`
		Total int    `json:"total"`
	}
	if err := json.Unmarshal(raw, &batch); err != nil {
		return fail(e, err)
	}
	if !wait.enabled {
		if g.json {
			printJSON(e.Stdout, raw)
		} else {
			fmt.Fprintf(e.Stdout, "Launched batch %s (%d runs).\nWait for it with: testforge runs wait --batch %s\n", batch.ID, batch.Total, batch.ID)
		}
		return ExitOK
	}
	fmt.Fprintf(e.Stderr, "Launched batch %s (%d runs).\n", batch.ID, batch.Total)
	return waitBatch(ctx, e, t.client, ws, batch.ID, wait.timeout, g.json)
}

type waitOpts struct {
	enabled bool
	timeout time.Duration
}

func waitFlags(fs *flag.FlagSet) *waitOpts {
	w := &waitOpts{}
	fs.BoolVar(&w.enabled, "wait", false, "wait for the runs and their evaluations; exit 2 unless all pass")
	fs.DurationVar(&w.timeout, "timeout", 30*time.Minute, "how long --wait waits")
	return w
}

func cmdRunsWait(ctx context.Context, e *Env, args []string) int {
	fs, g := newFlags(e, "runs wait", "--batch <batch-id>", "Wait for a batch's runs and evaluations; exit 2 unless all pass")
	batch := fs.String("batch", "", "batch id (required)")
	timeout := fs.Duration("timeout", 30*time.Minute, "how long to wait")
	if _, code, ok := parseExact(e, fs, args, 0); !ok {
		return code
	}
	if *batch == "" {
		fmt.Fprintln(e.Stderr, "testforge runs wait: --batch is required")
		fs.Usage()
		return ExitError
	}
	t, ws, err := connect(ctx, e, g)
	if err != nil {
		return fail(e, err)
	}
	return waitBatch(ctx, e, t.client, ws, *batch, *timeout, g.json)
}

var terminalRun = map[string]bool{"completed": true, "failed": true, "cancelled": true, "interrupted": true}
var terminalJudgement = map[string]bool{"completed": true, "failed": true, "skipped": true}

// settled reports whether a run will not change any more: it ended and, when
// it completed, its evaluation ended too.
func settled(r runView) bool {
	if !terminalRun[r.Status] {
		return false
	}
	if r.Status != "completed" {
		return true
	}
	return r.JudgementStatus != nil && terminalJudgement[*r.JudgementStatus]
}

// passed is the CI verdict of one settled run: it completed, its evaluation
// did not fail and, when judged, it passed. A skipped evaluation (no judge
// configured) does not fail the build.
func passed(r runView) bool {
	if r.Status != "completed" || r.JudgementStatus == nil || *r.JudgementStatus == "failed" {
		return false
	}
	return r.Passed == nil || *r.Passed || *r.JudgementStatus == "skipped"
}

func listBatch(ctx context.Context, c *client.Client, ws, batchID string) ([]runView, error) {
	var all []runView
	f := client.RunFilter{BatchID: batchID, Limit: 200}
	for {
		raw, err := c.ListRuns(ctx, ws, f)
		if err != nil {
			return nil, err
		}
		var page runPage
		if err := json.Unmarshal(raw, &page); err != nil {
			return nil, err
		}
		all = append(all, page.Items...)
		if page.NextCursor == nil || len(page.Items) == 0 {
			return all, nil
		}
		f.Before = *page.NextCursor
	}
}

func waitBatch(ctx context.Context, e *Env, c *client.Client, ws, batchID string, timeout time.Duration, asJSON bool) int {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	last, warned := "", false
	for {
		runs, err := listBatch(ctx, c, ws, batchID)
		var apiErr *client.Error
		switch {
		case err == nil:
			done := 0
			for _, r := range runs {
				if settled(r) {
					done++
				}
			}
			progress := fmt.Sprintf("%d/%d runs settled", done, len(runs))
			if progress != last {
				fmt.Fprintln(e.Stderr, progress)
				last = progress
			}
			if len(runs) > 0 && done == len(runs) {
				return report(e, batchID, runs, asJSON)
			}
		case errors.As(err, &apiErr) || ctx.Err() != nil:
			if ctx.Err() == nil {
				return fail(e, err)
			}
		default:
			// Network trouble: keep polling until the deadline.
			if !warned {
				fmt.Fprintf(e.Stderr, "warning: %s; retrying\n", err)
				warned = true
			}
		}
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fail(e, fmt.Errorf("timed out after %s; batch %s is still running (testforge runs wait --batch %s)", timeout, batchID, batchID))
			}
			return fail(e, ctx.Err())
		case <-time.After(e.PollEvery):
		}
	}
}

func report(e *Env, batchID string, runs []runView, asJSON bool) int {
	ok := 0
	for _, r := range runs {
		if passed(r) {
			ok++
		}
	}
	allPassed := ok == len(runs)
	if asJSON {
		b, _ := json.MarshalIndent(map[string]any{"batchId": batchID, "passed": allPassed, "runs": runs}, "", "  ")
		fmt.Fprintln(e.Stdout, string(b))
	} else {
		rows := make([][]string, 0, len(runs))
		for _, r := range runs {
			verdict := "PASS"
			if !passed(r) {
				verdict = "FAIL"
			}
			rows = append(rows, []string{verdict, r.ID, r.Status, strOr(r.JudgementStatus), intOr(r.Score), clip(r.ScenarioName, 40), clip(r.PersonaName, 24)})
		}
		table(e.Stdout, "RESULT\tRUN\tSTATUS\tEVALUATION\tSCORE\tSCENARIO\tPERSONA", rows)
		fmt.Fprintf(e.Stdout, "\n%d of %d runs passed.\n", ok, len(runs))
	}
	if allPassed {
		return ExitOK
	}
	return ExitFailed
}
