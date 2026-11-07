# TestForge

Test voice and text AI agents: connect to an agent, run scenarios with a
simulated user, pull traces from Langfuse, evaluate and report.

See [docs/architecture.md](docs/architecture.md) for the design and roadmap,
and [backend/api/openapi.yaml](backend/api/openapi.yaml) for the API contract.

## Requirements

- Go 1.27+
- Node.js 24+ and pnpm 12+
- Docker (local Postgres)

## Quick start

```bash
make db-up      # Postgres on localhost:5433
make api        # Go API on :8080 (applies migrations)
pnpm install
make web        # Next.js on :3000, proxies /api to the Go API
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
| `TEST_DATABASE_URL` | `postgres://testforge:testforge@localhost:5433/postgres?sslmode=disable` | Server where tests create throwaway databases |

The web app reads `API_URL` (default `http://localhost:8080`).

## Tests

```bash
make test       # backend: unit + integration tests against a throwaway database
pnpm --filter web lint && pnpm --filter web build
```

## Authentication

- Browser: `tf_session` HttpOnly cookie from sign-up or log-in.
- Programmatic (CLI, MCP, CI): workspace API keys, sent as
  `Authorization: Bearer tf_...`. A key reaches only its workspace, with the
  `member` role. Create them in **Settings → API keys**.
