# Spotube Current State – PocketBase Era

_Last reviewed: 2025-10-12_

## 1. Project Overview
- **Purpose:** Spotube synchronises playlists bidirectionally between Spotify and YouTube Music, providing OAuth setup flows, mapping management, background analysis/execution jobs, and conflict handling.
- **Repo layout:** Monorepo with `backend/` (Go + PocketBase), `frontend/` (React 19 + Vite), shared `docs/` (PRD + RFCs), and root-level Makefile using Air for dev hot reload.
- **PocketBase-centric design:** The backend embeds PocketBase, leveraging its collections, access rules, migrations, and admin UI. Custom Go code registers additional HTTP endpoints and scheduled jobs.

## 2. Backend Architecture
- **Language/tooling:** Go 1.24 using PocketBase v0.21.0 (`github.com/pocketbase/pocketbase`) with migrations compiled into the binary.
- **Key packages:**
  - `cmd/server/main.go` – PocketBase bootstrap, route registration, job schedulers.
  - `internal/pbext/` – custom API surfaces and hooks (`setupwizard`, `spotifyauth`, `googleauth`, `dashboard`, `mappings`).
  - `internal/auth/` – unified OAuth credential/token helpers for Spotify and Google.
  - `internal/jobs/` – cron-based analysis/executor jobs driving playlist sync queue.
  - `internal/activitylogger/` – wraps PocketBase logging into `activity_logs` collection.
  - `internal/testhelpers/` – testing harness built around PocketBase test app utilities.
- **Dependencies:** rely heavily on PocketBase and its DAO; additional libs include `echo/v5` (PocketBase uses Echo internally), `zmb3/spotify`, Google APIs, `samber/lo` for collection utilities.

### 2.1 Server boot
- `main.go` loads `.env`, instantiates PocketBase, optionally enables debug logging via env `LOG_LEVEL=debug`.
- Registers modules: setup wizard routes/hooks, Spotify/Google OAuth routes, dashboard stats endpoint, mapping hooks, job schedulers.
- `migratecmd.MustRegister` attaches PocketBase CLI migrations (`go run ./cmd/server migrate up`).
- Starts PocketBase on default port 8090 (overridden via `PORT`). Dev workflow runs with Air (`go run github.com/air-verse/air@latest`).

### 2.2 Collections, migrations, and data model
- Migrations in `backend/migrations/*.go` define PocketBase collections:
  - `settings` singleton (Spotify/Google client creds).
  - `oauth_tokens` storing provider tokens + expiry + scopes.
  - `mappings` for playlist pairings with sync toggles, intervals, cached names, timestamps.
  - `sync_items` work queue with status fields, payload JSON, relation to mapping, retry info, track metadata.
  - `blacklist` for skipped tracks with reason, counters, per-mapping or global scope.
  - Supporting collections: `activity_logs`, `sync_items` indexes, `analysis` fields, `blacklist` indexes.
- PocketBase collections expose REST endpoints automatically (`/api/collections/<name>`). Access rules enforced via migrations/hooks.

### 2.3 Route surface (PocketBase + custom handlers)
- **Admin UI:** Default PocketBase admin at `/_/` still active.
- **Custom API additions:**
  - `/api/setup/status` (GET) + `/api/setup` (POST) – environment wizard.
  - `/api/auth/spotify/login|callback`, `/api/spotify/playlists` – Spotify OAuth/device playlist proxy.
  - `/api/auth/google/login|callback`, `/api/youtube/playlists` – YouTube OAuth + playlist proxy.
  - `/api/dashboard/stats` – synthesised metrics for mappings, queue, recent runs, quota.
- **PocketBase core endpoints:** Collections (mappings, sync_items, blacklist, activity_logs) accessed via generic REST.
- **No conventional router:** Application logic rides on PocketBase hook system instead of explicit Echo route groups, which complicates transition to standalone Echo server.

### 2.4 Background jobs
- Jobs registered via `pocketbase/tools/cron`:
  - **Analysis (`jobs/analysis.go`):** runs every minute, fetches all mappings, performs diff across Spotify/YouTube tracks, enqueues `sync_items`. Handles blacklist filtering, timestamp scheduling (`last_analysis_at`, `next_analysis_at`).
  - **Executor (`jobs/executor.go`):** runs every minute (intended 5s) with concurrency control. Processes queue items, performs track search, adds tracks, handles retries, quota tracking, blacklist maintenance.
- Jobs depend on PocketBase DAO + OAuth helpers; they assume synchronous access to PocketBase collections.

### 2.5 OAuth and credential storage
- Unified in `internal/auth` per RFC-008b:
  - Credentials retrieved from `settings` collection with env fallback.
  - Tokens persisted in `oauth_tokens` collection; auto-refresh with 30s buffer.
  - Helpers for API handlers (`auth.NewAPIAuthContext`) and jobs (`NewJobAuthContext`).
  - Spotify integration uses `zmb3/spotify` with PKCE; YouTube uses Google API client.
- Credentials loaded via Setup wizard or environment variables (`SPOTIFY_ID`, `SPOTIFY_SECRET`, etc.).

