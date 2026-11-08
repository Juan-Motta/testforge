# TestForge

Test voice and text AI agents: connect to an agent, run scenarios with a
simulated user, pull traces from Langfuse, evaluate and report.

The repository has two projects: `backend/` (Go API, worker, CLI and MCP; this
folder) and `frontend/` (Next.js web app, see [its README](../frontend/README.md)).
`compose.yaml` at the root runs the whole stack, or only Postgres for development.

See [docs/architecture.md](docs/architecture.md) for the design and roadmap,
and [api/openapi.yaml](api/openapi.yaml) for the API contract.

## Requirements

- Go 1.27+
- Node.js 24+ and pnpm 12+
- Docker (local Postgres, or the whole stack)

## Run everything with Docker

From the repository root:

```bash
docker compose up --build
```

This starts Postgres, the API (:8080, applies migrations), the worker and the
web app. Open http://localhost:3000 and sign up. The compose file uses a
development-only `SECRET_KEY` and allows private network addresses, so agents
running on your machine are reachable at `http://host.docker.internal:<port>`.
Override `SECRET_KEY`, `ALLOW_PRIVATE_NETWORK`, `APP_ORIGINS` or `LOG_FORMAT`
from your shell. Run audio is kept in the `artifacts` volume.

## Develop locally

```bash
cd backend
make db-up      # Postgres on localhost:5433
make api        # Go API on :8080 (applies migrations)
make worker     # executes queued runs (separate terminal, in backend/)

cd frontend
pnpm install
pnpm dev        # Next.js on :3000, proxies /api to the Go API
```

Open http://localhost:3000 and sign up.

## Configuration

The API reads environment variables; defaults suit local development.

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_URL` | `postgres://testforge:testforge@localhost:5433/testforge?sslmode=disable` | Postgres connection |
| `HTTP_ADDR` | `:8080` | Listen address |
| `APP_ORIGINS` | `http://localhost:3000` | Comma-separated origins allowed to send cookie-authenticated unsafe requests |
| `COOKIE_SECURE` | `false` | Mark the session cookie `Secure` (set `true` behind HTTPS) |
| `AUTO_MIGRATE` | `true` | Apply migrations when `serve` starts |
| `LOG_FORMAT` | `text` | `text` or `json` |
| `SECRET_KEY` | required (`make api` sets a dev-only key) | Base64 32-byte key that encrypts provider credentials. Generate with `openssl rand -base64 32`; losing it makes stored secrets unreadable |
| `ALLOW_PRIVATE_NETWORK` | `false` (`make api` sets `true`) | Let provider URLs use `http` and private addresses, e.g. a local Langfuse. Keep `false` in production |
| `ARTIFACTS_DIR` | `./data/artifacts` | Where the worker stores run audio (API and worker must share it) |
| `WORKER_CONCURRENCY` | `3` | Runs one worker process executes at a time |
| `JUDGE_CONCURRENCY` | `2` | Judgements one worker process evaluates at a time (1–20) |
| `JUDGE_TRACE_WAIT` | `3m` | How long to wait for Langfuse traces to settle before judging; `0` reads once without waiting |
| `JUDGE_TRACE_POLL` | `20s` | How often to re-poll Langfuse while waiting (1s–10m) |
| `TEST_DATABASE_URL` | `postgres://testforge:testforge@localhost:5433/postgres?sslmode=disable` | Server where tests create throwaway databases |

The web app reads `API_URL` (default `http://localhost:8080`). `next dev` reads it at startup; for `next build`/`next start` the rewrite destination is fixed at **build** time, so set `API_URL` when building.

## Tests

```bash
make test                                      # in backend/: unit + integration tests against a throwaway database
pnpm lint && pnpm typecheck && pnpm test && pnpm build   # in frontend/
```

## Authentication

- Browser: `tf_session` HttpOnly cookie from sign-up or log-in.
- Programmatic (CLI, MCP, CI): workspace API keys, sent as
  `Authorization: Bearer tf_...`. A key reaches only its workspace, with the
  `member` role. Create them in **Settings → API keys**.

## CLI and MCP

`testforge` is one binary: a command-line client for terminals and CI, and an
MCP server for AI assistants. Both use a workspace API key.

```bash
make build-cli   # in backend/
```

```bash
bin/testforge login --url https://testforge.example.com
```

```bash
bin/testforge runs launch --tag smoke --label "$GIT_SHA" --wait
```

`login` prompts for the key, or reads it from `TESTFORGE_API_KEY` or stdin. It
stores the URL, key and workspace in `~/.config/testforge/config.json` (or under
`$XDG_CONFIG_HOME`), readable only by you. The key is only ever sent:

- over https, or over http to localhost;
- to the URL it was stored for.

| Setting | Flag | Environment |
|---|---|---|
| API URL | `--url` | `TESTFORGE_URL` |
| API key | stdin on `login` | `TESTFORGE_API_KEY` |
| Workspace | `--workspace` | `TESTFORGE_WORKSPACE` (default: the key's workspace) |

Flags override the environment, and the environment overrides the stored login.
Every command accepts `--json` to print the API's JSON. `testforge -h` lists all
commands.

### In CI

`runs launch --wait` waits for every run and its evaluation, then prints one
line per run. `runs wait --batch <id>` does the same for a batch already
launched. The exit code tells CI the result:

| Exit code | Meaning |
|---|---|
| `0` | Every run completed, and none failed its evaluation. |
| `2` | At least one run did not complete, its evaluation failed, or it did not pass. |
| `1` | Usage error, API or network error, or `--timeout` reached (default 30m). |

A completed run counts as passed when its scenario has no judge.

```yaml
# GitHub Actions
- run: testforge runs launch --tag smoke --label "${{ github.sha }}" --wait
  env:
    TESTFORGE_URL: https://testforge.example.com
    TESTFORGE_API_KEY: ${{ secrets.TESTFORGE_API_KEY }}
```

### MCP

`testforge mcp` serves the Model Context Protocol over stdio. It reads the same
stored login or environment variables as the CLI. Every tool returns the API's
JSON.

| Area | Tools |
|---|---|
| Workspace | `whoami`, `list_agents`, `list_scenarios`, `get_scenario` |
| Runs | `launch_runs`, `list_runs`, `get_run`, `cancel_run` |
| Evaluation | `get_judgement`, `rejudge_run`, `reports_overview` |

To add it to Claude Code:

```bash
claude mcp add testforge -- /path/to/testforge mcp
```

For clients configured with JSON:

```json
{ "mcpServers": { "testforge": { "command": "/path/to/testforge", "args": ["mcp"] } } }
```
