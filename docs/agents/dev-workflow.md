# Development workflow

## Commands

Use **Make** from the repo root (see root `Makefile`). Run `make help` for all targets.

| Goal | Command |
|------|---------|
| Install dependencies (backend + frontend) | `make install` |
| Run backend + frontend dev servers | `make dev` |
| Backend only (migrations + Jet codegen + server) | `make backend/dev` |
| Frontend only (Vite) | `make frontend/dev` |
| All tests | `make test` |
| Lint both projects | `make lint` |
| Production build both | `make build` |
| Clean artifacts | `make clean` |

**Dev servers:** Backend logs go to **`dev.log`** (JSON when redirected; color in an interactive terminal). `make backend/dev` runs the API in the **foreground** with `tee` so you see logs in the terminal and in `dev.log`. `make frontend/dev` runs Vite in the foreground only (not `dev.log`). `make dev` starts both in the background and tails `dev.log`. `make dev-stop` stops background processes.

**Mapping sync workers:** Off by default (`SYNC_WORKERS_ENABLED=false`). When true, analysis runs on **`SYNC_ANALYSIS_CRON_SPEC`** (default `0 * * * * *` = every minute). Auto executor cron is off by default (`SYNC_EXECUTOR_AUTO_ENABLED=false`); V1 execution will be manual per item (see `docs/handoffs/EXECUTOR_V1_MANUAL_PATCH_PLAN.md`).

**Frontend dev server:** Vite on **:5173**. In `frontend/.env` (or `.env.local`):

- `VITE_API_URL` — backend origin for API calls (host must match `PUBLIC_URL`: both `localhost` or both `127.0.0.1`).
- `VITE_PUBLIC_URL` — optional; same host as `PUBLIC_URL` for OAuth login links.
- `FRONTEND_URL` (backend) — where you open the UI (`http://localhost:5173`). Do **not** set this to `127.0.0.1:5173` unless Vite is bound to that host.

**Formatting:** There is no root `make format`. The frontend can use `npm run format` inside `frontend/` when needed; backend uses `make backend/lint` (`go fmt`, `go vet`).

**Node / Go versions:** See `.nvmrc` (Node) and `backend/go.mod` (Go toolchain). Prefer matching those locally and in CI.
