package run

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"testforge/internal/agent"
	"testforge/internal/connection"
	"testforge/internal/platform/database"
)

const (
	leaseDuration  = 30 * time.Second
	heartbeatEvery = 3 * time.Second
	pollEvery      = time.Second
)

// Job is everything an engine needs to run one conversation.
type Job struct {
	RunID          string
	WorkspaceID    string
	ThreadID       string
	Snapshot       Snapshot
	AgentSecrets   map[string]string
	Simulator      connection.Credentials
	SimulatorModel string
	// Voice is set for voice agents: the persona's TTS connection and voice.
	Voice *VoiceCredentials
}

// VoiceCredentials are the persona voice and its TTS connection.
type VoiceCredentials struct {
	connection.Credentials
	Voice string
	Model string
}

// Outcome is what an engine reports when the conversation ends.
type Outcome struct {
	Status         string
	EndReason      string
	Error          string
	Transcript     []Turn
	Metrics        Metrics
	TraceSessionID string
	// Audio maps a track (user, agent, conversation) to WAV bytes.
	Audio map[string][]byte
}

// Engine runs one conversation. It must return when ctx is cancelled.
type Engine interface {
	Run(ctx context.Context, job Job, rec Recorder) Outcome
}

// Engines picks an engine per modality.
type Engines func(modality string) Engine

// Worker claims queued runs and executes them.
type Worker struct {
	pool        *pgxpool.Pool
	connections *connection.Service
	agents      *agent.Service
	engines     Engines
	storage     Storage
	concurrency int
	owner       string
	slots       chan struct{}
	wg          sync.WaitGroup
}

// NewWorker builds a worker running at most concurrency runs at a time.
func NewWorker(pool *pgxpool.Pool, connections *connection.Service, agents *agent.Service, engines Engines, storage Storage, concurrency int) *Worker {
	host, _ := os.Hostname()
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return &Worker{
		pool: pool, connections: connections, agents: agents, engines: engines, storage: storage,
		concurrency: concurrency, owner: host + "-" + hex.EncodeToString(b), slots: make(chan struct{}, concurrency),
	}
}

// Run claims and executes runs until ctx is cancelled, then waits for active runs to stop.
func (w *Worker) Run(ctx context.Context) {
	slog.Info("worker started", "owner", w.owner, "concurrency", w.concurrency)
	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()
	for {
		w.reap(ctx)
		w.fill(ctx)
		select {
		case <-ctx.Done():
			w.wg.Wait()
			return
		case <-ticker.C:
		}
	}
}

// fill claims runs while there are free slots.
func (w *Worker) fill(ctx context.Context) {
	for {
		select {
		case w.slots <- struct{}{}:
		default:
			return
		}
		claimed, err := w.claim(ctx)
		if err != nil || claimed == nil {
			<-w.slots
			if err != nil && ctx.Err() == nil {
				slog.Error("claim run", "err", err)
			}
			return
		}
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			defer func() { <-w.slots }()
			w.execute(ctx, claimed)
		}()
	}
}

type claimedRun struct {
	id, workspaceID, threadID string
	snapshot                  Snapshot
}

