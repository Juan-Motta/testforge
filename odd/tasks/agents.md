# Agents

Objective: workspace agents defined as a generic transport (WebSocket or HTTP) plus a codec, with presets (Vivi Live, OpenAI Realtime, Vivi text, generic JSON), encrypted agent secrets, safe templating and a connection probe.
Branch: `feat/agents` (from `feat/connections`). Delivery: ask-on-risk (no remote yet).
Test runner: `go test ./...` (backend), `pnpm --filter web lint typecheck test build` (web).

## Specs

- **S1** Agent model and CRUD under `/api/v1/workspaces/{slug}/agents`: name (unique per workspace, case-insensitive), description (context shared by scenarios, ≤ 20000 chars), modality `voice|text`, transport, codec, variables (non-secret key/values), secrets (names + hints only in responses), optional Langfuse trace connection (must be a `traces` connection of the same workspace) and `traceSessionId` template (default `{{run.thread_id}}`). List/read for any member; create/update/delete for owner/admin. User (L2): "los agentes comparten algo en comun que el la conexion por ws y ya los parametros y cabeceras y demas deberian poder ser ajustable, es decir si jecesito query params deberia poder configurarls a voluntad, si necesito headers, igual, si necesit boy igual".
- **S2** Transport: `url` (`wss`/`https`; `ws`/`http` only with `ALLOW_PRIVATE_NETWORK`), scheme and host literal (templates allowed in path), editable `query` and `headers` lists, `body` template (HTTP only, must reference `{{message}}`), `initMessages` (WebSocket only, JSON templates sent after connecting), optional `preSession` HTTP call (method, URL, headers, body) whose JSON response fields are captured as `{{session.<name>}}`.
- **S3** Templates: variables `{{run.thread_id}}`, `{{var.<key>}}`, `{{secret.<name>}}`, `{{session.<name>}}`, `{{message}}` (HTTP body only), `{{audio}}` (generic JSON codec send template only). Unknown or misplaced references are rejected on save (422 with the field path). Values are escaped for their context: JSON-string escaping inside JSON templates, URL escaping in URL paths and query values, CR/LF rejected in headers.
- **S4** Codecs (`GET /api/v1/agent-codecs` catalog with option fields): voice `vivi_live`, `openai_realtime` (option for the Vivi `playback.interrupted` extension), `json_audio` (configurable type field, event types and dot paths for ready, agent audio, agent/user transcripts, send-audio template, close message); text `http_json` (reply path) and `http_stream` (line-delimited `data:` JSON with delta, skip and end selectors). All voice codecs use base64 PCM16 mono with a configurable sample rate (default 24000). The codec interface also covers what the runner will need (send audio/text, request agent turn, interrupt, close).
- **S5** Presets (`GET /api/v1/agent-presets`): ready-made drafts for Vivi Live, Vivi Realtime (OpenAI Realtime protocol), OpenAI Realtime, Vivi text (HTTP stream), generic JSON WebSocket and generic HTTP JSON. A preset only fills the editable form. User (L2): "vivi debe ser una aplicacion especifica de un agente general".
- **S6** Secrets: agent secrets sealed like connection secrets (bound to workspace, agent and secret name), never returned. Update semantics: provided list replaces names; an empty value keeps the stored value; when the transport or pre-session URL changes, every secret must be entered again (same rule as connections).
- **S7** Probe: `POST .../agents/{id}/probe` (member or above) connects with the stored config and a fresh thread id, runs the pre-session call and init messages, listens (voice) or sends one message (text, default "Hello"), for up to `durationSeconds` (default 8, max 30), closes per codec, and returns `ok`, a summary, a timestamped event log (connect, send, ready, audio with ms of audio, transcript, reply, error, close), captured transcript/reply and the resolved trace session id. Logs never contain secret values. Outbound connections follow the egress guard (S5 of Connections), no cross-host redirects.
- **S8** Web: Agents list and editor matching the design canvas (presets, transport editor with query/headers/body/handshake, codec picker with generated options, variables and secrets, trace settings, probe panel with live-looking log). Read-only for member/viewer, probe for member+.

Out of scope (Runs milestone): turn-taking, simulated caller audio, barge-in accounting, protocol auto-detection, binary PCM codec, resampling.

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S1-S8 | inline | done | contract in `backend/api/openapi.yaml` |
| T2 | S1-S3, S5, S6 | inline | done | tmpl RED→GREEN; 8 integration tests RED (404s) → GREEN |
| T3 | S4, S7 | inline | done | codec RED→GREEN; probe tests vs fake Vivi Live WS, JSON relay with pre-session, Vivi text stream; `-race` clean |
| T4 | S8 | delegated (parallel with T2-T3: disjoint surface `apps/web/**`) | pending | |
| T5 | S1-S8 | inline E2E + independent verifier (Risk item 2: secrets, SSRF, template injection) | pending | |

## Log

- **L1** (2026-11-07, user): "Sí, Agents".
- **L2** (2026-11-06, user, from Foundation L2): "vivi debe ser una aplicacion especifica de un agente general es decir los agentes comparten algo en comun que el la conexion por ws y ya los parametros y cabeceras y demas deberian poder ser ajustable, es decir si jecesito query params deberia poder configurarls a voluntad, si necesito headers, igual, si necesit boy igual". Decision accepted ("ok"): built-in codecs plus a configurable generic JSON codec.
- **L3** (2026-11-07): Legacy wire formats mapped (targets/vivi_live.py, vivi_realtime.py, relay.py, http_chat.py). Vivi Live: `voice.ready`, `voice.audio.delta{delta}`, `voice.transcript.update{role,messageId,text}` cumulative, client `voice.input.audio{audio}` and `voice.close`; no turn-end event. Vivi Realtime: OpenAI Realtime GA allow-list plus `playback.interrupted{itemId,responseId,contentIndex,audioEndMs}`; ready = first `session.updated`; close = WebSocket close 1000. Relay: optional POST returning `session_id`, `{type:audio,audio}`/`{type:stop}`, `ready`, `audio`, `agent_transcript{delta}`. Vivi text: `POST .../channelsApi/{channelId}/stream` body `{message, threadId}`, lines `data: <json>`, delta when `event=on_chat_model_stream` and no `type`, end `event=on_chat_end`. Assumptions: auto-detection dropped (presets choose the codec); relay is covered by `json_audio` + pre-session capture.
- **L4** (2026-11-07): Implementation notes: template parser treats `}}` as literal (nested JSON) and rejects stray `{{`; probe logs drop query strings and redact secret values (raw and JSON-escaped); probe reader goroutine exits on context cancel; WebSocket dial uses the egress client with `Timeout = 0` (coder/websocket uses the context).
