# RFC-011: Spotube Backend Rewrite – Echo + Goose + Jet Migration

**Status:** Draft  
**Branch:** `rfc/011-spotube-backend-echo-migration`  
**Related Docs:** [`docs/spotube_current_state_codex.md`](../spotube_current_state_codex.md), [`docs/ebjoy_migration_reference_codex.md`](../ebjoy_migration_reference_codex.md)

## 1. Goal
- Replace PocketBase-based backend with a first-party Echo server leveraging Goose migrations, Jet ORM, and Gorilla sessions, mirroring EBJoy’s architecture while serving Spotube’s domain (Spotify/YouTube sync orchestration).
- Provide parity APIs for the frontend and background jobs, enabling incremental migration without feature regression.
- Establish consistent tooling (Makefile workflow, testing harness) and documentation to support ongoing development.

## 2. Background & Context
- Current backend is tightly coupled to PocketBase collections and hooks (see `spotube_current_state_codex.md`).
- EBJoy successfully completed a similar rewrite (see `ebjoy_migration_reference_codex.md`), offering patterns, library choices, and lessons.
- Frontend depends on PocketBase SDK and specific REST endpoints; these must be re-mapped to new HTTP APIs exposed by the Echo server.
- Goal is local functionality (build, migrate, run tests), no deployment/Docker scope.

## 3. Technical Design

### 3.1 Backend Architecture
- **Framework:** Echo v4 (matching EBJoy) with middleware for logging, request IDs, CORS, CSRF, recovery.
- **Database:** SQLite with Goose migrations and Jet codegen.
- **Config:** `.env` with keys analogous to EBJoy (`APP_ENV`, `PORT`, `DB_PATH`, etc.).
- **Sessions & Auth:** Gorilla sessions + AuthBoss if owner login is introduced; otherwise minimal session for admin/operator endpoints. Evaluate need for dedicated user accounts (current Spotube may rely on PocketBase admin). Document decision in Implementation Notes.
- **Modules:**
  - `cmd/server/main.go`: wire config, DB, migrations, Jet, HTTP server.
  - `internal/config`, `internal/sqliteconn`, `internal/logging`, `internal/http` similar to EBJoy.
  - Feature packages: `internal/handlers/auth`, `.../spotify`, `.../youtube`, `.../mappings`, `.../sync`, `.../blacklist`, `.../dashboard`, `.../jobs`.
  - `internal/jobs`: replicate analysis/executor logic using Jet + sql transactions.
  - `internal/oauth`: persist tokens, credential loading (reuse patterns from legacy `internal/auth`).
- **Routes:** Provide REST endpoints analogous to PocketBase usage (documented below).

### 3.2 Data Model (Goose + Jet)
- Tables to reproduce PocketBase collections:
  - `settings` (singleton, Spotify/Google client IDs/secrets, optional future fields).
  - `oauth_tokens` (provider, access_token, refresh_token, expiry, scopes).
  - `mappings` (id, spotify_playlist_id, youtube_playlist_id, cached names, sync toggles, interval, timestamps, soft delete fields, next/last analysis).
  - `sync_items` (id, mapping_id FK, service, action, status, attempts, payload JSON, attempt_backoff_secs, next_attempt_at, source_track metadata, last_error timestamps).
  - `blacklist` (id, mapping_id FK nullable, service, track_id, reason, skip_counter, last_skipped_at).
  - `activity_logs` (id, job_type, level, message, metadata, created_at).
  - Additional tables as needed for user/session (decision item).
- Jet codegen to produce typed models and query builders.

### 3.3 API Surface & Parity Map
- **Health:** `GET /api/health` with status/version/service.
- **Setup:**
  - `GET /api/setup/status`
  - `POST /api/setup` to store credentials (validate env gating/`UPDATE_ALLOWED`).
- **Spotify OAuth:**
  - `GET /api/auth/spotify/login`
  - `GET /api/auth/spotify/callback`
  - `GET /api/spotify/playlists` (requires session + stored tokens).
