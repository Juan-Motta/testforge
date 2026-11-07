package run

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"testforge/internal/agent"
	"testforge/internal/authn"
	"testforge/internal/platform/apperr"
	"testforge/internal/platform/database"
	"testforge/internal/scenario"
	"testforge/internal/workspace"
)

var (
	uuidPattern     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	errRunMissing   = apperr.NotFound("Run not found.")
	errBatchMissing = apperr.NotFound("Batch not found.")
)

// Service implements launching and reading runs.
type Service struct {
	pool      *pgxpool.Pool
	agents    *agent.Service
	scenarios *scenario.Service
	storage   Storage
}

// NewService builds the service.
func NewService(pool *pgxpool.Pool, agents *agent.Service, scenarios *scenario.Service, storage Storage) *Service {
	return &Service{pool: pool, agents: agents, scenarios: scenarios, storage: storage}
}

func canOperate(ws workspace.Workspace) error {
	if ws.Role == workspace.RoleViewer {
		return apperr.Forbidden("Viewers cannot launch or change runs.")
	}
	return nil
}

// Launch validates the scenarios, snapshots them and queues one run per
// scenario × repetition. Member or above.
func (s *Service) Launch(ctx context.Context, p authn.Principal, ws workspace.Workspace, in LaunchInput) (Batch, error) {
	if err := canOperate(ws); err != nil {
		return Batch{}, err
	}
	fields := map[string]string{}
	reps, conc := 1, 3
	if in.Repetitions != nil {
		reps = *in.Repetitions
	}
	if in.Concurrency != nil {
		conc = *in.Concurrency
	}
	if reps < 1 || reps > 10 {
		fields["repetitions"] = "Use 1 to 10."
	}
	if conc < 1 || conc > 10 {
		fields["concurrency"] = "Use 1 to 10."
	}
	label := strings.TrimSpace(in.Label)
	if len([]rune(label)) > 100 {
		fields["label"] = "Use at most 100 characters."
	}
	switch n := len(in.ScenarioIDs); {
	case n == 0:
		fields["scenarioIds"] = "Choose at least one scenario."
	case n > 100:
		fields["scenarioIds"] = "Choose at most 100 scenarios."
	}

	snapshots := make([]Snapshot, 0, len(in.ScenarioIDs))
	seen := map[string]bool{}
	for i, id := range in.ScenarioIDs {
		f := fmt.Sprintf("scenarioIds.%d", i)
		if seen[id] {
			fields[f] = "Listed twice."
			continue
		}
		seen[id] = true
		snap, problem, err := s.snapshot(ctx, ws, id)
		if err != nil {
			return Batch{}, err
		}
		if problem != "" {
			fields[f] = problem
			continue
		}
		snapshots = append(snapshots, snap)
	}
	if err := apperr.Invalid(fields); err != nil {
		return Batch{}, err
	}

	var batchID string
	err := database.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx,
			`INSERT INTO run_batches (workspace_id, label, concurrency, created_by) VALUES ($1, $2, $3, $4) RETURNING id`,
			ws.ID, label, conc, p.UserID).Scan(&batchID); err != nil {
			return fmt.Errorf("insert batch: %w", err)
		}
		for _, snap := range snapshots {
			raw, _ := json.Marshal(snap)
			for r := 1; r <= reps; r++ {
				if _, err := tx.Exec(ctx,
					`INSERT INTO runs (workspace_id, batch_id, scenario_id, agent_id, attempt, status, snapshot)
					 VALUES ($1, $2, $3, $4, $5, 'queued', $6)`,
					ws.ID, batchID, snap.Scenario.ID, snap.Agent.ID, r, raw); err != nil {
					return fmt.Errorf("insert run: %w", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return Batch{}, err
	}
	return s.batch(ctx, ws, batchID)
}

// snapshot loads what a scenario needs to run, or explains why it cannot run.
func (s *Service) snapshot(ctx context.Context, ws workspace.Workspace, id string) (Snapshot, string, error) {
	sc, err := s.scenarios.GetScenario(ctx, ws, id)
	if isNotFound(err) {
		return Snapshot{}, "Scenario not found in this workspace.", nil
	}
	if err != nil {
		return Snapshot{}, "", err
	}
	a, err := s.agents.Get(ctx, ws, sc.AgentID)
	if err != nil {
		return Snapshot{}, "", err
	}
	persona, err := s.scenarios.GetPersona(ctx, ws, sc.PersonaID)
	if err != nil {
		return Snapshot{}, "", err
	}
	name := sc.Name + ": "
	switch {
	case sc.Simulator.Mode == "realtime":
		return Snapshot{}, name + "the realtime simulator is not supported yet; use pipeline.", nil
	case sc.Simulator.ConnectionID == nil || sc.Simulator.Model == "":
		return Snapshot{}, name + "choose a simulator connection and model.", nil
	case a.Modality == agent.ModalityVoice && persona.Voice == nil:
		return Snapshot{}, name + "give the persona " + persona.Name + " a voice.", nil
	}
	return Snapshot{Agent: a, Scenario: sc, Persona: persona}, "", nil
}

func isNotFound(err error) bool {
	e, ok := apperr.As(err)
	return ok && e.Kind == apperr.KindNotFound
}

const selectSummary = `SELECT r.id, r.batch_id, r.scenario_id, r.snapshot->'scenario'->>'name', r.agent_id,
	r.snapshot->'agent'->>'name', r.snapshot->'persona'->>'name', r.snapshot->'agent'->>'modality',
	r.status, r.end_reason, r.error, r.attempt, r.created_at, r.started_at, r.finished_at, r.metrics`

func scanSummary(row interface{ Scan(...any) error }, extra ...any) (Summary, error) {
	var sm Summary
	var metrics []byte
	dest := append([]any{&sm.ID, &sm.BatchID, &sm.ScenarioID, &sm.ScenarioName, &sm.AgentID, &sm.AgentName, &sm.PersonaName,
		&sm.Modality, &sm.Status, &sm.EndReason, &sm.Error, &sm.Attempt, &sm.CreatedAt, &sm.StartedAt, &sm.FinishedAt, &metrics}, extra...)
	if err := row.Scan(dest...); err != nil {
		return Summary{}, err
	}
	if metrics != nil {
		sm.Metrics = &Metrics{}
		if err := json.Unmarshal(metrics, sm.Metrics); err != nil {
			return Summary{}, fmt.Errorf("decode metrics: %w", err)
		}
	}
	return sm, nil
}

// ListFilter narrows a run list.
type ListFilter struct {
	BatchID, ScenarioID, AgentID, Status string
	Limit                                int
	Before                               string
}

// ListRuns returns runs newest first, with a cursor for the next page.
func (s *Service) ListRuns(ctx context.Context, ws workspace.Workspace, f ListFilter) ([]Summary, *string, error) {
	where := []string{"r.workspace_id = $1"}
	args := []any{ws.ID}
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	fields := map[string]string{}
	for _, c := range []struct{ name, val, col string }{{"batchId", f.BatchID, "batch_id"}, {"scenarioId", f.ScenarioID, "scenario_id"}, {"agentId", f.AgentID, "agent_id"}} {
		if c.val == "" {
			continue
		}
		if !uuidPattern.MatchString(c.val) {
			fields[c.name] = "Use an id."
			continue
		}
		where = append(where, "r."+c.col+" = "+arg(c.val))
	}
	if f.Status != "" {
		switch f.Status {
		case StatusQueued, StatusRunning, StatusCompleted, StatusFailed, StatusCancelled, StatusInterrupted:
			where = append(where, "r.status = "+arg(f.Status))
		default:
			fields["status"] = "Unknown status."
		}
	}
	if f.Limit == 0 {
		f.Limit = 50
	}
	if f.Limit < 1 || f.Limit > 200 {
		fields["limit"] = "Use 1 to 200."
	}
	if f.Before != "" {
		at, id, ok := decodeCursor(f.Before)
		if !ok {
			fields["before"] = "Invalid cursor."
		} else {
			where = append(where, "(r.created_at, r.id) < ("+arg(at)+", "+arg(id)+")")
		}
	}
	if err := apperr.Invalid(fields); err != nil {
		return nil, nil, err
	}
	rows, err := s.pool.Query(ctx, selectSummary+` FROM runs r WHERE `+strings.Join(where, " AND ")+
		` ORDER BY r.created_at DESC, r.id DESC LIMIT `+arg(f.Limit+1), args...)
	if err != nil {
		return nil, nil, fmt.Errorf("list runs: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Summary, error) { return scanSummary(r) })
	if err != nil {
		return nil, nil, err
	}
	var next *string
	if len(items) > f.Limit {
		items = items[:f.Limit]
		last := items[len(items)-1]
		c := encodeCursor(last.CreatedAt, last.ID)
		next = &c
	}
	return items, next, nil
}

func encodeCursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + id))
}

func decodeCursor(c string) (time.Time, string, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return time.Time{}, "", false
	}
	ts, id, ok := strings.Cut(string(raw), "|")
	at, err := time.Parse(time.RFC3339Nano, ts)
	if !ok || err != nil || !uuidPattern.MatchString(id) {
		return time.Time{}, "", false
	}
	return at, id, true
}

