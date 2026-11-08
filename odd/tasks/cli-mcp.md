# CLI and MCP

Objective: reach a workspace from a terminal, CI and AI assistants with a workspace API key: a `testforge` CLI and an MCP server (stdio) built on one typed HTTP client of the public API.
Branch: `feat/cli-mcp` (from `feat/evaluation`). Delivery: ask-on-risk (no remote yet); work-unit commits.
Test runner: `go test ./...` (backend).

## Specs

- **S1** Config and auth: `testforge login --url <api-url>` reads the API key from `TESTFORGE_API_KEY` or the first line of stdin, checks it with `GET /api/v1/auth/me` (must answer `authMethod: api_key`) and stores `{url, apiKey, workspace}` in `$XDG_CONFIG_HOME/testforge/config.json` (default `~/.config/testforge`), directory 0700, file 0600. `logout` removes the file; `whoami` prints user, workspace and URL. Precedence: flags (`--url`, `--workspace`) over env (`TESTFORGE_URL`, `TESTFORGE_API_KEY`, `TESTFORGE_WORKSPACE`) over the file. The workspace defaults to the key's workspace. The key is never printed and is only sent to the configured URL over `https`, or `http` to a loopback host.
- **S2** Commands (table output, `--json` prints the API's JSON): `agents list|get`, `scenarios list|get` (filters `--agent`, `--tag`, `--group`, `--q`), `runs list` (filters `--batch`, `--scenario`, `--agent`, `--status`, `--limit`), `runs get <id>` (metrics, judgement and transcript), `runs cancel <id>`, `runs judgement <id>`, `runs rejudge <id>`, `reports [--days 7|30|90] [--agent] [--scenario]`. API errors print the API message (and field errors) on stderr with exit 1.
- **S3** Launch for CI: `runs launch` takes `--scenario <id>` (repeatable) or selects scenarios with `--agent`/`--tag`/`--group`, plus `--repetitions`, `--concurrency`, `--label`. With `--wait` (also `runs wait --batch <id>`) it polls until every run is terminal and every judgement is terminal (`--timeout`, default 30m), prints one line per run (status, score, passed) and exits 0 when every run completed and no judged run failed, 2 otherwise (a run not completed, a judgement failed, or `passed=false`), 1 on usage/API errors or timeout.
- **S4** MCP: `testforge mcp` serves MCP over stdio with the same config. Tools: `whoami`, `list_agents`, `list_scenarios`, `get_scenario`, `launch_runs`, `list_runs`, `get_run`, `get_judgement`, `rejudge_run`, `cancel_run`, `reports_overview`; read tools carry `readOnlyHint`. Results are the API's JSON; API errors become tool errors with the API message.
- **S5** Docs: README section (install/build, login, CI example, MCP client config); `--help` for every command.

Out of scope: creating or editing agents/scenarios/connections from the CLI, a remote (HTTP) MCP transport, OAuth, live event streaming.

## Tasks

| ID | Specs | Route | Status | Evidence |
|---|---|---|---|---|
| T1 | S1, S2 | inline | pending | |
| T2 | S3 | inline | pending | |
| T3 | S4 | inline | pending | |
| T4 | S5 | inline | pending | |
| T5 | S1-S5 | inline E2E + independent verifier (Risk item 2: API key storage/transport; item 5: new dependency) | pending | |

## Log

- **L1** (2026-11-06, user, from Foundation L1): "...tener usuarios, workspaces y conexion via api/cli y mcp."
- **L2** (2026-11-07, user): "Yes, CLI and MCP".
- **L3** (2026-11-07): The API already supports workspace API keys (`Authorization: Bearer tf_...`, member role, one workspace; `GET /auth/me` lists that workspace). No legacy CLI/MCP exists. MCP Go SDK: `github.com/modelcontextprotocol/go-sdk` v1.8.0 (`mcp.NewServer`, generic `mcp.AddTool`, `StdioTransport`). Decisions: one Go binary `backend/cmd/testforge` (CLI + `mcp` subcommand) so the MCP server reuses the CLI's config and client; stdlib `flag`; the launch response has no run ids, so waiting lists runs by `batchId`.
