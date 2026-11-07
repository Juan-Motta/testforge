-- +goose Up
ALTER TABLE personas ADD COLUMN voice_model text;

CREATE TABLE run_batches (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    label        text NOT NULL DEFAULT '',
    concurrency  integer NOT NULL CHECK (concurrency BETWEEN 1 AND 10),
    created_by   uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX run_batches_workspace_idx ON run_batches (workspace_id, created_at DESC);

CREATE TABLE runs (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id     uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    batch_id         uuid NOT NULL REFERENCES run_batches (id) ON DELETE CASCADE,
    -- Runs outlive their scenario and agent: the snapshot keeps what ran.
    scenario_id      uuid REFERENCES scenarios (id) ON DELETE SET NULL,
    agent_id         uuid REFERENCES agents (id) ON DELETE SET NULL,
    attempt          integer NOT NULL,
    status           text NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed', 'cancelled', 'interrupted')),
    end_reason       text,
    error            text,
    snapshot         jsonb NOT NULL,
    thread_id        uuid,
    trace_session_id text,
    transcript       jsonb NOT NULL DEFAULT '[]'::jsonb,
    metrics          jsonb,
    audio_tracks     text[] NOT NULL DEFAULT '{}',
    cancel_requested boolean NOT NULL DEFAULT false,
    lease_owner      text,
    lease_until      timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    started_at       timestamptz,
    finished_at      timestamptz
);
CREATE INDEX runs_queue_idx ON runs (created_at, id) WHERE status = 'queued';
CREATE INDEX runs_running_idx ON runs (batch_id) WHERE status = 'running';
CREATE INDEX runs_workspace_idx ON runs (workspace_id, created_at DESC, id DESC);

CREATE TABLE run_events (
    seq        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id     uuid NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
    kind       text NOT NULL,
    data       jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX run_events_run_idx ON run_events (run_id, seq);

-- +goose Down
DROP TABLE run_events;
DROP TABLE runs;
DROP TABLE run_batches;
ALTER TABLE personas DROP COLUMN voice_model;
