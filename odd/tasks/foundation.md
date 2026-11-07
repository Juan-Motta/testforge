# Foundation

Objective: clean monorepo with Go API, Postgres, auth, workspaces, API keys and a modern Next.js shell.
Branch: `feat/foundation`. Delivery: ask-on-risk (no remote yet; PR strategy decided before first push).
Test runner: `go test ./...` (backend, needs local Postgres from `compose.yaml`), `pnpm lint && pnpm build` (web).

## Specs

- **S1** Monorepo: `apps/web` (Next.js, shadcn, Tailwind), `backend` (Go), `compose.yaml` with Postgres, `Makefile` with dev targets. "usar nextjs, shadcn, tailwind, y go para el backend con postgres".
- **S2** API skeleton: Go stdlib HTTP server, config from env, embedded SQL migrations, `/healthz`, JSON errors, OpenAPI contract in `backend/api/openapi.yaml`.
- **S3** Users and sessions: sign up, log in, log out, current user. Passwords hashed with argon2id; session cookie (HttpOnly, SameSite=Lax), only a hash of the token stored. "tener usuarios".
- **S4** Workspaces: sign-up creates a personal workspace; list, create and read workspaces; memberships with roles `owner|admin|member|viewer`; every workspace route checks membership. "workspaces".
- **S5** API keys per workspace: create (secret shown once), list, revoke; `Authorization: Bearer tf_...` authenticates API calls scoped to that workspace. "conexion via api/cli y mcp".
- **S6** Web shell: login and sign-up pages, workspace layout with sidebar (Agents, Scenarios, Personas, Runs, Reports, Connections, Settings), workspace switcher, light/dark theme, API keys settings page working end to end; other sections show empty states. "quiero que la ui sea limpia, moderna y que tenga una buena ui/ux".

Out of scope here (later milestones in `docs/architecture.md`): connections, agents, scenarios, runs, evaluation, CLI, MCP.

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S1, S2 | inline | done | `c6094ec`, `5c4c5a9`; `/healthz` smoke OK |
| T2 | S3, S4, S5 | inline | done | `5c4c5a9`; RED 12/12 failing against stub, GREEN `go test ./...` |
| T3 | S6 | delegated (parallel with T2: disjoint surface `apps/web/**`) | done | web shell commit; lint/typecheck/build green, 10 Vitest tests; E2E against real API (signup, key create/use/revoke, logout, wrong + right login, workspace create) |
| T4 | S1-S6 | independent verifier (Risk item 2: auth) | pending | |

## Log

- **L1** (2026-11-06, user): "quiero iniciar un nuevo proyecto, este proyecto tiene como idea principal el poder probar agentes de voz y texto, para ello tengo un proyecto el cual hace algo similar pero me gustaria mejorar la ui e iniciarlo limpio por ahora teniendo la capacidad de poder probar agentes de voz, texto usando los mismos principios es decir el poder conectarse a servicios como langfuse para poder obtener trazas y evauarlo, el poder generar conexion con el agente, el poder crear escenarios de pruebas y tener todo el informe de las evaluaciones, quiero que la ui sea limpia, mordena y que tenga una buena ui/ux, para las tecnologias estaba pensando usar nextjs, shadcn, tailwind, y go para el backend con postgres no se que tan viable sea pero me gustaria tener un buen diseno y enfoque y puedes que mas adelante se extiendan las funcionalidades entonces debe estar preparado asi como tener usuarios, workspaces y conexion via api/cli y mcp."
- **L2** (2026-11-06, user): "vivi debe ser una aplicacion especifica de un agente general es decir los agentes comparten algo en comun que el la conexion por ws y ya los parametros y cabeceras y demas deberian poder ser ajustable, es decir si jecesito query params deberia poder configurarls a voluntad, si necesito headers, igual, si necesit boy igual" — Decision: generic agent = transport + codec; built-in codecs plus a configurable generic JSON codec (user: "ok").
- **L3** (2026-11-07): Go 1.27.3 installed with Homebrew at the user's request. Legacy reference: `test-forge-main` (FastAPI). RDD is off globally.
- **L4** (2026-11-07): Design canvas for iteration: https://claude.ai/artifact/VGyvMoq5VabjDwA1c91qXN (Login, Overview, Agent transport+codec, Run report, API keys). User chose "Seguir integración".
- **L5** (2026-11-07): Integration bug found E2E: the global 401 handler redirected wrong-credential log-ins to /login, hiding the error. Fixed with `meta.handlesUnauthorized` on log-in/sign-up mutations (`src/lib/api/unauthorized.ts`, RED then GREEN).
