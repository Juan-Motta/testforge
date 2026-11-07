-- +goose Up
CREATE TABLE connections (
    id                   uuid PRIMARY KEY,
    workspace_id         uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name                 text NOT NULL,
    kind                 text NOT NULL,
    config               jsonb NOT NULL DEFAULT '{}'::jsonb,
    secret_ciphertext    text NOT NULL,
    secret_hint          text NOT NULL,
    last_test_ok         boolean,
    last_test_message    text,
    last_test_latency_ms integer,
    last_tested_at       timestamptz,
    created_by           uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX connections_workspace_name_idx ON connections (workspace_id, lower(name));

-- +goose Down
DROP TABLE connections;
