# Sandbox

Objective: let a workspace build hosted test agents (system prompt, model, memory, mock tools with fault injection), chat with them, generate their configuration and tools with AI, and test them with scenarios like any other agent.
Branch: `feat/sandbox` (from `main`). Delivery: ask-on-risk; work-unit commits.
Test runner: `go test ./...` in `backend/`; `pnpm lint && pnpm typecheck && pnpm test && pnpm build` in `frontend/`.

## Specs

- **S1** Sandbox agents (any member): `GET|POST /workspaces/{slug}/sandbox-agents`, `GET|PUT|DELETE .../sandbox-agents/{id}`. Fields: `name` (1–100, unique per workspace, case-insensitive), `description` (≤ 2000), `modality` (`text`; `voice` answers 422 "Voice sandbox agents are not available yet."), `connectionId` (an OpenAI or Azure OpenAI connection of the workspace; other kinds answer 422 "Sandbox agents support OpenAI and Azure OpenAI connections."), `model` (1–200), `systemPrompt` (1–20000), `temperature` (0–2, optional), `memory` `{conversation, longTerm}` booleans, `tools` (≤ 20). User (L2): "un lugar donde pueda crear agentes de pruebas tanto de voz como de texto, que puedan tener memoria y a los cuales les pueda asignar tools"; (L4): "los principales proveedores que usare seran azure oepnai y open ai".
- **S2** Mock tools: `name` (`^[a-zA-Z0-9_-]{1,64}$`, unique in the agent, not `remember`/`recall`), `description` (1–1000), `parameters` (JSON Schema whose `type` is `object`), `response` (text returned to the model; `{{args.<name>}}` is replaced by that argument, objects and arrays as JSON, missing as empty), `latencyMs` (0–30000), `errorRate` (0–1), `errorMessage` (≤ 500, default "The tool failed."). A call waits `latencyMs`, then with probability `errorRate` returns `{"error": errorMessage}` to the model, else the rendered response.
- **S3** Chat: `POST .../sandbox-agents/{id}/chat` `{message (1–8000), threadId? (≤ 200), userId? (≤ 200)}` with a session or a workspace API key → `{message, threadId, toolCalls: [{name, arguments, result, error, latencyMs}], usage: {inputTokens, outputTokens}}`. A missing `threadId` starts a new one (returned). The model may call tools for up to 8 rounds; past that the reply is 502 "The agent called tools too many times without answering." Provider failures answer 502 `upstream_error` with the provider message (secrets redacted).
  - Conversation memory on: messages (user, assistant, tool calls and results) are stored per `(agent, threadId)` and the last 50 are sent each turn; off: each message is answered alone and nothing is stored.
  - Long-term memory on: built-in tools `remember(fact)` and `recall(query)` store and search facts per `(agent, userId)` (`userId` default `anonymous`); `GET .../sandbox-agents/{id}/memories?userId=` lists them, `DELETE .../memories/{memoryId}` removes one.
- **S4** Link: `POST .../sandbox-agents/{id}/link` (owners and admins, session only, like API keys) creates a regular text agent named `<name> (sandbox)` (suffixed ` 2`, ` 3`… when taken) with codec `http_json` (`replyPath` `message`), URL `<PUBLIC_API_URL>/api/v1/workspaces/{slug}/sandbox-agents/{id}/chat`, header `Authorization: Bearer {{secret.api_key}}`, body `{"message":"{{message}}","threadId":"{{run.thread_id}}","userId":"{{var.userId}}"}`, variable `userId=test-user`, and secret `api_key` set to a new workspace API key named `Sandbox: <name>`. Returns the agent (201). `PUBLIC_API_URL` defaults to `http://localhost:8080`; compose sets `http://api:8080`.
- **S5** AI generation (any member): `POST .../sandbox/generate` `{kind: agent|tool, connectionId, model, description (1–4000), agent?}` returns a draft, never saved: `kind=agent` → `{name, description, systemPrompt, memory, tools}`; `kind=tool` → one tool (the optional `agent` draft gives context). Uses the connection's structured output (OpenAI, Azure OpenAI or Anthropic). Drafts are validated like S1/S2; an invalid model answer is 502 "The model returned an invalid draft: …". User (L3): "poder generar esas tools con ia ... e incluso poder generar la config de un agente con ia".
- **S6** Web: a Sandbox section: list; editor (identity, connection + model, system prompt, temperature, memory toggles, tools editor with JSON Schema, mock response, latency and error rate); "Generate with AI" for the whole agent (fills the form) and per tool; playground chat with tool calls, new thread and user id; memories list with delete; "Use in scenarios" (link) that opens the created agent.
- **S7** Docs: README section; architecture roadmap row.

Out of scope (later phases): voice sandbox agents, HTTP tools, Langfuse traces from sandbox agents, Anthropic tool calling, CLI/MCP sandbox commands.

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S1-S5 | inline | done | contract in `backend/api/openapi.yaml` (`afecea8`); no new lint errors beyond the existing operation-summary style |
| T2 | S1-S4 | inline | done | `afecea8`; integration tests: CRUD/validation/tenancy/viewer, tool loop + history, fault injection, 8-round limit, provider error redaction, long-term memory, link + scenario run end to end |
| T3 | S5 | inline | done | `afecea8`; generate agent/tool drafts, invalid draft 502, field validation |
| T4 | S6 | delegated (parallel with T2-T3: disjoint surface `frontend/**`) | done | `76a95b2`; 158 Vitest tests, lint/typecheck/build green; browser E2E on Docker with a fake OpenAI: generate draft → save → playground tool call → link → probe through `api:8080` |
| T5 | S1-S7 | inline E2E + independent verifier (Risk item 2: link mints API keys; chat accepts API keys) | done | verifier: S1-S7 PASS, no blockers; probes: tenancy, viewer, link key revoked on failure, no secret in responses, history never starts with orphan tool, LIKE escaping |

## Log

- **L1** (2026-11-08, user): "ok otra cosa que me gustaria agregar es un agent sandobx si es el nombre correcto, un lugar donde pueda crear agentes de pruebas tanto de voz como de texto, que puedan tener memoria y a los cuales les pueda asignar tools, como podria manejar eso antes de implementar algo".
- **L2** (2026-11-08): Proposal accepted with phases: 1 text agent + mock tools + memory + playground; 2 persistent memory, HTTP tools, fault injection; 3 voice. Hosted agents speak `http_json` so scenarios, runs and evaluation treat them like any agent.
- **L3** (2026-11-08, user): "iniciemos, ademas algo que me gustaria es poder generar esas tools con ia, es decir poder usar ia para generarlas y poder automatizar todo e incluso poder generar la config de un agente con ia seria posible?"
- **L4** (2026-11-08, user): "sabiendo que los principales proveedores que usare seran azure oepnai y open ai".
- **L5** (2026-11-08): Decisions: this delivery pulls long-term memory and fault injection forward from phase 2 (both small); the hosted endpoint is the workspace route authenticated by a workspace API key, so no new credential type; tool calling uses Chat Completions `tools` (OpenAI/Azure); generated tool schemas come back as a JSON string because strict structured outputs cannot describe arbitrary schemas.
- **L6** (2026-11-08): Backend and web committed (`afecea8`, `76a95b2`). Viewers cannot chat (the UI hides the playground for them); chat 403 added to the contract. Browser E2E passed on an isolated compose project.
- **L7** (2026-11-08): Verifier advisories: linked key is a full member key that outlives the agents (accepted design); link needs a reachable PUBLIC_API_URL; 8 rounds counts model requests; concurrent turns on one thread can interleave; generate `toolNames` now capped (20 names, 64 chars).
