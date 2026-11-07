-- +goose Up
CREATE TABLE judgements (
    run_id          uuid PRIMARY KEY REFERENCES runs (id) ON DELETE CASCADE,
    workspace_id    uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    status          text NOT NULL CHECK (status IN ('queued', 'waiting_traces', 'running', 'completed', 'failed', 'skipped')),
    attempt         integer NOT NULL DEFAULT 1,
    model           text NOT NULL DEFAULT '',
    error           text,
    result          jsonb,
    traces          jsonb,
    score           integer,
    passed          boolean,
    -- Trace polling state: observation count of the last poll and when polling began.
    trace_count     integer,
    trace_since     timestamptz,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    lease_owner     text,
    lease_until     timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    completed_at    timestamptz
);
CREATE INDEX judgements_queue_idx ON judgements (next_attempt_at) WHERE status IN ('queued', 'waiting_traces');

-- +goose Down
DROP TABLE judgements;