- **YouTube OAuth:** similar routes to Spotify (`/api/auth/google/*`, `/api/youtube/playlists`).
- **Mappings:**
  - `GET /api/mappings` (pagination, sorting Option A).
  - `GET /api/mappings/:id`
  - `POST /api/mappings`
  - `PATCH /api/mappings/:id`
  - `DELETE /api/mappings/:id`
- **Queue/Logs:**
  - `GET /api/sync-items` (if needed for admin views) or reuse per mapping endpoints.
  - `GET /api/blacklist` / `DELETE /api/blacklist/:id` for UI parity.
  - `GET /api/dashboard/stats` replicating metrics produced by jobs.
- **Jobs:** Provide CLI/HTTP triggers for debugging (optional) but ensure background cron runs on server start.
- Document any modifications to route shapes; maintain return bodies consistent with frontend expectations (Option A lists: `{ data, meta }`, etc.).

### 3.4 Tooling & Workflow
- Introduce root Makefile akin to EBJoy with aggregated tasks.
- Backend Makefile: `backend/dev`, `backend/test`, `backend/build`, `backend/lint`, `backend/install`, `backend/db/*`.
- Frontend Makefile mirroring EBJoy but tailored to Spotube (no brand-specific tasks).
- Remove Air/Docker-specific commands from default workflow (document optional scripts separately if needed).

### 3.5 Frontend Migration Plan
- Replace PocketBase SDK with custom HTTP client (`src/lib/api/client.ts`) using fetch/axios, `credentials: 'include'`, CSRF retrieval (`/api/csrf` if implemented).
- Update API modules (`src/lib/api.ts`, route loaders/hooks) to new endpoints.
- Ensure date/time fields (timestamps) follow EBJoy conventions (Unix epoch seconds or ISO strings—document chosen format).
- Update MSW mocks/tests to new shapes.
- Ensure OAuth flows redirect to new backend port (likely 8091) and update env `VITE_API_URL` default.

### 3.6 Background Job Port
- Port analysis + executor jobs to standalone modules: use Jet queries, transactions, logging.
- Implement YouTube quota tracker, retry/backoff logic, blacklist updates.
- Provide configuration for cron frequency (analysis every minute; executor every few seconds).
- Add tests verifying job behavior with temporary SQLite DB.

## 4. Dependencies
- Backend libraries (matching EBJoy versions where sensible):
  - `github.com/labstack/echo/v4`
  - `github.com/pressly/goose/v3`
  - `github.com/go-jet/jet/v2`
  - `github.com/rs/zerolog`
  - `github.com/joho/godotenv`
  - `github.com/gorilla/sessions`
  - Optional `github.com/aarondl/authboss/v3` if owner auth required.
  - Reuse Spotify/Google API deps from current Spotube backend.
- Frontend: continue using React 19, TanStack Router/Query, MSW; add fetch client utilities as needed (no major dependency change expected).

## 5. Checklist

### Phase 1 – Foundation & Infrastructure
- [ ] **Task 1: Bootstrap Echo project skeleton**
  - Files: `backend/cmd/server/main.go`, `backend/internal/config`, `backend/internal/logging`, `backend/internal/sqliteconn`, `backend/internal/http/server.go`.
  - Tests:
    - [ ] Unit test for config loader default values and env overrides.
    - [ ] Integration test verifying `/api/health` returns expected payload.
    - [ ] Test ensuring SQLite connection enforces WAL + FK pragmas (e.g., query PRAGMA results).
- [ ] **Task 2: Set up Goose migrations + Jet codegen pipeline**
  - Create initial migrations for core tables (settings, oauth_tokens, mappings, sync_items, blacklist, activity_logs).
  - Implement Jet codegen Make target.
  - Tests:
    - [ ] Migration `up`/`down` tests verifying table creation/destruction.
    - [ ] Jet codegen smoke test (build passes after running `backend/db/gen`).

### Phase 2 – OAuth Credentials & Setup Wizard
- [ ] **Task 3: Implement settings + OAuth token persistence**
  - Handlers for `/api/setup/status`, `/api/setup` storing credentials in DB.
  - DB repository functions for credentials and tokens.
  - Tests:
    - [ ] Unit tests for handlers covering success, missing fields, conflict when updates disallowed.
    - [ ] DB tests ensuring tokens persist/refresh correctly.
