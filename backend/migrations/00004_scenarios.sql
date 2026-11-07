-- +goose Up
CREATE TABLE personas (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name                text NOT NULL,
    locale              text NOT NULL,
    temperament         text NOT NULL,
    notes               text NOT NULL DEFAULT '',
    voice_connection_id uuid REFERENCES connections (id) ON DELETE SET NULL,
    voice_name          text,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX personas_workspace_name_idx ON personas (workspace_id, lower(name));

CREATE TABLE scenario_groups (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name         text NOT NULL,
    description  text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX scenario_groups_workspace_name_idx ON scenario_groups (workspace_id, lower(name));

CREATE TABLE scenarios (
    id                      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id            uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    -- RESTRICT: deleting an agent or persona in use answers 409 instead of losing scenarios.
    agent_id                uuid NOT NULL REFERENCES agents (id) ON DELETE RESTRICT,
    persona_id              uuid NOT NULL REFERENCES personas (id) ON DELETE RESTRICT,
    group_id                uuid REFERENCES scenario_groups (id) ON DELETE SET NULL,
    name                    text NOT NULL,
    tags                    text[] NOT NULL DEFAULT '{}',
    context                 text NOT NULL DEFAULT '',
    goal                    text NOT NULL,
    instructions            text NOT NULL DEFAULT '',
    facts                   jsonb NOT NULL DEFAULT '[]'::jsonb,
    criteria                jsonb NOT NULL,
    end_conditions          jsonb NOT NULL,
    interruptions           jsonb NOT NULL DEFAULT '[]'::jsonb,
    simulator_mode          text NOT NULL,
    simulator_connection_id uuid REFERENCES connections (id) ON DELETE SET NULL,
    simulator_model         text NOT NULL DEFAULT '',
    judge_connection_id     uuid REFERENCES connections (id) ON DELETE SET NULL,
    judge_model             text NOT NULL DEFAULT '',
    created_by              uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at              timestamptz NOT NULL DEFAULT now(),
    updated_at              timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX scenarios_agent_name_idx ON scenarios (agent_id, lower(name));
CREATE INDEX scenarios_workspace_idx ON scenarios (workspace_id, lower(name));
CREATE INDEX scenarios_tags_idx ON scenarios USING gin (tags);

-- +goose Down
DROP TABLE scenarios;
DROP TABLE scenario_groups;
DROP TABLE personas;
