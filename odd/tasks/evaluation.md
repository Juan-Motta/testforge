# Evaluation

Objective: judge every completed run against its scenario criteria with an LLM judge that cites evidence from the transcript, timings and the agent's Langfuse traces, and turn results into reports.
Branch: `feat/evaluation` (from `feat/runs`). Delivery: ask-on-risk (no remote yet); work-unit commits.
Test runner: `go test ./...` (backend), `pnpm --filter web lint typecheck test build` (web).

## Specs

- **S1** Judgement lifecycle: when a run finishes `completed`, a judgement is queued (`skipped` with a reason when the scenario has no judge connection/model). The worker processes judgements (`JUDGE_CONCURRENCY`, default 2, separate from runs) with the same claim/lease rules as runs. Statuses `queued|waiting_traces|running|completed|failed|skipped`. `POST .../runs/{id}/judgement` re-judges (member+); `GET .../runs/{id}/judgement` reads it. User (L1): "el poder conectarse a servicios como langfuse para poder obtener trazas y evauarlo ... y tener todo el informe de las evaluaciones".
- **S2** Traces: when the agent has a Langfuse trace connection and the run has a trace session id, the worker fetches the session's observations (v2 `GET /api/public/v2/observations?sessionId=` with a time window and cursor pagination; fallback to v1 traces + observations when v2 answers 404), waiting for ingestion: it re-polls every `JUDGE_TRACE_POLL` (default 20 s) until two consecutive polls return the same non-zero count, or `JUDGE_TRACE_WAIT` (default 3 min) passes. It stores trace stats (traces, observations, errors, models, tool calls, tokens, cost, slowest generation, time to first token) and a Langfuse session URL; status `available|unavailable|not_configured|error`. Calls go through the egress guard; secrets never appear in results.
- **S3** Judge: the scenario's judge connection/model evaluates each criterion from bounded, escaped evidence blocks (criteria, scenario, persona, agent description, numbered transcript, turn timings, trace digest) under the legacy rules: anchored bands 1–5, worst case wins, closed failure tags (capping tags cap at band 2), cited evidence (`ref` + exact `quote`), reasoning before the band, `observable=false` when evidence is missing, the simulated user is never judged. Three samples; per criterion the lower median band of the observable samples. Gating in code before the call: `trace` criteria are not observable when traces are unavailable; `timing` criteria when the run has no turn timings.
- **S4** Scoring: band → score 5:100, 4:90, 3:70, 2:50, 1:25; verdict 4–5 pass, 3 partial, 1–2 fail; overall score = weight-normalized mean over observable criteria (null when none); `passed` = at least one observable criterion and no `must` criterion failed. Results also carry a summary, strengths, prioritized suggestions (`area` prompt|tools|flow|latency|voice|other, `priority` high|medium|low) and failure-tag counts.
- **S5** Runs expose `judgementStatus`, `score` and `passed` in summaries (lists and detail) so lists can show scores.
- **S6** Reports: `GET .../reports/overview?days=7|30|90&agentId=&scenarioId=` returns totals by status, judged count, average score, pass rate, average p50/p95 latency, LLM tokens, a daily trend (runs, average score, pass rate), the five weakest criteria (lowest average score, with fail rate), per-scenario rows (runs, average score, pass rate, last run) and failure-tag counts. Any member.
- **S7** Web: run detail Evaluation panel (score, passed badge, summary, per-criterion verdict/band/score with reasoning and evidence quotes linked to transcript turns, failure tags, suggestions, strengths, trace stats with Langfuse link, judgement status while pending, re-judge); score column in the runs list; Reports page with filters (period, agent, scenario), KPI cards, trend chart, weakest criteria, per-scenario table, failure tags.

Out of scope: writing scores back to Langfuse, judge stability analysis, translations, pricing/cost estimation beyond Langfuse-reported cost.

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S1-S7 | inline | done | contract in `backend/api/openapi.yaml` |
| T2 | S2 | inline | done | langfuse RED→GREEN (v2 cursor, v1 fallback, digest, URL, egress) |
| T3 | S1, S3, S4, S5 | inline | done | judge RED→GREEN (gating, escaping, per-sample caps + lower median, scoring, 3 samples); E2E with fake judge + fake Langfuse (waiting_traces observed, score 91, rejudge, skipped, not_configured, must fail) |
| T4 | S6 | inline | done | report asserted in the E2E test (judged, avg, pass rate, 7-day trend, weakest criteria, scenarios, days=5 → 422) |
| T5 | S7 | delegated (parallel with T2-T4: disjoint surface `apps/web/**`) | done | 138 Vitest tests; lint/typecheck/build green; mock click-through incl. evidence links and re-judge |
| T6 | S1-S7 | inline E2E + independent verifier (Risk item 2: Langfuse credentials/egress; prompt injection into the judge; item 4: judgement claims) | pending | |

## Log

- **L1** (2026-11-06, user, from Foundation L1): "...el poder conectarse a servicios como langfuse para poder obtener trazas y evauarlo, el poder generar conexion con el agente, el poder crear escenarios de pruebas y tener todo el informe de las evaluaciones...".
- **L2** (2026-11-07, user): "Sí, Evaluation".
- **L3** (2026-11-07): Legacy judge (`judge/domain.py`, `prompt.py`): verdicts pass/partial/fail/not_observable; failure tags fabricated_information, wrong_information, instruction_ignored, unsafe_or_policy_violation (capping, band ≤ 2), missing_information_request, incomplete_task, off_topic, unclear_or_unnatural_speech, cut_off_or_incomplete_reply, slow_response, tool_or_trace_error; BAND_SCORE 5:100 4:90 3:70 2:50 1:25; 3 samples, lower median; gating traces_unavailable / no_timing_data; evidence refs A#/C#/T#/R#; tagged evidence blocks with escaping. Langfuse docs (2026-11-07): v2 `GET /api/public/v2/observations` filters `sessionId`, `traceId`, `fromStartTime`, `toStartTime`, `fields` groups (core, basic, time, model, usage, io, metrics), cursor pagination (`limit` ≤ 1000, `meta.cursor`); v1 observations use page/limit (≤ 100); basic auth public:secret.
- **L4** (2026-11-07): Decisions: caps apply per judge sample before the lower median (a single outlier capping tag does not dominate; a majority caps); the worker judges with the scenario's current judge settings when the scenario still exists (configure a judge after a run, then re-judge), else the snapshot's; judgements reaped from a lost worker are requeued (idempotent).
