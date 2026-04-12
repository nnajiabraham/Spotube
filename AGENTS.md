# AGENTS.md

## Cursor Cloud specific instructions

### Services overview

| Service | Port | Start command |
|---|---|---|
| Go backend (Echo + SQLite) | 8090 | `make backend/dev` |
| React frontend (Vite) | 5173 | `make frontend/dev` |
| Both (parallel) | 8090 + 5173 | `make dev` |

All available commands are documented in the root `Makefile` and per-project Makefiles (`backend/Makefile`, `frontend/Makefile`). Run `make help` for a full list.

### Runtime requirements

- **Go 1.24.2** — required by `backend/go.mod`. The VM has it at `/usr/local/go/bin/go`; ensure `PATH` includes `/usr/local/go/bin` before `/usr/bin`.
- **Node 20.12.2** — specified in `.nvmrc`. Activate with `nvm use` (nvm is pre-installed).
- **No Docker, no external databases** — SQLite is embedded; the DB file is created automatically at `backend/data/spotube.db` on first `make backend/dev`.

### Environment files (not committed)

- `backend/.env` — copy from `backend/env.example`. OAuth credentials can be left blank; the app's setup wizard handles first-time configuration.
- `frontend/.env` — only needs `VITE_API_URL=http://localhost:8090`.

### Gotchas

- **Backend config test leaks from `.env`**: `TestLoadDefaults` in `internal/config` will fail if `LOG_LEVEL` is set in the environment (the `.env` sets it to `debug`, but the test expects the default `info`). The test does not clear `LOG_LEVEL` via `t.Setenv`. Running `LOG_LEVEL= make backend/test` works around this, or run tests from a shell without `.env` loaded.
- **Jet codegen downloads extra Go modules** the first time (`mysql`, `postgres` drivers) — this is expected and takes ~20 s on the first `make backend/dev` / `make backend/db/gen`.
- **`make backend/dev`** automatically runs migrations (`backend/db/up`) and Jet codegen (`backend/db/gen`) before starting the server, so no separate migration step is needed.
- **Frontend uses npm** (lockfile is `package-lock.json`), not pnpm or yarn.
