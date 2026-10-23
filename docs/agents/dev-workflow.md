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

**Sync:** "Background sync" (`SYNC_WORKERS_ENABLED`, default false) and "Apply changes automatically" (`SYNC_EXECUTOR_AUTO_ENABLED`, default true) are runtime settings (Settings page, `GET`/`PATCH /api/settings`); an env var set at startup wins until the next restart, otherwise the value lives in the `app_settings` table. The scheduler ticks on **`SYNC_ANALYSIS_CRON_SPEC`** (default every minute): due mappings (`next_run_at`) are analysed (YouTube probed for 1 unit, re-read only on change), then the queue is drained across mappings within the YouTube quota budget (`internal/quota`). "Sync now" (`POST /api/collections/mappings/records/:id/sync`) runs one mapping on demand. See the README sections "How sync works" and "YouTube quota and schedules", and RFC-013.

**Database:** `backend/data/spotube.db` holds the OAuth tokens and mappings. Never delete or reset it. `make backend/db/up` / `down` back it up to `backend/data/backups/` before changing the schema; `make backend/db/backup` makes a copy on demand.

**Live testing and quota:** Real YouTube calls spend the shared 10,000-unit daily quota. Prefer reading (`GET /api/quota`, probes cost 1 unit) over actions that search or insert, and don't click Create/Delete in flows that would change the user's playlists unless asked.

**Hosts:** Use **`127.0.0.1`** everywhere in dev — Spotify rejects `localhost` redirect URIs. In `backend/.env`: `PUBLIC_URL=http://127.0.0.1:8090`, `FRONTEND_URL=http://127.0.0.1:5173`. In `frontend/.env`: `VITE_API_URL=http://127.0.0.1:8090`. Vite listens on all interfaces (`host: true`).

**OAuth testing:** Real provider logins need the account owner (Spotify emails a one-time code; Google may require a passkey). Drive the browser to the login page, then hand off to the user. Check `oauth_tokens.updated` in SQLite or `GET /api/auth/{spotify,youtube}/playlists` (401 = reconnect needed) to confirm.

**Formatting:** There is no root `make format`. The frontend can use `npm run format` inside `frontend/` when needed; backend uses `make backend/lint` (`go fmt`, `go vet`).

**Node / Go versions:** See `.nvmrc` (Node) and `backend/go.mod` (Go toolchain). Prefer matching those locally and in CI.
