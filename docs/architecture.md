# TestForge architecture

TestForge tests voice and text AI agents: it connects to an agent, drives a
simulated user through a scenario, collects transcripts, audio and traces
(Langfuse first), evaluates the conversation, and reports the results.

## Principles

1. **Multi-tenant from day one.** Every product row belongs to a workspace.
   Users reach workspaces through memberships with a role.
2. **One API, many clients.** The REST API described in
   `backend/api/openapi.yaml` is the only contract. The web app, the CLI and
   the MCP server are thin clients of that API.
3. **Durable execution.** Runs execute in a separate `worker` process through a
   Postgres-backed job queue, so they survive restarts and scale out.
4. **Generic agents, specific presets.** An agent is a generic WebSocket or
   HTTP target. Vendors such as Vivi or OpenAI Realtime are presets that fill
   the generic configuration; they are not special types.
5. **Screaming architecture.** Backend packages are named after features
   (`identity`, `workspace`, `apikey`, ...), each with its own domain, service,
   storage adapter and HTTP handlers. Cross-cutting code lives in `platform`.

## Layout

```text
apps/web/              Next.js (App Router), shadcn/ui, Tailwind CSS
backend/
  api/openapi.yaml     API contract (source of truth for every client)
  cmd/api/             HTTP API binary (also runs migrations)
  cmd/worker/          Job worker binary (runs, judgements) — later
  internal/platform/   config, database, HTTP helpers, crypto
  internal/<feature>/  domain + service + store + http per feature
  migrations/          SQL migrations (goose, embedded)
docs/                  architecture and decisions
odd/tasks/             feature documents (specs, tasks, log)
compose.yaml           local Postgres (and object storage later)
```

## Agent connection model

An agent has a **transport** and a **codec**.

- **Transport**: `ws` or `http`; URL, query parameters, headers and body are all
  editable and accept templates such as `{{secret.api_key}}` or
  `{{run.thread_id}}`. An optional pre-session HTTP call and an optional
  initial message cover handshakes.
- **Codec**: how messages inside the connection are encoded. Built-in codecs
  (OpenAI Realtime, Vivi Live, binary PCM) are tested code. A configurable
  "generic JSON" codec maps audio, text, end-of-turn and interruption fields
  through JSON paths for protocols without a built-in codec.

## Roadmap

| Milestone | Content |
|---|---|
| Foundation | Monorepo, API skeleton, auth, workspaces, API keys, web shell |
| Connections | Provider connections (LLM, TTS, STT, Langfuse) with encrypted secrets |
| Agents | Transport + codec model, presets, connection test |
| Scenarios | Reusable personas, scenarios, criteria, groups |
| Runs | Worker, job queue, conversation engine, live view, artifacts in S3 |
| Evaluation | Langfuse traces, LLM judge, reports and dashboards |
| Clients | CLI and MCP server on top of the API |
