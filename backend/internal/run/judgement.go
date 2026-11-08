package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"testforge/internal/judge"
	"testforge/internal/langfuse"
	"testforge/internal/platform/apperr"
	"testforge/internal/platform/database"
	"testforge/internal/workspace"
)

// Judgement statuses.
const (
	JudgeQueued        = "queued"
	JudgeWaitingTraces = "waiting_traces"
	JudgeRunning       = "running"
	JudgeCompleted     = "completed"
	JudgeFailed        = "failed"
	JudgeSkipped       = "skipped"
)

// TraceStats is what the API shows about a run's traces.
type TraceStats struct {
	Status    string  `json:"status"`
	Message   string  `json:"message"`
	SessionID *string `json:"sessionId"`
	URL       *string `json:"url"`
	langfuse.Stats
}

// Judgement is a run's evaluation.
type Judgement struct {
	RunID       string        `json:"runId"`
	Status      string        `json:"status"`
	Attempt     int           `json:"attempt"`
	Model       string        `json:"model"`
	Error       *string       `json:"error"`
	Result      *judge.Result `json:"result"`
	Traces      *TraceStats   `json:"traces"`
	CreatedAt   time.Time     `json:"createdAt"`
	UpdatedAt   time.Time     `json:"updatedAt"`
	CompletedAt *time.Time    `json:"completedAt"`
}

// queueJudgement queues (or skips) the judgement of a completed run.
func queueJudgement(ctx context.Context, db database.DBTX, runID, workspaceID string, snap Snapshot) error {
	status, errMsg := JudgeQueued, ""
	if snap.Scenario.Judge.ConnectionID == nil || snap.Scenario.Judge.Model == "" {
		status, errMsg = JudgeSkipped, "The scenario has no judge connection and model."
	}
	_, err := db.Exec(ctx,
		`INSERT INTO judgements (run_id, workspace_id, status, model, error) VALUES ($1, $2, $3, $4, nullif($5, ''))
		 ON CONFLICT (run_id) DO NOTHING`, runID, workspaceID, status, snap.Scenario.Judge.Model, errMsg)
	return err
}

// GetJudgement returns a run's judgement.
func (s *Service) GetJudgement(ctx context.Context, ws workspace.Workspace, runID string) (Judgement, error) {
	if !uuidPattern.MatchString(runID) {
		return Judgement{}, errRunMissing
	}
	var j Judgement
	var result, traces []byte
	err := s.pool.QueryRow(ctx,
		`SELECT run_id, status, attempt, model, error, result, traces, created_at, updated_at, completed_at
		 FROM judgements WHERE workspace_id = $1 AND run_id = $2`, ws.ID, runID,
	).Scan(&j.RunID, &j.Status, &j.Attempt, &j.Model, &j.Error, &result, &traces, &j.CreatedAt, &j.UpdatedAt, &j.CompletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err := s.summary(ctx, ws, runID); err != nil {
			return Judgement{}, err
		}
		return Judgement{}, apperr.NotFound("This run has not been judged: only completed runs are.")
	}
	if err != nil {
		return Judgement{}, err
	}
	if result != nil {
		j.Result = &judge.Result{}
		if err := json.Unmarshal(result, j.Result); err != nil {
			return Judgement{}, fmt.Errorf("decode judgement: %w", err)
		}
	}
	if traces != nil {
		j.Traces = &TraceStats{}
		if err := json.Unmarshal(traces, j.Traces); err != nil {
			return Judgement{}, fmt.Errorf("decode traces: %w", err)
		}
	}
	return j, nil
}

// Rejudge queues a new judgement of a completed run. Member or above.
func (s *Service) Rejudge(ctx context.Context, ws workspace.Workspace, runID string) (Judgement, error) {
	if err := canOperate(ws); err != nil {
		return Judgement{}, err
	}
	if !uuidPattern.MatchString(runID) {
		return Judgement{}, errRunMissing
	}
	var status string
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT status, snapshot FROM runs WHERE workspace_id = $1 AND id = $2`, ws.ID, runID).Scan(&status, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Judgement{}, errRunMissing
	}
	if err != nil {
		return Judgement{}, err
	}
	if status != StatusCompleted {
		return Judgement{}, apperr.Conflict("Only completed runs can be judged.")
	}
	var snap Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return Judgement{}, err
	}
	err = database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var current string
		err := tx.QueryRow(ctx, `SELECT status FROM judgements WHERE run_id = $1 FOR UPDATE`, runID).Scan(&current)
		if errors.Is(err, pgx.ErrNoRows) {
			// No judgement yet (the run predates judging): queue it and let the
			// worker apply the scenario's current judge settings.
			_, err := tx.Exec(ctx, `INSERT INTO judgements (run_id, workspace_id, status, model) VALUES ($1, $2, 'queued', $3)
				 ON CONFLICT (run_id) DO NOTHING`, runID, ws.ID, snap.Scenario.Judge.Model)
			return err
		}
		if err != nil {
			return err
		}
		if current == JudgeQueued || current == JudgeWaitingTraces || current == JudgeRunning {
			return apperr.Conflict("This run is being judged already.")
		}
		_, err = tx.Exec(ctx,
			`UPDATE judgements SET status = 'queued', attempt = attempt + 1, error = NULL, result = NULL, traces = NULL,
			   score = NULL, passed = NULL, trace_count = NULL, trace_since = NULL, next_attempt_at = now(),
			   completed_at = NULL, updated_at = now()
			 WHERE run_id = $1`, runID)
		return err
	})
	if err != nil {
		return Judgement{}, err
	}
	return s.GetJudgement(ctx, ws, runID)
}
