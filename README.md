# Spotube — YouTube Music ⇆ Spotify playlist sync

A small self-hosted app that keeps pairs of Spotify and YouTube playlists in sync. Add a track
to one side and, on the next sync run, it is found and added to the other side.

**Stack:** Go backend (Echo, Goose migrations, Jet codegen, SQLite) and a React 19 SPA
(Vite, TanStack Router/Query, Tailwind v4). No Docker; the database is a local SQLite file.

> Built largely with AI agents. Historical plans live in [`docs/rfcs`](./docs/rfcs); they describe
> intent at the time and are **not** an accurate description of the current code. This README is.

## How sync works

The full walkthrough (every workflow and page) is in [`docs/app-guide.md`](./docs/app-guide.md).

```
cron tick (SYNC_ANALYSIS_CRON_SPEC, default every minute)
  ├─ if Background sync is on: for each mapping whose next_run_at has come
  │    1. read Spotify in full (free); probe YouTube (1 unit) and read it in full only if its
  │       video count changed, there is no cached read, or the cache is stale
  │    2. refresh playlist names; queue a YouTube rename if sync_name and titles differ
  │    3. for each track: check remembered track links first, then fuzzy-match the other
  │       playlist; queue an `add` sync_item for anything missing
  │    4. record last_sync_status / last_sync_error and the next run time
  └─ if Apply automatically is on: drain the queue across all mappings (renames, then songs
     already matched, then the rest, taking turns between mappings), up to 100 items a tick,
     while the YouTube quota budget allows
```

