package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"

	"testforge/internal/judge"
	"testforge/internal/langfuse"
	"testforge/internal/llm"
)

const judgeLease = 10 * time.Minute

// JudgeOptions configure judging in a worker.
type JudgeOptions struct {
	Concurrency  int
	TraceWait    time.Duration // how long to wait for traces to stabilize
	TracePoll    time.Duration // how often to re-poll Langfuse
	AllowPrivate bool
	Samples      int
}

// EnableJudging makes the worker also process judgements.
func (w *Worker) EnableJudging(o JudgeOptions) {
	if o.Concurrency < 1 {
		o.Concurrency = 2
	}
	if o.TraceWait == 0 {
		o.TraceWait = 3 * time.Minute
	}
	if o.TracePoll == 0 {
		o.TracePoll = 20 * time.Second
	}
	if o.Samples == 0 {
		o.Samples = 3
	}
	w.judging = &o
	w.judgeSlots = make(chan struct{}, o.Concurrency)
}

func (w *Worker) fillJudgements(ctx context.Context) {
	if w.judging == nil {
		return
	}
	for {
		select {
		case w.judgeSlots <- struct{}{}:
		default:
			return
		}
		var runID string
		err := w.pool.QueryRow(ctx,
			`UPDATE judgements SET status = 'running', lease_owner = $1, lease_until = now() + make_interval(secs => $2), updated_at = now()
			 WHERE run_id = (SELECT run_id FROM judgements WHERE status IN ('queued', 'waiting_traces') AND next_attempt_at <= now()
			                 ORDER BY next_attempt_at, run_id FOR UPDATE SKIP LOCKED LIMIT 1)
			 RETURNING run_id`, w.owner, judgeLease.Seconds()).Scan(&runID)
		if err != nil {
			<-w.judgeSlots
			if !errors.Is(err, pgx.ErrNoRows) && ctx.Err() == nil {
				slog.Error("claim judgement", "err", err)
			}
			return
		}
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			defer func() { <-w.judgeSlots }()
			w.judgeRun(ctx, runID)
		}()
	}
}

// reapJudgements requeues judgements whose worker vanished; judging is idempotent.
func (w *Worker) reapJudgements(ctx context.Context) {
	if w.judging == nil {
		return
	}
	_, _ = w.pool.Exec(ctx,
		`UPDATE judgements SET status = 'queued', lease_owner = NULL, lease_until = NULL, next_attempt_at = now(), updated_at = now()
		 WHERE status = 'running' AND lease_until < now()`)
}

type judgeJob struct {
	workspaceID string
	snap        Snapshot
	transcript  []Turn
	traceID     *string
	scenarioID  *string
	started     *time.Time
	finished    *time.Time
	traceCount  *int
	traceSince  *time.Time
}

func (w *Worker) judgeRun(ctx context.Context, runID string) {
	var j judgeJob
	var raw, transcript []byte
	err := w.pool.QueryRow(ctx,
		`SELECT r.workspace_id, r.snapshot, r.transcript, r.trace_session_id, r.scenario_id, r.started_at, r.finished_at,
		   g.trace_count, g.trace_since
		 FROM runs r JOIN judgements g ON g.run_id = r.id WHERE r.id = $1`, runID,
	).Scan(&j.workspaceID, &raw, &transcript, &j.traceID, &j.scenarioID, &j.started, &j.finished, &j.traceCount, &j.traceSince)
	if err == nil {
		err = json.Unmarshal(raw, &j.snap)
	}
	if err == nil {
		err = json.Unmarshal(transcript, &j.transcript)
	}
	if err != nil {
		w.finishJudgement(runID, JudgeFailed, "Could not load the run: "+err.Error(), "", nil, nil)
		return
	}

	// The scenario's current judge settings win, so a judge configured after
	// the run can still evaluate it; the snapshot is the fallback.
	connID, model := j.snap.Scenario.Judge.ConnectionID, j.snap.Scenario.Judge.Model
	if j.scenarioID != nil {
		var cur *string
		var curModel string
		if err := w.pool.QueryRow(ctx, `SELECT judge_connection_id, judge_model FROM scenarios WHERE id = $1`, *j.scenarioID).Scan(&cur, &curModel); err == nil && cur != nil && curModel != "" {
			connID, model = cur, curModel
		}
	}
	if connID == nil || model == "" {
		w.finishJudgement(runID, JudgeSkipped, "The scenario has no judge connection and model.", model, nil, nil)
		return
	}

	stats, rows, wait := w.traces(ctx, runID, j)
	if wait {
		return
	}

	creds, err := w.connections.Credentials(ctx, j.workspaceID, *connID)
	if err != nil {
		w.finishJudgement(runID, JudgeFailed, unavailable("judge connection", err).Error(), model, stats, nil)
		return
	}
	client, err := llm.New(creds, w.judging.AllowPrivate)
	if err != nil {
		w.finishJudgement(runID, JudgeFailed, err.Error(), model, stats, nil)
		return
	}
	in := judgeInput(j, rows, stats)
	result, err := judge.Run(ctx, client, model, in, w.judging.Samples)
	if err != nil {
		if ctx.Err() != nil {
			return // shutting down: the reaper requeues it
		}
		w.finishJudgement(runID, JudgeFailed, err.Error(), model, stats, nil)
		return
	}
	w.finishJudgement(runID, JudgeCompleted, "", model, stats, &result)
}

