-- +goose Up
CREATE TABLE agents (
    id                  uuid PRIMARY KEY,
    workspace_id        uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name                text NOT NULL,
    description         text NOT NULL DEFAULT '',
    modality            text NOT NULL CHECK (modality IN ('voice', 'text')),
    transport           jsonb NOT NULL,
    codec               jsonb NOT NULL,
    variables           jsonb NOT NULL DEFAULT '[]'::jsonb,
    -- [{name, ciphertext, hint}]; ciphertexts are sealed per workspace, agent and name.
    secrets             jsonb NOT NULL DEFAULT '[]'::jsonb,
    trace_connection_id uuid REFERENCES connections (id) ON DELETE SET NULL,
    trace_session_id    text NOT NULL,
    created_by          uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX agents_workspace_name_idx ON agents (workspace_id, lower(name));

-- +goose Down
DROP TABLE agents;