// claim takes the oldest queued run whose batch is below its concurrency.
// Locking the batch row serializes claims within a batch, so concurrent
// workers never exceed it.
func (w *Worker) claim(ctx context.Context) (*claimedRun, error) {
	var out *claimedRun
	err := database.WithTx(ctx, w.pool, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx,
			`SELECT r.id, r.batch_id FROM runs r WHERE r.status = 'queued' AND NOT r.cancel_requested
			 ORDER BY r.created_at, r.id FOR UPDATE SKIP LOCKED LIMIT 20`)
		if err != nil {
			return err
		}
		type cand struct{ id, batch string }
		cands, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (cand, error) {
			var c cand
			return c, r.Scan(&c.id, &c.batch)
		})
		if err != nil {
			return err
		}
		full := map[string]bool{}
		for _, c := range cands {
			if full[c.batch] {
				continue
			}
			var limit, running int
			// SKIP LOCKED: a batch another worker is claiming from is skipped this
			// round instead of waiting, which also rules out lock-order deadlocks.
			err := tx.QueryRow(ctx, `SELECT concurrency FROM run_batches WHERE id = $1 FOR UPDATE SKIP LOCKED`, c.batch).Scan(&limit)
			if errors.Is(err, pgx.ErrNoRows) {
				full[c.batch] = true
				continue
			}
			if err != nil {
				return err
			}
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM runs WHERE batch_id = $1 AND status = 'running'`, c.batch).Scan(&running); err != nil {
				return err
			}
			if running >= limit {
				full[c.batch] = true
				continue
			}
			threadID := newUUID()
			var raw []byte
			out = &claimedRun{id: c.id, threadID: threadID}
			if err := tx.QueryRow(ctx,
				`UPDATE runs SET status = 'running', lease_owner = $2, lease_until = now() + make_interval(secs => $3),
				   started_at = now(), thread_id = $4
				 WHERE id = $1 RETURNING workspace_id, snapshot`,
				c.id, w.owner, leaseDuration.Seconds(), threadID).Scan(&out.workspaceID, &raw); err != nil {
				return err
			}
			if err := json.Unmarshal(raw, &out.snapshot); err != nil {
				return fmt.Errorf("decode snapshot: %w", err)
			}
			return recordEvent(ctx, tx, c.id, "status", statusData(StatusRunning, "", ""))
		}
		return nil
	})
	return out, err
}

// execute runs one claimed run with a heartbeat that also delivers cancellation.
func (w *Worker) execute(parent context.Context, c *claimedRun) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	cancelled := make(chan struct{})
	lost := make(chan struct{})
	go w.heartbeat(ctx, c.id, cancel, cancelled, lost)

	rec := newRecorder(w.pool, c.id)
	outcome := func() Outcome {
		job, err := w.prepare(ctx, c)
		if err != nil {
			return Outcome{Status: StatusFailed, EndReason: EndError, Error: err.Error()}
		}
		engine := w.engines(c.snapshot.Agent.Modality)
		if engine == nil {
			return Outcome{Status: StatusFailed, EndReason: EndError, Error: "no engine for " + c.snapshot.Agent.Modality + " agents"}
		}
		return engine.Run(ctx, job, rec)
	}()

	select {
	case <-lost:
		slog.Warn("run lease lost; not finishing", "run", c.id)
		return
	default:
	}
	select {
	case <-cancelled:
		outcome.Status, outcome.EndReason, outcome.Error = StatusCancelled, EndCancelled, ""
	default:
	}
	if parent.Err() != nil && outcome.Status != StatusCancelled {
		// The worker is shutting down: leave the run to the reaper as interrupted.
		outcome.Status, outcome.EndReason, outcome.Error = StatusInterrupted, EndWorkerLost, "The worker stopped during the run."
	}
	w.finish(context.Background(), c, outcome)
}

func (w *Worker) heartbeat(ctx context.Context, runID string, cancel context.CancelFunc, cancelled, lost chan struct{}) {
	t := time.NewTicker(heartbeatEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		var requested bool
		err := w.pool.QueryRow(ctx,
			`UPDATE runs SET lease_until = now() + make_interval(secs => $3)
			 WHERE id = $1 AND lease_owner = $2 AND status = 'running' RETURNING cancel_requested`,
			runID, w.owner, leaseDuration.Seconds()).Scan(&requested)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			close(lost)
			cancel()
			return
		case err != nil:
			continue
		case requested:
			close(cancelled)
			cancel()
			return
		}
	}
}

func (w *Worker) prepare(ctx context.Context, c *claimedRun) (Job, error) {
	snap := c.snapshot
	job := Job{RunID: c.id, WorkspaceID: c.workspaceID, ThreadID: c.threadID, Snapshot: snap, SimulatorModel: snap.Scenario.Simulator.Model}
	secrets, err := w.agents.Secrets(ctx, c.workspaceID, snap.Agent.ID)
	if err != nil {
		return Job{}, fmt.Errorf("the agent is no longer available: %w", err)
	}
	job.AgentSecrets = secrets
	if snap.Scenario.Simulator.ConnectionID == nil {
		return Job{}, errors.New("the scenario has no simulator connection")
	}
	if job.Simulator, err = w.connections.Credentials(ctx, c.workspaceID, *snap.Scenario.Simulator.ConnectionID); err != nil {
		return Job{}, fmt.Errorf("the simulator connection is no longer available: %w", err)
	}
	if snap.Agent.Modality == agent.ModalityVoice {
		v := snap.Persona.Voice
		if v == nil {
			return Job{}, errors.New("the persona has no voice")
		}
		creds, err := w.connections.Credentials(ctx, c.workspaceID, v.ConnectionID)
		if err != nil {
			return Job{}, fmt.Errorf("the voice connection is no longer available: %w", err)
		}
		job.Voice = &VoiceCredentials{Credentials: creds, Voice: v.Voice}
		if v.Model != nil {
			job.Voice.Model = *v.Model
		}
	}
	return job, nil
}

func (w *Worker) finish(ctx context.Context, c *claimedRun, o Outcome) {
	var tracks []string
	for track, data := range o.Audio {
		if err := w.storage.Put(ctx, audioKey(c.workspaceID, c.id, track), data); err != nil {
			slog.Error("store audio", "run", c.id, "err", err)
			continue
		}
		tracks = append(tracks, track)
	}
	sort.Strings(tracks)
	if tracks == nil {
		tracks = []string{}
	}
	if o.Transcript == nil {
		o.Transcript = []Turn{}
	}
	transcript, _ := json.Marshal(o.Transcript)
	metrics, _ := json.Marshal(o.Metrics)
	var trace *string
	if o.TraceSessionID != "" {
		trace = &o.TraceSessionID
	}
	err := database.WithTx(ctx, w.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE runs SET status = $3, end_reason = $4, error = nullif($5, ''), transcript = $6, metrics = $7,
			   audio_tracks = $8, trace_session_id = $9, finished_at = now(), lease_owner = NULL, lease_until = NULL
			 WHERE id = $1 AND lease_owner = $2`,
			c.id, w.owner, o.Status, o.EndReason, o.Error, transcript, metrics, tracks, trace)
		if err != nil || tag.RowsAffected() == 0 {
			return err
		}
		if err := recordEvent(ctx, tx, c.id, "metrics", o.Metrics); err != nil {
			return err
		}
		if err := recordEvent(ctx, tx, c.id, "status", statusData(o.Status, o.EndReason, o.Error)); err != nil {
			return err
		}
		return recordEvent(ctx, tx, c.id, "end", map[string]string{"status": o.Status})
	})
	if err != nil {
		slog.Error("finish run", "run", c.id, "err", err)
	}
}

// reap marks runs whose worker vanished (expired lease) as interrupted.
func (w *Worker) reap(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		`UPDATE runs SET status = 'interrupted', end_reason = 'worker_lost', error = 'The worker running this conversation stopped.',
		   finished_at = now(), lease_owner = NULL, lease_until = NULL
		 WHERE status = 'running' AND lease_until < now() RETURNING id`)
	if err != nil {
		return
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return
	}
	for _, id := range ids {
		_ = recordEvent(ctx, w.pool, id, "status", statusData(StatusInterrupted, EndWorkerLost, "The worker running this conversation stopped."))
		_ = recordEvent(ctx, w.pool, id, "end", map[string]string{"status": StatusInterrupted})
	}
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