// GetRun returns a run with its transcript.
func (s *Service) GetRun(ctx context.Context, ws workspace.Workspace, id string) (Run, error) {
	if !uuidPattern.MatchString(id) {
		return Run{}, errRunMissing
	}
	var r Run
	var transcript []byte
	sm, err := scanSummary(s.pool.QueryRow(ctx, selectSummary+`, r.thread_id, r.trace_session_id, r.transcript, r.audio_tracks
		FROM runs r WHERE r.workspace_id = $1 AND r.id = $2`, ws.ID, id),
		&r.ThreadID, &r.TraceSessionID, &transcript, &r.AudioTracks)
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, errRunMissing
	}
	if err != nil {
		return Run{}, err
	}
	r.Summary = sm
	if err := json.Unmarshal(transcript, &r.Transcript); err != nil {
		return Run{}, fmt.Errorf("decode transcript: %w", err)
	}
	if r.AudioTracks == nil {
		r.AudioTracks = []string{}
	}
	return r, nil
}

func (s *Service) summary(ctx context.Context, ws workspace.Workspace, id string) (Summary, error) {
	sm, err := scanSummary(s.pool.QueryRow(ctx, selectSummary+` FROM runs r WHERE r.workspace_id = $1 AND r.id = $2`, ws.ID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Summary{}, errRunMissing
	}
	return sm, err
}

// Cancel stops a queued or running run. Member or above.
func (s *Service) Cancel(ctx context.Context, ws workspace.Workspace, id string) (Summary, error) {
	if err := canOperate(ws); err != nil {
		return Summary{}, err
	}
	if !uuidPattern.MatchString(id) {
		return Summary{}, errRunMissing
	}
	if _, err := s.summary(ctx, ws, id); err != nil {
		return Summary{}, err
	}
	if err := s.cancelWhere(ctx, "workspace_id = $1 AND id = $2", ws.ID, id); err != nil {
		return Summary{}, err
	}
	return s.summary(ctx, ws, id)
}

// cancelWhere cancels queued runs at once and flags running ones for their worker.
func (s *Service) cancelWhere(ctx context.Context, cond string, args ...any) error {
	rows, err := s.pool.Query(ctx,
		`UPDATE runs SET status = 'cancelled', end_reason = 'cancelled', finished_at = now()
		 WHERE `+cond+` AND status = 'queued' RETURNING id`, args...)
	if err != nil {
		return fmt.Errorf("cancel queued runs: %w", err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := recordEvent(ctx, s.pool, id, "status", statusData(StatusCancelled, EndCancelled, "")); err != nil {
			return err
		}
		if err := recordEvent(ctx, s.pool, id, "end", map[string]string{"status": StatusCancelled}); err != nil {
			return err
		}
	}
	_, err = s.pool.Exec(ctx, `UPDATE runs SET cancel_requested = true WHERE `+cond+` AND status = 'running'`, args...)
	return err
}

// Rerun queues a new run from the same snapshot. Member or above.
func (s *Service) Rerun(ctx context.Context, ws workspace.Workspace, id string) (Summary, error) {
	if err := canOperate(ws); err != nil {
		return Summary{}, err
	}
	if !uuidPattern.MatchString(id) {
		return Summary{}, errRunMissing
	}
	var newID string
	err := s.pool.QueryRow(ctx,
		`INSERT INTO runs (workspace_id, batch_id, scenario_id, agent_id, attempt, status, snapshot)
		 SELECT workspace_id, batch_id, scenario_id, agent_id,
		   (SELECT coalesce(max(attempt), 0) + 1 FROM runs x WHERE x.batch_id = r.batch_id
		      AND x.snapshot->'scenario'->>'id' = r.snapshot->'scenario'->>'id'),
		   'queued', snapshot
		 FROM runs r WHERE r.workspace_id = $1 AND r.id = $2 RETURNING id`, ws.ID, id).Scan(&newID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Summary{}, errRunMissing
	}
	if err != nil {
		return Summary{}, fmt.Errorf("rerun: %w", err)
	}
	return s.summary(ctx, ws, newID)
}

const selectBatches = `SELECT b.id, b.label, b.concurrency, b.created_at, u.id, u.name,
	coalesce((SELECT jsonb_object_agg(status, n) FROM (SELECT status, count(*) AS n FROM runs WHERE batch_id = b.id GROUP BY status) c), '{}')
	FROM run_batches b LEFT JOIN users u ON u.id = b.created_by`

func scanBatch(row interface{ Scan(...any) error }) (Batch, error) {
	var b Batch
	var uid, uname *string
	var counts []byte
	if err := row.Scan(&b.ID, &b.Label, &b.Concurrency, &b.CreatedAt, &uid, &uname, &counts); err != nil {
		return Batch{}, err
	}
	if err := json.Unmarshal(counts, &b.Counts); err != nil {
		return Batch{}, err
	}
	for _, n := range b.Counts {
		b.Total += n
	}
	if uid != nil && uname != nil {
		b.CreatedBy = &Creator{ID: *uid, Name: *uname}
	}
	return b, nil
}

func (s *Service) batch(ctx context.Context, ws workspace.Workspace, id string) (Batch, error) {
	if !uuidPattern.MatchString(id) {
		return Batch{}, errBatchMissing
	}
	b, err := scanBatch(s.pool.QueryRow(ctx, selectBatches+` WHERE b.workspace_id = $1 AND b.id = $2`, ws.ID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Batch{}, errBatchMissing
	}
	return b, err
}

// ListBatches returns recent batches with progress.
func (s *Service) ListBatches(ctx context.Context, ws workspace.Workspace, limit int) ([]Batch, error) {
	if limit == 0 {
		limit = 20
	}
	if limit < 1 || limit > 100 {
		return nil, apperr.Invalid(map[string]string{"limit": "Use 1 to 100."})
	}
	rows, err := s.pool.Query(ctx, selectBatches+` WHERE b.workspace_id = $1 ORDER BY b.created_at DESC, b.id DESC LIMIT $2`, ws.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("list batches: %w", err)
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Batch, error) { return scanBatch(r) })
}

// CancelBatch cancels every unfinished run of a batch. Member or above.
func (s *Service) CancelBatch(ctx context.Context, ws workspace.Workspace, id string) (Batch, error) {
	if err := canOperate(ws); err != nil {
		return Batch{}, err
	}
	if _, err := s.batch(ctx, ws, id); err != nil {
		return Batch{}, err
	}
	if err := s.cancelWhere(ctx, "workspace_id = $1 AND batch_id = $2", ws.ID, id); err != nil {
		return Batch{}, err
	}
	return s.batch(ctx, ws, id)
}

// EventsAfter returns up to 200 events of a run after seq, and whether the run has finished.
func (s *Service) EventsAfter(ctx context.Context, ws workspace.Workspace, id string, after int64) ([]Event, bool, error) {
	if !uuidPattern.MatchString(id) {
		return nil, false, errRunMissing
	}
	var status string
	err := s.pool.QueryRow(ctx, `SELECT status FROM runs WHERE workspace_id = $1 AND id = $2`, ws.ID, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, errRunMissing
	}
	if err != nil {
		return nil, false, err
	}
	rows, err := s.pool.Query(ctx, `SELECT seq, kind, data FROM run_events WHERE run_id = $1 AND seq > $2 ORDER BY seq LIMIT 200`, id, after)
	if err != nil {
		return nil, false, fmt.Errorf("list events: %w", err)
	}
	events, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Event, error) {
		var e Event
		err := r.Scan(&e.Seq, &e.Kind, &e.Data)
		return e, err
	})
	finished := status != StatusQueued && status != StatusRunning
	return events, finished, err
}

// Audio opens a run's audio track.
func (s *Service) Audio(ctx context.Context, ws workspace.Workspace, id, track string) ([]byte, error) {
	r, err := s.GetRun(ctx, ws, id)
	if err != nil {
		return nil, err
	}
	for _, t := range r.AudioTracks {
		if t == track {
			return s.storage.Get(ctx, audioKey(ws.ID, id, track))
		}
	}
	return nil, apperr.NotFound("This run has no " + track + " audio.")
}

func audioKey(wsID, runID, track string) string {
	return wsID + "/" + runID + "/" + track + ".wav"
}
