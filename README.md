# Spotube — YouTube Music ⇆ Spotify playlist sync

A small self-hosted app that keeps pairs of Spotify and YouTube playlists in sync. Add a track
to one side and, on the next sync run, it is found and added to the other side.

**Stack:** Go backend (Echo, Goose migrations, Jet codegen, SQLite) and a React 19 SPA
(Vite, TanStack Router/Query, Tailwind v4). No Docker; the database is a local SQLite file.

> Built largely with AI agents. Historical plans live in [`docs/rfcs`](./docs/rfcs); they describe
> intent at the time and are **not** an accurate description of the current code. This README is.

## How sync works

```
cron tick (SYNC_ANALYSIS_CRON_SPEC, default every minute)
  └─ for each mapping whose interval_minutes has elapsed:
       1. fetch both playlists (once)
       2. refresh playlist names; queue a YouTube rename if sync_name and titles differ
       3. diff tracks by normalized title/artist → queue `add` sync_items for anything missing
       4. auto-apply (SYNC_EXECUTOR_AUTO_ENABLED, default on): search the destination service,
          pick the best confident match, add it — up to 25 items per mapping per run
       5. record last_sync_status / last_sync_error on the mapping
```

- **Additive only.** Tracks are added, never removed. A track that was already synced once is
  not re-queued (unique `(mapping, service, operation, track_id)`), so deleting a synced track on
  one side does not bring it back.
- **Matching** (`backend/internal/matching`) compares metadata, never platform IDs. YouTube titles
  are parsed as `Artist - Title`; YouTube Music "`Artist - Topic`" channels supply the artist. Noise
  like `(Official Video)`, `feat.` credits, and accents are ignored. Threshold: 0.85.
- **Playlist names:** Spotify is the source of truth; with `sync_name` on, the YouTube playlist is
  renamed to match.
- **Failures:** a failed run is stamped, so it waits for the mapping's interval before retrying.
  Items that fail are retried automatically up to 3 times; after that use **Execute** in the Sync
  Queue. Expired/revoked OAuth shows as **Reconnect needed** on the Mappings page.
- **YouTube quota:** each YouTube search costs 100 of the default 10,000 daily units, so roughly
  ~90 tracks/day can be added to YouTube. A run stops early when quota is exhausted.
- **Sync now:** the refresh button on the Mappings page runs steps 1–5 for that mapping
  immediately (`POST /api/collections/mappings/records/:id/sync`), even if cron workers are off.

## Quick start

Prerequisites: Go (version in `backend/go.mod`), Node (version in `.nvmrc`), Make.

```bash
make install
cp backend/env.example backend/.env      # then fill in OAuth credentials (below)
echo 'VITE_API_URL=http://127.0.0.1:8090' > frontend/.env
make dev                                 # backend :8090 + frontend :5173, tails dev.log
```

Open http://127.0.0.1:5173, connect Spotify and YouTube on the dashboard, then create a mapping.
Set `SYNC_WORKERS_ENABLED=true` in `backend/.env` to run the scheduled sync.

### Hostnames: use 127.0.0.1

Spotify no longer accepts `localhost` redirect URIs, only loopback IPs. Use `127.0.0.1` for
`PUBLIC_URL` (backend) and for the URL you open the UI at, and keep the frontend's `VITE_API_URL`
on the same host. OAuth state cookies are host-scoped, so mixing `localhost` and `127.0.0.1`
breaks the callback.

## OAuth setup

Credentials can come from `backend/.env` or the setup wizard (`/setup`, stored in the DB; DB wins).

**Spotify** — https://developer.spotify.com/dashboard → create an app → add redirect URI
`http://127.0.0.1:8090/api/auth/spotify/callback`. While the app is in development mode, add
your Spotify account under *User Management* or calls fail with "user is not registered".
Scopes requested: `playlist-read-private playlist-read-collaborative playlist-modify-private playlist-modify-public`.

