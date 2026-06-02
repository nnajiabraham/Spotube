# Agent guide (start here)

## Spotube quick reference

| Service | Port | Start |
|---------|------|--------|
| Go backend (Echo + SQLite) | 8090 | `make backend/dev` |
| React frontend (Vite) | 5173 | `make frontend/dev` |
| Both (parallel) | 8090 + 5173 | `make dev` |

- **Install:** `make install` (root orchestrates `backend/` and `frontend/`).
- **Tests / lint / build:** `make test`, `make lint`, `make build`.
- **Env (local, not committed):** `backend/.env` from `backend/env.example`; `frontend/.env` with `VITE_API_URL=http://localhost:8090`. OAuth keys can be empty at first; the app can guide setup via the UI.
- **Dev logs:** `make backend/dev` appends API logs to `dev.logs` and tails them. `make frontend/dev` prints Vite output in the terminal only (not `dev.logs`).
- **Sync workers:** `SYNC_WORKERS_ENABLED=false` by default in `backend/.env`. Analysis cron only runs when `true`; executor is still a stub.

### Cursor Cloud / VM notes

- **Go:** `backend/go.mod` requires Go **1.24.2**. If the default `go` on PATH is older, prepend `/usr/local/go/bin` to `PATH`.
- **Node:** Use the version in `.nvmrc` (e.g. `nvm use`).
- **SQLite:** Created automatically at `backend/data/spotube.db` when the backend runs; no separate DB service.
- **Config test gotcha:** `internal/config.TestLoadDefaults` expects default `LOG_LEVEL` when the variable is unset. If `LOG_LEVEL` is set from `backend/.env`, that test can fail; use `LOG_LEVEL= make backend/test` or run tests without loading `.env`.
- **Jet first run:** `make backend/dev` / `make backend/db/gen` may download extra Go modules on first codegen; that is normal.

### MCP configuration

`.cursor/mcp.json` configures optional MCP servers (e.g. shadcn, chrome-devtools). Adjust or remove entries to match your environment.

## Essentials (applies to almost every task)

- Prefer **make** targets from the repo root over raw `go run` / ad hoc npm scripts (see [Development workflow](docs/agents/dev-workflow.md)).
- Read surrounding code before changing it; keep edits minimal and consistent with the file’s style.

## More detailed guidance (progressive disclosure)

- [Project orientation](docs/agents/project.md)
- [Development workflow](docs/agents/dev-workflow.md)
- [Coding conventions](docs/agents/coding-conventions.md)
- **Always follow:** [Communication style (incl. “banter mode”)](docs/agents/communication-style.md)
