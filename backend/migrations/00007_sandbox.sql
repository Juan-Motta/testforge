-- +goose Up
CREATE TABLE sandbox_agents (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id  uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name          text NOT NULL,
    description   text NOT NULL DEFAULT '',
    modality      text NOT NULL,
    -- SET NULL keeps the agent editable after its connection is deleted; chat then answers 422.
    connection_id uuid REFERENCES connections (id) ON DELETE SET NULL,
    model         text NOT NULL,
    system_prompt text NOT NULL,
    temperature   double precision,
    memory        jsonb NOT NULL,
    tools         jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_by    uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX sandbox_agents_workspace_name_idx ON sandbox_agents (workspace_id, lower(name));

-- Conversation memory: one row per message in a thread, in order.
CREATE TABLE sandbox_messages (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    agent_id   uuid NOT NULL REFERENCES sandbox_agents (id) ON DELETE CASCADE,
    thread_id  text NOT NULL,
    message    jsonb NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sandbox_messages_thread_idx ON sandbox_messages (agent_id, thread_id, id);

-- Long-term memory: facts the model chose to remember, per user.
CREATE TABLE sandbox_memories (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id   uuid NOT NULL REFERENCES sandbox_agents (id) ON DELETE CASCADE,
    user_key   text NOT NULL,
    content    text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sandbox_memories_user_idx ON sandbox_memories (agent_id, user_key, created_at DESC);

-- +goose Down
DROP TABLE sandbox_memories;
DROP TABLE sandbox_messages;
DROP TABLE sandbox_agents;