**Google / YouTube** — https://console.cloud.google.com → enable *YouTube Data API v3* → OAuth
consent screen (External; add yourself as a test user) → Credentials → OAuth client (Web
application) → authorized redirect URI `http://127.0.0.1:8090/api/auth/youtube/callback`.
Scope requested: `https://www.googleapis.com/auth/youtube`.

> Google consent screens in **Testing** status issue refresh tokens that expire after 7 days. For
> unattended syncing, publish the consent screen (it can stay unverified for personal use).

Refreshed access tokens — and Spotify's rotated refresh tokens — are written back to the
`oauth_tokens` table automatically.

## Configuration (`backend/.env`)

| Variable | Default | Purpose |
|----------|---------|---------|
| `PUBLIC_URL` | — | Backend origin used for OAuth callbacks (`http://127.0.0.1:8090`) |
| `FRONTEND_URL` | `http://localhost:5173` | Where OAuth callbacks send the browser afterwards |
| `CORS_ALLOW_ORIGINS` | `FRONTEND_URL` | Comma-separated UI origins |
| `SPOTIFY_CLIENT_ID` / `_SECRET` | — | Spotify app credentials |
| `GOOGLE_CLIENT_ID` / `_SECRET` | — | Google OAuth client credentials |
| `SESSION_SECRET` | dev placeholder | Signs OAuth session cookies; set a long random value |
| `SYNC_WORKERS_ENABLED` | `false` | Run the scheduled sync |
| `SYNC_ANALYSIS_CRON_SPEC` | `0 * * * * *` | Scheduler tick (6-field cron); each mapping still honours its own interval |
| `SYNC_EXECUTOR_AUTO_ENABLED` | `true` | Apply queued items automatically after analysis (otherwise use Sync Queue → Execute) |
| `DB_PATH` | `./data/spotube.db` | SQLite file |
| `LOG_LEVEL` | `info` | `debug` logs every match decision |

## Observability

- **Mappings page:** last sync time, status (OK / Failed / Reconnect needed) and error.
- **Sync Queue:** every queued item with status, attempts, and a detail view showing the analysis
  decision (best match + score) and executor run (search query, candidates and scores, outcome).
- **Logs (advanced):** `activity_logs` for analysis runs, executor runs, and scheduler events.
- **`dev.log`:** structured backend logs (`make dev` / `make backend/dev`).

## Commands

| Command | What it does |
|---------|--------------|
| `make dev` / `make dev-stop` | Start/stop backend + frontend in the background |
| `make backend/dev` | Backend in the foreground (migrations + Jet codegen first) |
| `make frontend/dev` | Vite dev server |
| `make test` | Backend (`go test`) + frontend (Vitest) tests |
| `make lint` | `go fmt`/`go vet` + ESLint |
| `make build` | Production builds |
| `make backend/db/create NAME=x` | New Goose migration |
| `make backend/db/gen` | Apply migrations and regenerate Jet models in `backend/internal/db` |

Tip: run backend tests with `LOG_LEVEL= make backend/test` so `backend/.env` doesn't leak into
config tests.

## Layout

| Path | Purpose |
|------|---------|
| `backend/cmd/server` | Server entrypoint and wiring |
| `backend/internal/jobs` | Scheduler, analysis (diff + queue), executor (apply items) |
| `backend/internal/matching` | Cross-platform track matching |
| `backend/internal/auth` | OAuth credentials, token storage, API client factory |
| `backend/internal/handlers` | HTTP API |
| `backend/migrations` | SQL migrations (Goose) |
| `frontend/src/routes` | Pages (TanStack file routes) |
| `frontend/src/lib/api` | API client |

## Known gaps

- Removals are not propagated (by design for now).
- The dashboard "YouTube Quota" tile is a placeholder (always 0).
- Single user: no app login; anyone who can reach the server can use the connected accounts.
  Run it on localhost or behind your own auth.
