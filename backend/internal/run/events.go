package run

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"testforge/internal/platform/database"
)

// Recorder publishes a run's live events. Engines call it from their goroutines.
type Recorder interface {
	// Transcript upserts turn index; final marks it complete.
	Transcript(index int, turn Turn, final bool)
	Metrics(m Metrics)
	Log(level, message string)
}

func recordEvent(ctx context.Context, db database.DBTX, runID, kind string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := db.Exec(ctx, `INSERT INTO run_events (run_id, kind, data) VALUES ($1, $2, $3)`, runID, kind, raw); err != nil {
		return fmt.Errorf("record %s event: %w", kind, err)
	}
	return nil
}

func statusData(status, endReason, errMsg string) map[string]any {
	d := map[string]any{"status": status, "endReason": nil, "error": nil}
	if endReason != "" {
		d["endReason"] = endReason
	}
	if errMsg != "" {
		d["error"] = errMsg
	}
	return d
}

// dbRecorder writes events, throttling non-final transcript updates per turn
// so streaming transcripts do not flood the table.
type dbRecorder struct {
	db    database.DBTX
	runID string
	mu    sync.Mutex
	last  map[int]time.Time
}

const transcriptThrottle = 300 * time.Millisecond

func newRecorder(db database.DBTX, runID string) *dbRecorder {
	return &dbRecorder{db: db, runID: runID, last: map[int]time.Time{}}
}

func (r *dbRecorder) write(kind string, data any) {
	// Live events are best effort: the final transcript is stored with the run.
	_ = recordEvent(context.Background(), r.db, r.runID, kind, data)
}

func (r *dbRecorder) Transcript(index int, turn Turn, final bool) {
	r.mu.Lock()
	if !final && time.Since(r.last[index]) < transcriptThrottle {
		r.mu.Unlock()
		return
	}
	r.last[index] = time.Now()
	r.mu.Unlock()
	r.write("transcript", map[string]any{"index": index, "turn": turn, "final": final})
}

func (r *dbRecorder) Metrics(m Metrics) { r.write("metrics", m) }

func (r *dbRecorder) Log(level, message string) {
	r.write("log", map[string]string{"level": level, "message": message})
}
