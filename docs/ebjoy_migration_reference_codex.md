# EBJoy Backend Rewrite Reference – Echo + Goose + Jet Stack

_Last reviewed: 2025-10-12_

## 1. Rewrite Goals and Context
- **Objective:** Replace PocketBase backend with a first-party Echo server using Goose migrations and Jet ORM while maintaining existing frontend contract and guest workflows.
- **Drivers:** Need for tighter control over API surface, consistent query conventions, improved testing harness, and alignment with future roadmap (downloads, moderation, admin tooling).
- **Process:** RFC-driven (RFC-020 discovery through RFC-029 phase wrap-up) with sequential phases focusing on auth, events domain, guest access, entries/files, moderation, FE integration.

## 2. Target Architecture Summary
- **Monolith:** Echo v4 server with explicit route registration and middleware.
- **Storage:** SQLite accessed via Goose migrations + Jet codegen for typed queries.
- **Sessions:** Gorilla sessions for cookie-based auth (AuthBoss integration for signup/login/logout).
- **Observability:** Zerolog structured logging, request ID middleware, sanitised error model.

### 2.1 Backend stack components
- **config:** Centralised config loader reading `.env` (APP_ENV, PORT, DB_PATH, CORS, VERSION, session settings).
- **sqliteconn:** Helper enforcing SQLite pragmas (`_journal_mode=WAL`, `_synchronous=NORMAL`, `_busy_timeout`, `_pragma=foreign_keys(1)`), plus connection pooling (max 1 connection by default).
- **migrate:** Goose wrapper for applying migrations programmatically on startup (`migrate.Up(db, dir)`).
- **handlers:** Modularised packages (auth, events, guest, entries, files, moderation, downloads, health) receiving dependency structs (db, logger, session store, config).
- **http server:** `internal/http/server.go` builds Echo instance with middleware stack and custom error handler.

### 2.2 Database schema management
- **Goose migrations (`backend/migrations/*.sql`):** define tables `users`, `events`, `entries`, `entry_files`, `oauth_tokens`, `activity_logs`, indexes, constraints.
- **Jet code generation (`backend/internal/db`)**: generated models/tables used in handlers and repositories.
- **Migration orchestration:** Make targets `backend/db/up`, `backend/db/down`, `backend/db/gen` ensure migrations run prior to codegen; tests use temporary SQLite DB with Goose up/down per suite.

### 2.3 HTTP server & middleware baseline
- Middleware includes: request ID, logging, CORS (allow list from config), CSRF (with `GET /api/csrf` fallback), recovery, session management.
- Routes grouped by auth requirements: `/api/auth/*`, `/api/events/*` (owner), `/api/guest/*` (password gated), file serving `/files/events/*`, moderation endpoints.
- Error responses conform to `{ "error": { "code": string, "message": string } }` with codes like `bad_request`, `unauthorized`, `conflict`, `internal_error`.

### 2.4 Authentication & sessions
- **AuthBoss** provides signup/login/logout with Gorilla session storage.
- Sessions configured via env (`SESSION_COOKIE_NAME`, `SESSION_TTL_SECONDS`, `SESSION_SECURE`).
- Password hashing configured via `BCRYPT_COST` env variable.
- Protected handlers check session presence via middleware injecting `CurrentUser` context.

## 3. Makefile & Project Workflow Patterns
- **Root Makefile:** aggregator forwarding to `backend/Makefile` and `frontend/Makefile`; provides `dev`, `test`, `build`, `lint`, `install`, `clean`, `help`.
- **Backend Makefile:** tasks for dev (`backend/dev`), tests (`backend/test` via gotestsum), build, lint (`go fmt` + `go vet`), install (`go mod download`), migrations (`backend/db/up|down|create`), Jet codegen.
- **Frontend Makefile:** wrappers for Vite dev, test, build, preview, lint, type-check, clean, install, route generation.
- **Tools Makefile:** optional Node-based utilities.
- **Philosophy:** consistent `make <component>/<task>` naming, avoids ad-hoc commands; emphasises `make install`, `make dev`, `make test`, `make lint` workflow.