// traces fetches the run's Langfuse session; wait reports that the judgement
// was put back to wait for more observations.
func (w *Worker) traces(ctx context.Context, runID string, j judgeJob) (*TraceStats, []string, bool) {
	a := j.snap.Agent
	if a.TraceConnectionID == nil || j.traceID == nil || *j.traceID == "" {
		return &TraceStats{Status: "not_configured", Message: "The agent has no Langfuse connection.", Stats: langfuse.Stats{Models: []string{}}}, nil, false
	}
	session := *j.traceID
	stats := &TraceStats{SessionID: &session, Stats: langfuse.Stats{Models: []string{}}}
	creds, err := w.connections.Credentials(ctx, j.workspaceID, *a.TraceConnectionID)
	if err != nil {
		stats.Status, stats.Message = "error", unavailable("Langfuse connection", err).Error()
		return stats, nil, false
	}
	client, err := langfuse.New(creds, w.judging.AllowPrivate)
	if err != nil {
		stats.Status, stats.Message = "error", err.Error()
		return stats, nil, false
	}
	from, to := time.Now().Add(-time.Hour), time.Now().Add(time.Minute)
	if j.started != nil {
		from = j.started.Add(-time.Minute)
	}
	if j.finished != nil {
		to = j.finished.Add(30 * time.Minute)
	}
	obs, err := client.SessionObservations(ctx, session, from, to)
	since := time.Now()
	if j.traceSince != nil {
		since = *j.traceSince
	}
	inWindow := time.Since(since) < w.judging.TraceWait
	count := len(obs)
	stable := err == nil && count > 0 && j.traceCount != nil && *j.traceCount == count
	if !stable && inWindow && ctx.Err() == nil {
		// Langfuse ingests asynchronously: poll until the count stops changing.
		_, _ = w.pool.Exec(context.WithoutCancel(ctx),
			`UPDATE judgements SET status = 'waiting_traces', trace_count = $3, trace_since = $4,
			   next_attempt_at = now() + make_interval(secs => $5), lease_owner = NULL, lease_until = NULL, updated_at = now()
			 WHERE run_id = $1 AND lease_owner = $2`, runID, w.owner, count, since, w.judging.TracePoll.Seconds())
		return nil, nil, true
	}
	if err != nil {
		stats.Status, stats.Message = "error", err.Error()
		return stats, nil, false
	}
	if url, err := client.SessionURL(ctx, session); err == nil {
		stats.URL = &url
	}
	st, rows := langfuse.Digest(obs)
	stats.Stats = st
	if count == 0 {
		stats.Status, stats.Message = "unavailable", fmt.Sprintf("Langfuse has no observations for session %s yet.", session)
		return stats, nil, false
	}
	stats.Status, stats.Message = "available", fmt.Sprintf("%d observations in %d traces.", st.Observations, st.Traces)
	return stats, rows, false
}

func judgeInput(j judgeJob, traceRows []string, stats *TraceStats) judge.Input {
	sc, p := j.snap.Scenario, j.snap.Persona
	in := judge.Input{
		Modality: j.snap.Agent.Modality, AgentDescription: j.snap.Agent.Description,
		Goal: sc.Goal, Instructions: sc.Instructions, Context: sc.Context,
		PersonaName: p.Name, Locale: p.Locale, Temperament: p.Temperament, PersonaNotes: p.Notes,
		TraceRows: traceRows,
	}
	if stats != nil {
		in.TraceStatus = stats.Message
	}
	for _, f := range sc.Facts {
		in.Facts = append(in.Facts, judge.Fact{Key: f.Key, Value: f.Value})
	}
	for _, c := range sc.Criteria {
		id := ""
		if c.ID != nil {
			id = *c.ID
		}
		in.Criteria = append(in.Criteria, judge.Criterion{ID: id, Text: c.Text, Kind: c.Kind, Source: c.Source, Weight: c.Weight})
	}
	for _, t := range j.transcript {
		in.Transcript = append(in.Transcript, judge.Turn{Role: t.Role, Text: t.Text, StartMs: t.StartMs, EndMs: t.EndMs, LatencyMs: t.LatencyMs, Interrupted: t.Interrupted})
	}
	return in
}

func (w *Worker) finishJudgement(runID, status, msg, model string, traces *TraceStats, result *judge.Result) {
	var resultJSON, tracesJSON []byte
	var score *int
	var passed *bool
	if result != nil {
		resultJSON, _ = json.Marshal(result)
		score, passed = result.Score, result.Passed
	}
	if traces != nil {
		tracesJSON, _ = json.Marshal(traces)
	}
	_, err := w.pool.Exec(context.Background(),
		`UPDATE judgements SET status = $3, error = nullif($4, ''), model = $5, result = $6, traces = $7, score = $8, passed = $9,
		   completed_at = now(), updated_at = now(), lease_owner = NULL, lease_until = NULL
		 WHERE run_id = $1 AND lease_owner = $2`,
		runID, w.owner, status, msg, model, resultJSON, tracesJSON, score, passed)
	if err != nil {
		slog.Error("finish judgement", "run", runID, "err", err)
	}
}