- [ ] **Task 4: Port Spotify/YouTube OAuth flows**
  - Implement login/callback routes, token storage, playlist proxy endpoints using Jet/Goose DB.
  - Reuse existing OAuth helper logic with necessary adjustments for new DB layer.
  - Tests:
    - [ ] Handler tests covering OAuth callback success/failure, token storage.
    - [ ] Playlist endpoint tests using mocks for Spotify/YouTube APIs.

### Phase 3 – Mappings & CRUD API
- [ ] **Task 5: Implement owner-facing mappings CRUD**
  - Routes: list/create/get/update/delete with session auth.
  - Option A query params, sorting, pagination responses.
  - Tests:
    - [ ] Handler tests for each route (success + validation errors + auth gating).
    - [ ] DB tests ensuring indexes/constraints enforced (unique playlist pair, interval >= 5).
- [ ] **Task 6: Implement blacklist + activity logs endpoints**
  - Provide endpoints required by dashboard and mapping detail pages.
  - Tests:
    - [ ] Handler tests for listing/deleting blacklist, retrieving activity logs.
    - [ ] Ensure sanitized errors.

### Phase 4 – Background Jobs & Sync Execution
- [ ] **Task 7: Port analysis job**
  - Use Jet queries to fetch mappings, track state, apply blacklist filters, enqueue sync_items.
  - Tests:
    - [ ] Job test using temporary DB verifying queue population, timestamps updated, blacklist filtering works.
- [ ] **Task 8: Port executor job**
  - Implement concurrency limiter, quota tracker, track search, playlist modification via APIs.
  - Maintain retry/backoff semantics.
  - Tests:
    - [ ] Job test verifying processing lifecycle (success, retry, fatal -> blacklist, quota skip).
    - [ ] Integration test with mocked Spotify/YouTube clients ensuring API interactions.

### Phase 5 – Dashboard & Reporting
- [ ] **Task 9: Implement `/api/dashboard/stats` and supporting queries**
  - Compute mapping counts, queue status, recent runs, quota usage.
  - Tests:
    - [ ] Handler test verifying response structure given seeded data.
    - [ ] DB query tests for stats functions.

### Phase 6 – Frontend Migration
- [ ] **Task 10: Introduce HTTP client and update API modules**
  - Replace PocketBase SDK usage with fetch client; update hooks/components to new endpoints.
  - Update MSW mocks and tests accordingly.
  - Tests:
    - [ ] Unit tests for API client error handling, CSRF token retrieval.
    - [ ] Updated React tests verifying screens integrate with new API shapes.
- [ ] **Task 11: Update env configs and dev workflow**
  - Provide `.env` templates (`frontend/.env`, `backend/.env`) pointing to new port.
  - Update docs (README, PRD references) to instruct `make` workflows.
  - Tests:
    - [ ] Ensure `npm run test`/`make frontend/test` and `make backend/test` pass with new client.

### Phase 7 – Final QA & Cleanup
- [ ] **Task 12: Comprehensive test pass & regression checks**
  - Run `make test`, `make lint` for both frontend and backend.
  - Perform manual validation using Playwright (if feasible) hitting major flows (setup, connect Spotify/YouTube, create mapping, view dashboard).
  - Update documentation summarising migration steps, open issues, and follow-ups.

## 6. Definition of Done
- New Echo backend builds and runs locally via `make backend/dev`, applying migrations and generating Jet models automatically.
- Frontend consumes new APIs successfully (setup wizard, OAuth connections, mapping management, dashboard stats, blacklist management, sync status).
- Background jobs run under new stack, verified by tests and manual observation of queue processing.
- All tests (backend + frontend) pass; linting clean; documentation updated.
- Legacy PocketBase-specific tooling/commands removed or clearly deprecated.
- Implementation Notes summarise key changes, file paths, and any deviations from plan, referencing test runs.

## 7. Implementation Notes / Summary
_Add notes here as tasks complete, referencing commands run, files updated, deviations, and test outcomes._
