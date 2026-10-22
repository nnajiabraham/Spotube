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

**Sync:** Off by default (`SYNC_WORKERS_ENABLED=false`). When true, the scheduler ticks on **`SYNC_ANALYSIS_CRON_SPEC`** (default every minute) and syncs each mapping whose `interval_minutes` has elapsed: analyze → queue `sync_items` → auto-apply up to 25 items (`SYNC_EXECUTOR_AUTO_ENABLED`, default true). The Mappings page "Sync now" button (`POST /api/collections/mappings/records/:id/sync`) runs the same pass on demand. See the README "How sync works" section.

**Hosts:** Use **`127.0.0.1`** everywhere in dev — Spotify rejects `localhost` redirect URIs. In `backend/.env`: `PUBLIC_URL=http://127.0.0.1:8090`, `FRONTEND_URL=http://127.0.0.1:5173`. In `frontend/.env`: `VITE_API_URL=http://127.0.0.1:8090`. Vite listens on all interfaces (`host: true`).

**OAuth testing:** Real provider logins need the account owner (Spotify emails a one-time code; Google may require a passkey). Drive the browser to the login page, then hand off to the user. Check `oauth_tokens.updated` in SQLite or `GET /api/auth/{spotify,youtube}/playlists` (401 = reconnect needed) to confirm.

**Formatting:** There is no root `make format`. The frontend can use `npm run format` inside `frontend/` when needed; backend uses `make backend/lint` (`go fmt`, `go vet`).

**Node / Go versions:** See `.nvmrc` (Node) and `backend/go.mod` (Go toolchain). Prefer matching those locally and in CI.