## 3. Frontend Integration
- **Framework:** React 19 + Vite, TanStack Router + Query, Tailwind.
- **API access:** centralised around PocketBase SDK (`src/lib/pocketbase.ts`) and wrapper functions in `src/lib/api.ts`.
- **State management:** React Query caches `mappings`, `dashboard`, `blacklist`, etc. Authentication relies on PocketBase auth session.
- **Routing:**
  - `/setup` wizard interacts with `/api/setup`.
  - `/dashboard` queries `/api/dashboard/stats` and playlist connection status via query params.
  - `/mappings` screens use PocketBase collection endpoints.
  - `/settings/spotify` & `/settings/youtube` drive OAuth login flows by redirecting to PocketBase routes.

### 3.1 API client usage
- `pb.send` for bespoke endpoints (dashboard, playlists, setup), `pb.collection().getList` for data-driven collections.
- Assumes PocketBase-style pagination (`page`, `perPage`, `items`, `totalItems`).
- Error handling expects PocketBase error shapes.

### 3.2 Routing and screens tied to backend APIs
- `routes/dashboard.lazy.tsx` pulls dashboard stats.
- `routes/_authenticated/mappings/*.tsx` perform CRUD via PocketBase SDK.
- `routes/settings/*` open OAuth flows and poll connection status.
- E2E specs rely on PocketBase responses (see `frontend/e2e/*.spec.ts`).

## 4. Tooling & Developer Workflow
- **Root Makefile:**
  - `dev` starts Air-enabled backend and Vite frontend concurrently.
  - `backend-dev`, `backend-workers` (Air with worker config), `migrate-up`, `test`, `lint`, `build-image` (Docker), `clean`, `kill-dev`.
  - Uses inline `go run github.com/air-verse/air@latest` rather than dedicated binary.
- **No sub-makefiles:** Backend/frontend make targets defined in root only; no consistent `install`/`lint` separation akin to EBJoy.
- **Migrations:** `go run ./cmd/server migrate up` via PocketBase plugin rather than Goose CLI.
- **Environment:** `.env` loaded automatically; `PUBLIC_URL`, OAuth secrets used for redirect adjustments.

## 5. Testing Landscape
- **Backend:**
  - Uses `github.com/pocketbase/pocketbase/tests` for in-memory test app.
  - Extensive tests under `internal/auth`, `internal/jobs`, `internal/pbext/*`, verifying OAuth flows, job behavior, dashboard stats.
  - HTTP behaviour tested with Echo contexts and PocketBase test DAO.
- **Frontend:**
  - Vitest + React Testing Library in `src/__tests__/routes`.
  - MSW-based mocks for API interactions use PocketBase-style endpoints.
  - Playwright E2E specs assume PocketBase server accessible at :8090.

## 6. Gaps vs Desired Echo + Goose + Jet Stack
- **Server runtime:** Currently tied to PocketBase lifecycle; lacks standalone Echo server, middleware stack, explicit routing, dependency injection used in EBJoy.
- **Database layer:** PocketBase collections/migrations not directly portable; need Goose SQL migrations and Jet-generated models mirroring existing schema (users/events/entries equivalents must be designed for Spotube’s domain: settings, oauth_tokens, mappings, sync_items, blacklist, activity_logs).
- **Auth/session:** PocketBase handles auth implicitly; Spotube lacks a bespoke owner/guest auth system. New stack must introduce session cookies (likely AuthBoss/Gorilla session analog) or alternative for controlling access to mapping CRUD + job endpoints.
- **File storage:** PocketBase manages uploads; Spotube currently relies on playlist data only (no local file uploads), but any future file needs would require new storage strategy.
- **Jobs:** Cron via PocketBase must be reimplemented (e.g., go-cron or custom scheduler) with transactional DB access via Jet.
- **Tooling parity:** Need Makefile parity with EBJoy: root aggregator + dedicated backend/frontend make targets (`install`, `dev`, `test`, `lint`, `build`, `clean`, etc.) without Air/Docker-specific commands.
- **Frontend client:** All API calls assume PocketBase endpoints and SDK semantics; requires rewriting to HTTP client with Option A query params, new auth flows, and adjusted response shapes.
- **Testing harness:** Backend tests must migrate from PocketBase test app to SQLite test DB with Goose up/down; new test helpers required.

## 7. Open Questions to Resolve During Migration
1. **Authentication model:** Do we introduce owner accounts akin to EBJoy or maintain anonymous access with API keys? (Current app relies on PocketBase admin + tokens stored in collections.)
2. **Schema mapping:** Which PocketBase collections translate to SQL tables, and do we need to normalise any fields (e.g., `payload` JSON -> structured columns)?
3. **Job scheduling:** Reuse EBJoy cron approach (e.g., `github.com/robfig/cron/v3`) or bespoke loop? How to persist job state (`next_attempt_at`) efficiently in SQLite via Jet.
4. **OAuth callback hosting:** With Echo server on :8091 (per EBJoy conventions), confirm PUBLIC_URL/FRONTEND_URL defaults and cookie configuration for CSRF/session interplay.
5. **Activity logging:** Maintain `activity_logs` table with structured logging (zerolog) similar to EBJoy? How to surface metrics to new dashboard endpoint.
6. **Front-end auth state:** Without PocketBase SDK session, how will frontend detect connection status for Spotify/YouTube and secure mapping CRUD? Need explicit endpoints for linking accounts and storing tokens.
7. **Migration strategy for existing data:** Decide whether to provide migration script from PocketBase SQLite to new schema or treat rewrite as clean cut.
8. **Testing parity:** Determine coverage goals for new Echo handlers and job logic; align Vitest/MSW mocks with new API response conventions.