## 4. RFC Migration Phases (RFC-020 → RFC-029)
1. **RFC-020 Discovery:** Documented PocketBase state, domain context, schema, behavior – used to plan rewrite.
2. **RFC-021 Design:** Architectural blueprint for Echo + Goose + Jet stack, auth model, error handling, pagination conventions.
3. **RFC-022 Phase 0:** Project bootstrapping (Echo skeleton, config, health endpoint, make targets, SQLite connection helper, base migrations).
4. **RFC-023 Phase 1:** Auth system (signup/login/logout), sessions, password hashing, user table migrations, tests.
5. **RFC-024 Phase 2:** Events domain (CRUD for owners, soft delete, pagination/filtering, Jet queries, Option A query params).
6. **RFC-025 Phase 3:** Guest access (event lookup by link/password, access window enforcement, sanitized errors).
7. **RFC-026 Phase 4:** Entries create + file uploads (multipart handling, storage layout, validation, file serving rules).
8. **RFC-027 Phase 5:** Sync job scaffolding (analysis/executor parity) or equivalent tasks.
9. **RFC-028 Phase 6:** Moderation endpoints (report entry, owner review/toggle).
10. **RFC-029 Phase 7:** Frontend integration polish, error hygiene, query convention enforcement, documentation updates.
- Each RFC includes checklist with tests, Implementation Notes referencing file paths, commands run, and any deviations.

## 5. Testing Strategy & Tooling
- **Backend:**
  - Uses `httptest` with temporary SQLite DB; test harness ensures Goose migrations applied before tests, torn down after.
  - Jet models used directly in tests for DB assertions.
  - Focus on handler tests (auth, events, guest, entries, moderation), migration tests (up/down), auth/session integration.
  - Command: `make backend/test` uses `gotestsum --format testname` for readable output.
- **Frontend:**
  - React tests use Vitest + MSW with new API shapes.
  - E2E via Playwright (only when requested).
- **Validation pipeline:** `make test` runs backend then frontend; lint tasks ensure gofmt/vet and ESLint.

## 6. Lessons Learned / Pitfalls to Avoid
- **SQLite concurrency:** Must enforce WAL mode + `SetMaxOpenConns(1)` initially to avoid locking; Jet queries should use transaction context where needed.
- **Query conventions:** Stick to Option A (`page`, `per_page`, `sort`, `order`) and whitelist fields; returning 422 for invalid combinations proved essential for FE alignment.
- **Error hygiene:** Always sanitize error responses; no DB/stack traces; map to stable codes to keep frontend error boundaries predictable.
- **CSRF handling:** SPAs on different ports needed `/api/csrf` helper and HTTP client that grabs cookie or makes fetch before mutations.
- **Jet codegen sequencing:** Always run migrations before codegen; integrate into make targets and CI to prevent stale models.
- **Testing reliability:** Temporary DB per test suite avoids cross-test contamination; avoid shared global state.
- **File uploads:** Validate MIME, size, and number of files; ensure storage paths match expected layout for downloads.

## 7. Reusable Components/Patterns for Spotube
- Adopt `sqliteconn.OpenWithPragmas` equivalent for Spotube to configure SQLite behavior consistently.
- Mirror Makefile structure (root aggregator + backend/frontend sub-makefiles) to provide `install`, `dev`, `test`, `lint`, `build`, `clean` commands.
- Reuse middleware stack (request ID, logging, CORS, CSRF, recovery) and sanitised error helper.
- Use Goose migrations + Jet for Spotube schema (collections: settings, oauth_tokens, mappings, sync_items, blacklist, activity_logs, plus any new user/session tables).
- Apply AuthBoss/Gorilla session pattern if Spotube introduces owner auth; otherwise consider lighter session approach while still leveraging Gorilla sessions and CSRF tokens.
- Use RFC-driven phased approach: plan sequential RFCs replicating EBJoy phases (discovery, design, auth, domain endpoints, jobs, moderation, FE integration).
- Testing harness blueprint: temporary SQLite DB, Goose up/down utilities, Jet models in assertions, domain-specific helper packages.
- HTTP client pattern for frontend: fetch wrapper with `credentials: 'include'`, CSRF token management, Option A query builder, error normalization.