Both toggles are settings (Settings page; `.env` sets the startup value, see
[Settings](#settings)). The full design of quota, schedules and settings is
[RFC-013](./docs/rfcs/RFC-013-quota-scheduling-settings.md).

- **Track links** (`track_links`, `backend/internal/tracklinks`). Every confident pair is
  remembered by ID: a Spotify track and the YouTube video it was matched to, and vice versa.
  Linked tracks skip matching and searching, so a song is copied once and never copied back.
  A link can also have an empty side, meaning "not on that service". Those songs are never
  searched again. Links are global across mappings and can be removed on the **Matches** page.
- **Review.** When a search finds no result scoring 0.85 or higher, the item goes to
  `review` instead of being retried. On the **Review** page you can:
  - pick a candidate;
  - search the destination in-app;
  - paste a link;
  - mark the song "not on {service}";
  - block it for that mapping.

  Choosing a match saves a manual link and adds the track straight away.
- **Additive only.** Tracks are added, never removed. A track that was already synced once is
  not re-queued (unique `(mapping, service, operation, track_id)`), so deleting a synced track on
  one side does not bring it back.
- **Matching** (`backend/internal/matching`):
  - Score = title similarity × 0.6 + artist similarity × 0.4, with adjustments:
    - length difference;
    - version mismatches (live, remix, acoustic…);
    - a bonus for YouTube Music "`Artist - Topic`" channels.
  - YouTube titles are parsed as `Artist - Title`. Noise is ignored: `(Official Video)`,
    `feat.` credits, accents.
  - Threshold: 0.85. Click any score in the UI for its breakdown.
  - Design and alternatives (including embeddings) are in
    [RFC-011](./docs/rfcs/RFC-011-track-matching-review-and-links.md).
- **Playlist names:** Spotify is the source of truth; with `sync_name` on, the YouTube playlist is
  renamed to match.
- **Failures:** a failed run is stamped, so it waits for the mapping's next run. An item whose
  API call fails is tried up to 3 times automatically (15 min, then 1 h apart); after that use
  **Retry** in the Sync Queue. Expired/revoked OAuth shows as **Reconnect needed** on the
  Mappings page. A missing YouTube playlist shows **Playlist missing**; a YouTube playlist at its
  5,000-video limit shows **Playlist full** and only its YouTube adds pause.
- **Pairing:** each playlist can be in one mapping only (enforced by unique indexes; pickers grey
  out paired playlists). If a playlist exists on just one service, Spotube can create the other
  side (`create_playlist: "youtube"` or `"spotify"`; YouTube costs 50 units). **Mirror
  playlists** does this for many playlists at once, each as its own mapping. "Start syncing now"
  runs the first sync straight away (default from the `sync_new_mappings_now` setting).
- **Deleting a mapping** keeps both playlists unless you tick one or both in the dialog
  (Spotify unfollows; YouTube deletes, 50 units).
- **Sync now:** the refresh button on the Mappings page analyses and applies that mapping
  immediately (`POST /api/collections/mappings/records/:id/sync`), even if Background sync is
  off. It re-reads YouTube unless that happened in the last 10 minutes, and doesn't move the
  schedule.

## YouTube quota and schedules

The YouTube Data API allows 10,000 units a day per Google Cloud project (resets at midnight
Pacific). A search costs 100 (+1 for durations), adding a video 50, creating/renaming/deleting a
playlist 50, listing 1 per 50 items. Copying a song Spotube hasn't matched before costs about
151 units, so roughly 60 new songs a day reach YouTube; big copies take days and Spotube plans
for that:

- **One budget for every YouTube call** (`internal/quota`), enforced in the HTTP transport.
  Usage is counted before each request (refunded if it never reached Google or was refused for
  quota) and persisted in `api_usage`. Work has three classes:
  - *interactive* (searches, Sync now's read, creating playlists, restores) may use the limit
    minus 50;
  - *read* (scheduled checks, backups) may use the limit minus the **reserve** kept for you
    (setting, default 500);
  - *drain* (applying the queue) may use the limit minus the reserve minus the scheduled checks
    still expected before the reset, so evening syncs still run.
- **Running out defers, never fails.** Items wait as pending with their attempts unchanged and
  continue after the reset. If Google says the quota is used up, YouTube calls stop and are
  re-tried after 30 minutes (Google's count can differ). Spotify work carries on.
- **Cheap checks.** A scheduled run probes the YouTube playlist (1 unit) and re-reads it only
  when its video count changed (Spotube's own adds don't count), there is no cached read, or the
  cached read is older than a day per 1,000 videos (max a week). Reads are cached in SQLite
  (`playlist_cache`), so a restart costs nothing. Durations are fetched only for unmatched
  videos.
- **Estimates.** Pairing and mirroring show the YouTube units a copy needs and when it will be
  done behind the current queue (`POST /api/quota/estimate`). The Dashboard and Settings show
  today's budget and when the queue finishes (`GET /api/quota`).
- **Schedules.** Each mapping runs every N minutes from a start time on chosen days, in your
  time zone (daily = once at the start time). When pairing, Spotube recommends one: shorter
  intervals for small playlists, longer ones while all mappings' daily checks would exceed 15%
  of the quota, and a start time spaced away from your other mappings. You can override it.
- **Searches in the app** run only on submit, are cached for 30 minutes, and are refused (429,
  with the reset time) when the quota can't cover one.

## Settings

The **Settings** page (`/settings`, API `GET`/`PATCH /api/settings`) changes these without a
restart:

| Setting | Env var | Default |
|---|---|---|
| Background sync | `SYNC_WORKERS_ENABLED` | off |
| Apply changes automatically | `SYNC_EXECUTOR_AUTO_ENABLED` | on |
| Sync new mappings straight away | — | on |
| Tracks applied per minute | — | 100 |
| YouTube daily quota | `YOUTUBE_DAILY_QUOTA` | 10,000 |
| Units kept for you | — | 500 |
| Scheduled backups | `BACKUPS_ENABLED` | on |

If the env var is set when the server starts, that value is used and the page marks it
**.env**; changing it there lasts until the next restart. Otherwise changes are saved in the
database (`app_settings`). OAuth credentials are separate (setup wizard; database wins).

## Playlist backups

Any playlist on either service can be backed up (Backups → **Add playlist**), whether or not it
is in a mapping. Design: [RFC-012](./docs/rfcs/RFC-012-playlist-backups.md).

- A **snapshot** is the playlist's name and ordered tracks, with each track's Spotify and
  YouTube IDs where `track_links` knows both (`playlist_snapshots.tracks_json`).
- **When:** right away, then on each backup's own interval (daily by default; checked every 10
  minutes, independent of `SYNC_WORKERS_ENABLED`), whenever a sync run reads the playlist (free),
  and on **Back up now**.
- **Only changes are stored.** Each check hashes the name and track order. If nothing changed,
  the latest snapshot's `confirmed_at` moves forward instead of adding a row. The newest 30
  snapshots are kept (configurable per backup).
- **Restore** (`POST /api/backups/snapshots/:id/restore`): put missing tracks back into the
  original (never removes), or create a new playlist on the same or the other service from the
  counterpart IDs. YouTube restores are checked against today's remaining quota first.
- **Export:** `GET /api/backups/snapshots/:id/export?format=json|csv`.
- Cost: a YouTube check is about 2 units per 50 videos; Spotify is free.

## Quick start

Prerequisites: Go (version in `backend/go.mod`), Node (version in `.nvmrc`), Make.

```bash
make install
cp backend/env.example backend/.env      # then fill in OAuth credentials (below)
echo 'VITE_API_URL=http://127.0.0.1:8090' > frontend/.env
make dev                                 # backend :8090 + frontend :5173, tails dev.log
```

Open http://127.0.0.1:5173, connect Spotify and YouTube on the dashboard, then create a mapping.
Turn on **Background sync** in Settings (or set `SYNC_WORKERS_ENABLED=true` in `backend/.env`)
to run mappings on their schedules.

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
| `SYNC_WORKERS_ENABLED` | `false` | Startup value of **Background sync** (run mappings on their schedules) |
| `SYNC_ANALYSIS_CRON_SPEC` | `0 * * * * *` | Scheduler tick (6-field cron); each mapping still runs on its own schedule |
| `SYNC_EXECUTOR_AUTO_ENABLED` | `true` | Startup value of **Apply changes automatically** (otherwise use Sync Queue → Execute) |
| `YOUTUBE_DAILY_QUOTA` | `10000` | Startup value of the YouTube daily quota (raise it if Google granted more) |
| `BACKUPS_ENABLED` | `true` | Startup value of **Scheduled backups** (Back up now and sync-time snapshots work regardless) |
| `DB_PATH` | `./data/spotube.db` | SQLite file |
| `LOG_LEVEL` | `info` | `debug` logs every match decision |

Unset toggles fall back to the value saved on the Settings page, then the default.

## Observability

- **Dashboard:** account status, queue counts (including **Needs review**), today's YouTube quota
  budget (used, scheduled checks, kept for you, queued work and when it finishes), paired
  playlists and recent activity.
- **Mappings page:** last sync time, status (OK / Failed / Reconnect needed / Waiting for quota /
  Playlist missing / Playlist full) and error, schedule and next run, track counts per side, and
  queue counts linking to the Queue and Review filtered to that mapping.
- **Review:** tracks Spotube couldn't match confidently, with the closest candidates and actions.
- **Sync Queue:** every queued item with status and attempts. Its detail drawer shows:
  - the analysis decision (best match and score);
  - each executor run (search query, candidates and scores, outcome).
- **Matches:** every remembered pair and "not on {service}" decision, with **Forget**.
- **Backups:** each backed-up playlist's snapshots, what changed between them, and its last
  check or error.
- **Status bar:** API health, queue counts, today's YouTube quota and backups, on every page.
- **Activity:** `activity_logs` for analysis runs, executor runs and scheduler events.
- **Design system:** `/design` shows every UI component (see
  [`docs/design-system.md`](./docs/design-system.md)).
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
| `make backend/db/up` / `down` | Apply / roll back migrations (backs up the database first) |
| `make backend/db/backup` | Copy the database to `backend/data/backups/` |
| `make backend/db/gen` | Apply migrations and regenerate Jet models in `backend/internal/db` |

The database holds your OAuth tokens and mappings. Migrations back it up to
`backend/data/backups/spotube-<time>-pre-up.db` (or `-pre-down`) before changing the schema;
never delete `backend/data/spotube.db` to "reset" things.

Tip: run backend tests with `LOG_LEVEL= make backend/test` so `backend/.env` doesn't leak into
config tests.

## Layout

| Path | Purpose |
|------|---------|
| `backend/cmd/server` | Server entrypoint and wiring |
| `backend/internal/jobs` | Scheduler, analysis (diff + queue), executor (apply items) |
| `backend/internal/matching` | Cross-platform track matching and scoring |
| `backend/internal/tracklinks` | Remembered track pairs and "not on service" decisions |
| `backend/internal/backups` | Playlist snapshots: store, schedule, restore, export |
| `backend/internal/platform` | Playlist/track fetching and link/ID parsing for both services |
| `backend/internal/apiusage` | Measures YouTube quota per API call; the transport that enforces the budget |
| `backend/internal/quota` | YouTube quota budget, work classes, backlog and estimates |
| `backend/internal/schedule` | Mapping schedules: next run, reads per day, recommendations |
| `backend/internal/settings` | Runtime settings (env at startup, else database, else default) |
| `backend/internal/auth` | OAuth credentials, token storage, API client factory |
| `backend/internal/handlers` | HTTP API |
| `backend/migrations` | SQL migrations (Goose) |
| `frontend/src/routes` | Pages (TanStack file routes) |
| `frontend/src/components/ui` | Design-system components ([docs](./docs/design-system.md)) |
| `frontend/src/lib/api` | API client |

## Known gaps

- Removals are not propagated (by design: syncing adds; backups are for history).
- Songs that exist on only one service each cost one YouTube search (101 units) and wait in
  Review; there is no bulk "not available" action yet.
- Spotify ignores the "private" flag when Spotube creates a playlist (on restore), so restored
  Spotify playlists show as public; change it in Spotify if needed.
- Correcting a wrong automatic match (Matches → Forget) does not remove the track that was
  already added; remove it by hand on that service.
- Single user: no app login; anyone who can reach the server can use the connected accounts.
  Run it on localhost or behind your own auth.
