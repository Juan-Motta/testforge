# Connections

Objective: workspace-scoped provider connections (LLM, speech, traces) with encrypted secrets, a provider catalog and connection tests.
Branch: `feat/connections` (from `feat/foundation`). Delivery: ask-on-risk (no remote yet).
Test runner: `go test ./...` (backend), `pnpm --filter web lint typecheck test build` (web).

## Specs

- **S1** Provider catalog: `GET /api/v1/connection-kinds` describes each kind (label, description, capabilities `llm|tts|stt|realtime|traces`, form fields) so the web app, CLI and MCP render and validate forms from one source. Kinds: `openai`, `anthropic`, `azure_openai`, `azure_speech`, `langfuse`. Roadmap: "Provider connections (LLM, TTS, STT, Langfuse) with encrypted secrets". User: "el poder conectarse a servicios como langfuse para poder obtener trazas y evaluarlo".
- **S2** CRUD under `/api/v1/workspaces/{slug}/connections`: list and read for every member (viewer included); create, update and delete for owner/admin only (API keys act as member, so they cannot manage). Names unique per workspace (409). Config validated per kind (422 with `config.<field>` keys). Update keeps the stored secret when `secret` is omitted.
- **S3** Secrets: encrypted at rest with AES-256-GCM using `SECRET_KEY` (base64, 32 bytes), bound to the connection and workspace via additional data, never returned by the API; responses carry only `secretHint` (last 4 characters). The API refuses to start without a valid key.
- **S4** Connection test: `POST .../connections/{id}/test` (member or above) calls the provider with the stored credentials, records `lastTest` (ok, message, latency, time) and returns it. Upstream errors are summarized without echoing secrets.
- **S5** Egress safety: provider URLs must be `https`; `http` and private, loopback or link-local addresses are allowed only when `ALLOW_PRIVATE_NETWORK=true` (local development, e.g. self-hosted Langfuse). Enforced at dial time, so DNS cannot bypass it.
- **S6** Web: Connections page with a provider picker, a form generated from the catalog, list of connections with capability badges and last-test status, test, edit (secret optional) and delete with confirmation; read-only for member/viewer apart from testing (member).

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S1-S6 | inline | done | contract in `backend/api/openapi.yaml` |
| T2 | S1-S3 | inline | done | sealbox RED→GREEN; 7 integration tests written before code (RED not run separately) → GREEN |
| T3 | S4, S5 | inline | done | egress RED→GREEN; tester unit test; dial-time block test |
| T4 | S6 | delegated (parallel with T2-T3: disjoint surface `apps/web/**`) | done | 22 Vitest tests; lint/typecheck/build green; E2E against real API + local fake provider |
| T5 | S1-S6 | inline E2E + independent verifier (Risk item 2: credentials, SSRF) | done | verifier: S1-S6 pass, 0 blockers; advisories 1-4 and 7 fixed with RED→GREEN tests |

## Log

- **L1** (2026-11-07, user): "Sí, Connections" — next milestone after Foundation, per `docs/architecture.md` roadmap row "Connections | Provider connections (LLM, TTS, STT, Langfuse) with encrypted secrets".
- **L2** (2026-11-07): Legacy reference `test-forge-main/backend/src/voice_eval/modules/connections/domain.py`: kinds openai, azure_openai, azure_speech, langfuse; https required except localhost in development; MultiFernet secrets. Decisions: add `anthropic`; models are chosen where a connection is used (scenario, judge), not hard-coded in the connection; one secret per kind; catalog endpoint drives forms.
- **L3** (2026-11-07): Package named `sealbox` because the session's permissions deny writing to directories called `secrets`. `make api` exports a dev-only `SECRET_KEY` and `ALLOW_PRIVATE_NETWORK=true`. Connection `createdBy` is nullable so deleting a user keeps workspace credentials.
- **L4** (2026-11-07): E2E: OpenAI (3 models), Langfuse ("Voice prod") and a failing Azure OpenAI (HTTP 404 message) against a local fake provider; secrets never in list responses. Fixed: success toast now shows the server message. Unverified: client fetches started ~3.6-12 s after navigation while the browser pane was hidden (background-tab throttling suspected); re-check with the pane visible.
- **L5** (2026-11-07): Independent verifier: S1-S6 pass, no blockers. Fixed: (1) an admin could move a stored secret to a host they control by changing a URL and testing — a URL change now requires the secret again (S2 updated in the contract); (2) provider auth headers followed cross-host redirects — redirects to another host are refused; (3) non-ASCII secret hint 500; (4) more reserved IPv4/IPv6 ranges blocked; (7) `api migrate` no longer needs SECRET_KEY and `serve` validates the key before migrating. Left as advisories: PATCH ignores unknown fields like `kind` (baseline-wide decoder behavior), non-string config values get a generic JSON error, Langfuse success message echoes the upstream project name.
