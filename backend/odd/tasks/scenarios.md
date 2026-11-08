# Scenarios

Objective: reusable personas, scenario groups and scenarios (goal, hidden facts, weighted criteria, end conditions, interruptions, simulator and judge settings) that the Runs milestone will execute.
Branch: `feat/scenarios` (from `feat/agents`). Delivery: ask-on-risk (no remote yet).
Test runner: `go test ./...` (backend), `pnpm --filter web lint typecheck test build` (web).

## Specs

- **S1** Personas (`/api/v1/workspaces/{slug}/personas`), reusable across scenarios: name (≤120, unique per workspace, case-insensitive), locale (BCP 47 like `es-MX`), temperament (`polite|hurried|skeptical|confused|angry|friendly|calm|detailed`), notes (≤2000), optional voice (`connectionId` of a connection with `tts` or `realtime` capability in the workspace, `voice` name ≤100). User (L1): "el poder crear escenarios de pruebas". Legacy pain point: personas were embedded per scenario and not reusable.
- **S2** Scenario groups (`/scenario-groups`): name (≤120, unique per workspace, case-insensitive), description (≤2000). Deleting a group keeps its scenarios (ungrouped).
- **S3** Scenarios (`/scenarios`): name (≤160, unique per agent, case-insensitive), agent (required, same workspace, cannot change), persona (required, same workspace), group (optional), tags (≤20 unique, each ≤40), context (≤20000), goal (1..5000), instructions (≤10000), facts (≤50 `{key,value}`, keys unique case-insensitive, key ≤80, value ≤500; revealed only when asked), criteria (1..30 `{id,text,kind must|should,source transcript|trace|timing,weight 1..100}`, weights total exactly 100, ids unique; the server assigns ids to new criteria; `trace` requires the agent to have a trace connection; `timing` only for voice agents), endConditions (`maxDurationSeconds` 30..1800 default 360, `maxSilenceSeconds` 5..120 default 15 voice only, `maxTurns` 1..100 default 20 text only, `hangUpWhen` `goal_or_goodbye|goodbye|never` default `goal_or_goodbye`), interruptions (voice only, ≤10 `{agentTurn ≥1 unique, afterMs 300..60000}`, only when the agent's codec supports interruptions), simulator (`mode` `pipeline|realtime` for voice, `text` for text; optional `connectionId` + `model`: pipeline/text need an `llm` connection, realtime a `realtime` one), judge (optional `connectionId` with `llm` capability + `model`). Validation errors use dotted field paths.
- **S4** Listing filters: `GET /scenarios?agentId=&groupId=&tag=&q=` (q matches name or goal, case-insensitive); `groupId=none` lists ungrouped scenarios. `POST /scenarios/{id}/duplicate` copies a scenario with name "<name> (copy)" (suffix number when taken).
- **S5** Roles: any member reads; owner, admin and member create, update, delete and duplicate personas, groups and scenarios (authoring is everyday work); viewers are read-only; API keys act as member.
- **S6** Referential integrity: deleting a persona or agent used by scenarios answers 409 naming how many scenarios use it; deleting a connection clears it from personas and scenarios; deleting a group ungroups its scenarios.
- **S7** Web: Personas page (cards with temperament and locale, create/edit dialog with voice picker from tts/realtime connections); Scenarios page with group sidebar, filters (agent, tag, search) and list; scenario editor page (agent and persona pickers, goal, instructions, context, facts table, criteria editor with live weight total and "distribute evenly", end conditions, interruptions for voice agents, simulator and judge pickers, duplicate and delete). Groups can be created, renamed and deleted from the sidebar.

Out of scope (Runs milestone): executing scenarios, runtime defaults for simulator/judge, import/export.

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S1-S7 | inline | done | contract in `backend/api/openapi.yaml` |
| T2 | S1-S6 | inline | done | 5 integration tests RED (404s) → GREEN; full suite green |
| T3 | S7 | delegated (parallel with T2: disjoint surface `apps/web/**`) | done | 75 Vitest tests; lint/typecheck/build green |
| T4 | S1-S7 | inline E2E | done | persona created in UI, scenario created in editor (server-assigned criterion id, defaults), list + tag filter against the real API |

Risk: item 3 is touched only by S6 changing `DELETE /agents/{id}` and connection deletes for agents/scenarios that are referenced (unreleased API, no consumers yet). No secrets or outbound calls in this milestone.

## Log

- **L1** (2026-11-06, user, from Foundation L1): "...el poder generar conexion con el agente, el poder crear escenarios de pruebas y tener todo el informe de las evaluaciones...".
- **L2** (2026-11-07, user): "Sí, Scenarios".
- **L3** (2026-11-07): Legacy `modules/scenarios/domain.py`: temperaments polite, hurried, skeptical, confused, angry, friendly, calm, detailed; limits name 160, tags 20×40, context 20000, goal 5000, instructions 10000, persona name 120 / notes 2000, facts 50 (key 80, value 500), criteria 1..30 (text 500, weights total 100, unique ids), interruptions ≤10 (after 300..60000 ms, unique agent turn), end conditions max duration 30..1800 default 360, max silence 5..120 default 15 (voice), max turns 1..100 default 20 (text), hang up when goal_or_goodbye|goodbye. Decisions: personas become a workspace resource; simulator/judge connection and model are per scenario and optional until Runs defines defaults; added `hangUpWhen: never`.
- **L4** (2026-11-07): E2E found a typed locale appended to the prefilled `en-US` and accepted by a loose regex; tightened to language[-Script][-REGION] (`es-MX-extra` and `en-USA-MX` now rejected; shape-valid tags like `en-Uses-MX` still pass without a code list). Follow-ups noted by the web writer: codec catalog has no interruption-support flag (the UI shows interruptions for every voice agent and relies on the server error); hidden end-condition fields are sent as defaults.
