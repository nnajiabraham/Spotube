# EBJoy Migration Reference – PocketBase to Echo Stack

**Document Version:** 1.0  
**Created:** October 12, 2025  
**Purpose:** Document the ebjoy project's migration from PocketBase to Echo + Goose + Jet stack as reference for Spotube migration

---

## 1. Executive Summary

The ebjoy project successfully migrated from PocketBase to a custom Go stack (Echo + Goose + Jet + SQLite) through a well-documented RFC process (RFC-020 through RFC-029). This document captures key patterns, architectural decisions, and lessons learned to guide the Spotube migration.

**Migration Timeline:**
- **RFC-020:** Discovery & context gathering
- **RFC-021:** Technical design & architecture
- **RFC-022–028:** Incremental implementation phases (0–6)
- **RFC-029:** Frontend integration & polish

**Key Outcomes:**
- ✅ Full backend rewrite without breaking frontend during development
- ✅ Improved type safety with Jet's typed SQL
- ✅ SQL-based migrations (more portable than Go code)
- ✅ Strict error hygiene and observability
- ✅ All tests passing (backend + frontend)

---

## 2. Tech Stack Comparison

### 2.1 Stack Transformation Table

| Component | PocketBase (Before) | Echo Migration (After) |
|-----------|---------------------|------------------------|
| **Web Framework** | PocketBase (built-in) | Echo v4 |
| **Database** | SQLite (PocketBase managed) | SQLite (modernc.org/sqlite) |
| **Migrations** | PocketBase migrations (Go) | Goose v3 (SQL) |
| **ORM/Query Builder** | PocketBase DAO | Jet v2 (typed SQL) |
| **Authentication** | PocketBase Auth | gorilla/sessions + bcrypt |
| **Session Store** | PocketBase | Cookie-based (gorilla/sessions) |
| **File Storage** | PocketBase managed | Custom file system implementation |
| **Logging** | PocketBase logs | Zerolog (structured) |
| **Environment** | .env loading | joho/godotenv |

### 2.2 Frontend Stack (Unchanged)
- React 19 + TypeScript + Vite
- TanStack Router & Query
- Tailwind CSS v4
- **Changed:** PocketBase SDK → Custom HTTP client with fetch

---

## 3. Migration Philosophy & Strategy

### 3.1 Core Principles
1. **Incremental delivery:** Small, testable phases (Phase 0–7)
2. **Preserve invariants:** Field names, semantics, access rules
3. **Test first:** Each phase includes specific test cases
4. **Documentation driven:** RFCs guide implementation
5. **No deployment scope:** Focus on local dev + tests

### 3.2 RFC-Driven Workflow
```
1. Discovery (RFC-020)
   └─> Document current state, API surface, collections, behavior

2. Design (RFC-021)
   └─> Define target architecture, conventions, schema, dependencies

3. Phases (RFC-022 to RFC-028)
   └─> Implement incrementally:
       Phase 0: Skeleton + health
       Phase 1: Migrations + codegen
       Phase 2: Auth
       Phase 3: Events (owner CRUD)
       Phase 4: Guest access
       Phase 5: Entries + file uploads
       Phase 6: Moderation flags

4. Integration (RFC-029)
   └─> Frontend migration + API conventions locked + tests green
```

### 3.3 Branch Strategy
- Each RFC can work directly on `main` (no long-lived branches required)
- Git history preserves all prior work
- No need for `old_app/` archive during migration (Git is the archive)

---

## 4. Key Architectural Patterns

### 4.1 Project Structure
```
ebjoy/
├── backend/
│   ├── cmd/
│   │   ├── server/main.go        # Echo server entry point
│   │   └── migrate/main.go       # Goose migration CLI
│   ├── internal/
│   │   ├── config/config.go      # Env-based configuration
│   │   ├── db/                   # Jet-generated models/tables
│   │   │   ├── model/            # Struct models
│   │   │   └── table/            # Table/column references
│   │   ├── handlers/             # HTTP handlers
│   │   │   ├── auth.go
│   │   │   ├── events.go
│   │   │   ├── guest.go
│   │   │   ├── entries.go
│   │   │   └── ...
│   │   ├── http/server.go        # Echo server setup + middleware
│   │   ├── logging/logger.go     # Zerolog setup
│   │   ├── migrate/migrate.go    # Goose wrapper
│   │   └── sqliteconn/           # SQLite connection helper
│   ├── migrations/               # Goose SQL migrations
│   │   └── *.sql
│   ├── Makefile                  # Backend targets
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── lib/api/              # HTTP client + API wrappers
│   │   ├── routes/               # TanStack Router routes
│   │   └── components/
│   ├── Makefile                  # Frontend targets
│   └── package.json
└── Makefile                      # Root orchestration
```

### 4.2 Configuration Pattern
**File:** `backend/internal/config/config.go`

```go
type Config struct {
    AppEnv           string
    Port             string
    DBPath           string
    CORSAllowOrigins []string
    Version          string
    SessionCookieName string
    SessionSecure     bool
    SessionTTLSeconds int
    BcryptCost        int
    CSRFAuthKey       string
    UploadsDir        string
    MigrationsDir     string
    RunMigrationsOnStart bool
}

func Load() Config {
    // Load from env with sensible defaults
    // Compute derived values (e.g., DBPath based on AppEnv)
    return cfg
}
```

**Key Features:**
- Single source of truth for all config
- Environment-aware defaults (`APP_ENV` drives behavior)
- No global state; passed via dependency injection

### 4.3 Server Setup Pattern
**File:** `backend/cmd/server/main.go`

```go
func main() {
    // 1. Load config
    _ = godotenv.Load()
    cfg := config.Load()
    logger := logging.Init(cfg.AppEnv, cfg.Version)

    // 2. Create Echo server with middleware
    e := httpserver.New(httpserver.ServerDeps{Cfg: cfg, Logger: logger})

    // 3. Health endpoint (no DB dependency)
    e.GET("/api/health", handlers.HealthHandler(...))

    // 4. Open DB with pragmas (WAL mode, foreign keys, busy timeout)
    db, err := sqliteconn.OpenWithPragmas(cfg.DBPath)
    // ...

    // 5. Run migrations (if enabled)
    if cfg.RunMigrationsOnStart {
        migrate.Up(db, cfg.MigrationsDir)
    }

    // 6. Session store
    store := sessions.NewCookieStore([]byte(...))
    store.Options = &sessions.Options{
        Path: "/", MaxAge: cfg.SessionTTLSeconds,
        HttpOnly: true, Secure: cfg.SessionSecure,
    }

    // 7. Auth routes
    authDeps := handlers.AuthDeps{DB: db, ...}
    e.POST("/api/auth/signup", handlers.SignupHandler(authDeps))
    e.POST("/api/auth/login", handlers.LoginHandler(authDeps))
    e.POST("/api/auth/logout", handlers.LogoutHandler(authDeps))

    // 8. Domain routes (events, guest, entries, moderation, downloads)
    // ...

    // 9. Graceful shutdown
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()
    go func() { e.Start(addr) }()
    <-ctx.Done()
    e.Shutdown(shutdownCtx)
}
```

**Patterns:**
- **Flat dependency injection:** Each handler receives explicit deps struct
- **No global DB:** Pass `*sql.DB` to handlers
- **Graceful shutdown:** Signal handling for clean termination
- **Middleware applied in server setup:** CORS, CSRF, logging, request ID

### 4.4 Middleware Stack
**File:** `backend/internal/http/server.go`

```go
func New(deps ServerDeps) *echo.Echo {
    e := echo.New()
    e.HideBanner = true
    e.HidePort = true

    // 1. Request ID
    e.Use(middleware.RequestIDWithConfig(...))

    // 2. Access logging (structured)
    e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
        return func(c echo.Context) error {
            start := time.Now()
            err := next(c)
            latency := time.Since(start).Milliseconds()
            logger.Info().
                Str("request_id", c.Response().Header().Get(echo.HeaderXRequestID)).
                Str("method", c.Request().Method).
                Str("path", c.Request().URL.Path).
                Int("status", c.Response().Status).
                Int64("latency_ms", latency).
                Msg("access_log")
            return err
        }
    })

    // 3. Recover (panic → 500)
    e.Use(middleware.Recover())

    // 4. CORS (from config)
    e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
        AllowOrigins: deps.Cfg.CORSAllowOrigins,
        AllowMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
        AllowHeaders: []string{"Content-Type", "Authorization", "X-CSRF-Token"},
        AllowCredentials: true,
    }))

    // 5. CSRF (for cookie auth; skip auth routes)
    e.Use(middleware.CSRFWithConfig(middleware.CSRFConfig{
        TokenLookup: "header:X-CSRF-Token,cookie:ebjoy_csrf",
        CookieName: "ebjoy_csrf",
        CookiePath: "/",
        CookieHTTPOnly: false, // FE needs to read for header
        Skipper: func(c echo.Context) bool {
            return strings.HasPrefix(c.Path(), "/api/auth/")
        },
    }))

    return e
}
```

**Key Middleware:**
- Request ID (tracing)
- Access logging (Zerolog structured)
- Recover (panic safety)
- CORS (SPA support)
- CSRF (state-changing routes)

### 4.5 Handler Pattern
**File:** `backend/internal/handlers/auth.go` (example)

```go
type AuthDeps struct {
    DB *sql.DB
    BcryptCost int
    SessionStore sessions.Store
    SessionName string
    Logger zerolog.Logger
}

func SignupHandler(deps AuthDeps) echo.HandlerFunc {
    return func(c echo.Context) error {
        logger := requestLogger(deps.Logger, c, "Signup")

        // 1. Bind and validate request
        var req signupRequest
        if err := c.Bind(&req); err != nil {
            return c.JSON(422, errorResponse("unprocessable_entity", "validation failed"))
        }

        // 2. Business logic (bcrypt hash, create user with Jet)
        hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), deps.BcryptCost)
        now := int32(time.Now().UTC().Unix())
        user := model.Users{...}
        _, err := table.Users.INSERT(...).MODEL(user).Exec(deps.DB)
        if err != nil {
            logger.Info().Err(err).Msg("signup_conflict")
            return c.JSON(409, errorResponse("conflict", "request conflict"))
        }

        // 3. Return sanitized response
        logger.Info().Str("user_id", id).Msg("signup_success")
        return c.JSON(201, map[string]any{
            "id": id, "email": user.Email, ...
        })
    }
}
```

**Patterns:**
- **Deps struct:** Explicit dependencies injected at registration time
- **Request logger:** Scoped logger with request ID and handler name
- **Sanitized errors:** Never leak SQL errors, paths, or stack traces
- **Structured logging:** All events logged with contextual fields
- **Jet for SQL:** Typed queries with compile-time safety

### 4.6 Error Model
**Consistent error shape across all endpoints:**
```json
{
  "error": {
    "code": "validation_failed | unauthorized | forbidden | not_found | conflict | internal_error",
    "message": "Human-readable message"
  }
}
```

**HTTP Status Codes:**
- `400 Bad Request` – Malformed input
- `401 Unauthorized` – Missing/invalid auth
- `403 Forbidden` – Insufficient permissions
- `404 Not Found` – Resource not found
- `409 Conflict` – Unique constraint violation
- `422 Unprocessable Entity` – Validation failed
- `500 Internal Server Error` – Unexpected errors

**Error Hygiene Rules:**
- ❌ Never include: Stack traces, SQL errors, file paths, secrets
- ✅ Always include: Generic message, appropriate status code
- ✅ Log detailed diagnostics server-side only

---

## 5. Database & Migrations

### 5.1 Goose Migration Pattern
**Directory:** `backend/migrations/`

**Filename Convention:** `YYYYMMDDHHMMSS_description.sql`

**Example:** `20250822012300_init_schema.sql`
```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  first_name TEXT NOT NULL,
  middle_name TEXT NULL,
  last_name TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
```

**Key Conventions:**
- **Timestamps:** INTEGER Unix epoch seconds (UTC)
- **Booleans:** INTEGER 0/1 with `NOT NULL DEFAULT`
- **IDs:** TEXT (UUID v4 as string)
- **Foreign keys:** `REFERENCES table(id)` with `ON DELETE CASCADE` where appropriate
- **Indexes:** Create for all foreign keys and common query fields
- **CHECKs:** Validate enum-like fields (e.g., `media_type IN ('image','video','text')`)

### 5.2 SQLite Pragmas (Production-Ready)
**File:** `backend/internal/sqliteconn/sqliteconn.go`

```go
func OpenWithPragmas(dbPath string) (*sql.DB, error) {
    dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_pragma=foreign_keys(1)", dbPath)
    db, err := sql.Open("sqlite", dsn)
    if err != nil {
        return nil, err
    }
    // Writer concurrency safety (WAL allows 1 writer + N readers)
    db.SetMaxOpenConns(1)
    db.SetMaxIdleConns(1)
    return db, nil
}
```

**Pragmas Explained:**
- `_journal_mode=WAL` – Write-Ahead Logging for better concurrency
- `_synchronous=NORMAL` – Balance durability vs throughput
- `_busy_timeout=5000` – Wait 5s on lock contention
- `_pragma=foreign_keys(1)` – Enforce referential integrity
- `MaxOpenConns=1, MaxIdleConns=1` – Safe writer concurrency under WAL

### 5.3 Jet Codegen
**Command (in Makefile):**
```bash
make backend/db/gen:
	@mkdir -p $(BACKEND_DIR)/internal/db
	@cd $(BACKEND_DIR) && go run github.com/go-jet/jet/v2/cmd/jet@latest \
	  -source=sqlite \
	  -dsn="file://$(shell pwd)/$(BACKEND_DIR)/$(DB_PATH)" \
	  -path internal/db
```

**Generated Structure:**
```
backend/internal/db/
├── model/              # Struct models
│   ├── users.go
│   ├── events.go
│   └── entries.go
└── table/              # Table/column references
    ├── users.go
    ├── events.go
    └── entries.go
```

**Usage in Handler:**
```go
import (
    "github.com/go-jet/jet/v2/sqlite"
    "github.com/manlikeabro/ebjoy/backend/internal/db/model"
    "github.com/manlikeabro/ebjoy/backend/internal/db/table"
)

// Insert
_, err := table.Users.INSERT(
    table.Users.ID,
    table.Users.Email,
    table.Users.PasswordHash,
).MODEL(user).Exec(db)

// Select
var users []model.Users
err := table.Users.SELECT(
    table.Users.AllColumns,
).WHERE(
    table.Users.Email.EQ(sqlite.String(email)),
).Query(db, &users)
```

**Benefits:**
- Compile-time type safety
- Autocomplete for columns
- Refactoring support (IDE can track column renames)
- No string-based SQL in application code

---

## 6. API Conventions

### 6.1 Query Parameters (Option A – Default)
**Pagination:**
- `page=1` (1-indexed)
- `per_page=20` (default: 20, max: 100)

**Sorting:**
- `sort=created_at` (field name)
- `order=desc` (asc | desc, default: desc)

**Filtering:**
- Flat params: `event_id=xyz`, `is_private=false`
- Date ranges: `wedding_date_gte=1234567890`, `wedding_date_lte=1234567890`

**Example:**
```
GET /api/events?sort=created_at&order=desc&page=1&per_page=50
```

**Response Shape (List):**
```json
{
  "data": [...],
  "meta": {
    "page": 1,
    "per_page": 50,
    "total": 123
  }
}
```

**Response Shape (Single):**
```json
{
  "id": "...",
  "name": "...",
  ...
}
```

### 6.2 Timestamp Handling
**Backend:**
- All timestamps stored as INTEGER Unix epoch seconds (UTC)
- All timestamps returned in JSON as integers

**Frontend:**
- Convert Date → epoch seconds on request
- Convert epoch seconds → Date on response
- Display in local timezone using date-fns or similar

**Example Conversion:**
```typescript
// Request: Date → epoch seconds
const weddingDate = Math.floor(new Date('2025-12-25').getTime() / 1000);

// Response: epoch seconds → Date
const weddingDate = new Date(data.wedding_date * 1000);
```

### 6.3 CSRF Token Handling
**Backend:**
- CSRF middleware generates token in cookie `ebjoy_csrf`
- Expects token in header `X-CSRF-Token` for state-changing requests
- Auth routes skipped (no CSRF on login/signup/logout)

**Frontend:**
- `GET /api/csrf` endpoint returns `{ csrf: "..." }` (derive from cookie)
- HTTP client automatically attaches `X-CSRF-Token` header on non-GET requests
- Fallback sequence: cookie → `/api/csrf` endpoint → cache

**Implementation (Frontend):**
```typescript
async function getCSRFToken(): Promise<string> {
  // Try cookie first
  const cookie = document.cookie.match(/ebjoy_csrf=([^;]+)/)?.[1];
  if (cookie) return cookie;

  // Fallback: fetch from endpoint
  const res = await fetch('/api/csrf', { credentials: 'include' });
  const { csrf } = await res.json();
  return csrf;
}

// Attach to requests
const token = await getCSRFToken();
await fetch('/api/events', {
  method: 'POST',
  headers: { 'X-CSRF-Token': token },
  credentials: 'include',
  body: JSON.stringify(data),
});
```

---

## 7. Testing Patterns

### 7.1 Backend Tests (Go + httptest)

**Test Structure:**
```go
func TestCreateEvent(t *testing.T) {
    // 1. Setup: temp SQLite DB + migrations
    db, cleanup := setupTestDB(t)
    defer cleanup()

    // 2. Seed data (if needed)
    seedUser(t, db, testUser)

    // 3. Create Echo app with handler
    e := echo.New()
    deps := handlers.EventsDeps{DB: db, ...}
    e.POST("/api/events", handlers.CreateEventHandler(deps))

    // 4. Make HTTP request via httptest
    body := `{"name":"Test Event", "wedding_date":1234567890, ...}`
    req := httptest.NewRequest("POST", "/api/events", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    rec := httptest.NewRecorder()
    e.ServeHTTP(rec, req)

    // 5. Assert response
    assert.Equal(t, 201, rec.Code)
    var resp map[string]any
    json.Unmarshal(rec.Body.Bytes(), &resp)
    assert.Equal(t, "Test Event", resp["name"])
}
```

**Test Helpers:**
```go
func setupTestDB(t *testing.T) (*sql.DB, func()) {
    // Use unique in-memory DB per test
    dbName := strings.ReplaceAll(t.Name(), "/", "_")
    dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=foreign_keys(1)", dbName)
    db, _ := sql.Open("sqlite", dsn)
    db.SetMaxOpenConns(1)
    db.SetMaxIdleConns(1)

    // Run migrations
    migrate.Up(db, "./migrations")

    cleanup := func() { db.Close() }
    return db, cleanup
}
```

**Key Patterns:**
- **In-memory SQLite:** Fast, isolated tests
- **Migrations in tests:** Validate schema + run Jet against it
- **httptest:** Test full HTTP layer (routing, middleware, handlers)
- **No mocks for DB:** Use real SQLite to catch schema issues

### 7.2 Frontend Tests (Vitest + Custom Mocks)

**Test Structure:**
```typescript
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { EventsList } from './EventsList';

test('renders events list', async () => {
  const queryClient = new QueryClient();
  
  // Mock API client
  vi.spyOn(httpClient, 'get').mockResolvedValue({
    data: [
      { id: '1', name: 'Wedding A', created_at: 1234567890 }
    ],
    meta: { page: 1, per_page: 20, total: 1 }
  });

  render(
    <QueryClientProvider client={queryClient}>
      <EventsList />
    </QueryClientProvider>
  );

  expect(await screen.findByText('Wedding A')).toBeInTheDocument();
});
```

**Key Patterns:**
- **No MSW needed for unit tests:** Mock `httpClient` directly
- **MSW reserved for integration tests:** Full API simulation if needed
- **TanStack Query wrapper:** Wrap components with QueryClientProvider
- **Date mocking:** Use `vi.setSystemTime()` for consistent timestamps

---

## 8. Makefile Structure

### 8.1 Root Makefile
**File:** `/Makefile`

```makefile
.DEFAULT_GOAL := help

help: ## Show all commands
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_\/-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## Install dependencies for both projects
	@$(MAKE) backend/install
	@$(MAKE) frontend/install

dev: ## Start both frontend and backend in development mode
	@$(MAKE) -j2 backend/dev frontend/dev

test: ## Run tests for both projects
	@$(MAKE) backend/test
	@$(MAKE) frontend/test

build: ## Build both projects
	@$(MAKE) backend/build
	@$(MAKE) frontend/build

lint: ## Run linters for both projects
	@$(MAKE) backend/lint
	@$(MAKE) frontend/lint

clean: ## Clean build artifacts
	@$(MAKE) backend/clean
	@$(MAKE) frontend/clean
```

### 8.2 Backend Makefile
**File:** `/backend/Makefile`

```makefile
backend/dev: ## Start Echo backend (db/up & db/gen first)
	@$(MAKE) backend/db/up
	@$(MAKE) backend/db/gen
	@cd $(BACKEND_DIR) && go run ./cmd/server/main.go

backend/test: ## Run backend tests
	@cd $(BACKEND_DIR) && go run gotest.tools/gotestsum@latest --format testname

backend/build: ## Build backend binary
	@cd $(BACKEND_DIR) && go build -o ../dist/$(BINARY_NAME) ./cmd/server/main.go

backend/lint: ## Run Go linters
	@cd $(BACKEND_DIR) && go fmt ./...
	@cd $(BACKEND_DIR) && go vet ./...

backend/db/create: ## Create new Goose migration
	@cd $(BACKEND_DIR) && go run github.com/pressly/goose/v3/cmd/goose@latest -dir ./migrations create $(NAME) sql

backend/db/up: ## Apply pending migrations
	@cd $(BACKEND_DIR) && mkdir -p $(dir $(DB_PATH)) && DB_PATH=$(DB_PATH) go run ./cmd/migrate up

backend/db/down: ## Roll back one migration
	@cd $(BACKEND_DIR) && DB_PATH=$(DB_PATH) go run ./cmd/migrate down

backend/db/gen: ## Run Jet codegen
	@mkdir -p $(BACKEND_DIR)/internal/db
	@cd $(BACKEND_DIR) && go run github.com/go-jet/jet/v2/cmd/jet@latest -source=sqlite -dsn="file://$(DB_PATH)" -path internal/db
```

### 8.3 Frontend Makefile
**File:** `/frontend/Makefile`

```makefile
frontend/dev: ## Start Vite dev server
	@cd $(FRONTEND_DIR) && npm run dev

frontend/test: ## Run frontend tests
	@cd $(FRONTEND_DIR) && npm run test

frontend/build: ## Build frontend for production
	@cd $(FRONTEND_DIR) && npm run build

frontend/lint: ## Run ESLint
	@cd $(FRONTEND_DIR) && npm run lint

frontend/type-check: ## Run TypeScript type checking
	@cd $(FRONTEND_DIR) && npm run type-check
```

**Key Patterns:**
- All targets namespaced (`backend/`, `frontend/`)
- Root `Makefile` orchestrates sub-projects
- Common workflows: `make dev`, `make test`, `make build`
- Database commands integrated into dev workflow

---

## 9. Lessons Learned from EBJoy Migration

### 9.1 What Worked Well

#### Discovery Phase (RFC-020)
✅ **Comprehensive context gathering saved time later**
- Documented all collections, fields, hooks, and access rules
- Captured API surface and response shapes
- Identified invariants that must be preserved for FE compatibility

✅ **PRD served as single source of truth**
- Clear feature definitions
- Non-functional requirements (performance, security)
- Testing strategy aligned with product goals

#### Design Phase (RFC-021)
✅ **Option A (explicit params) was simpler than PocketBase `-created` syntax**
- Easier to document and test
- More intuitive for frontend developers
- No magic syntax or special prefixes

✅ **Epoch timestamps (UTC) eliminated timezone confusion**
- Backend stores integers → simple, portable
- Frontend converts to local display → clear responsibility
- No ambiguity about "what timezone is this?"

✅ **Jet codegen caught schema bugs early**
- Compile errors on missing columns
- IDE autocomplete for SQL builders
- Refactoring support (rename column → update all queries)

#### Implementation Phases
✅ **Small phases prevented scope creep**
- Each phase had clear deliverables and tests
- Easy to review and validate before moving on
- Bugs caught early when surface area was small

✅ **Test-first approach prevented regressions**
- Define test cases in RFC before implementation
- All tests pass before marking phase complete
- Integration tests with real SQLite caught schema issues mocks would miss

✅ **httptest + real SQLite was faster than mocking**
- No mock setup/maintenance
- Validates actual SQL execution
- In-memory DB is fast enough for tests

#### Frontend Integration (RFC-029)
✅ **Thin HTTP client wrapper kept frontend simple**
- Single place to handle credentials, CSRF, errors
- Easy to mock in tests
- No heavy SDK dependency

✅ **CSRF endpoint (`GET /api/csrf`) solved SPA cookie visibility**
- Cross-port dev setup (5173 → 8091) made cookies hard to read
- Endpoint provides fallback without weakening security
- Automatic CSRF handling in HTTP client

### 9.2 What Could Be Improved

#### Discovery Phase
⚠️ **Some edge cases only discovered during implementation**
- Soft delete behavior for related records (cascade?)
- Date-only vs datetime semantics (both stored as epoch seconds)
- File serving access rules required multiple iterations

**Lesson for Spotube:** Budget extra time in discovery for edge cases specific to OAuth flows and job scheduling.

#### Migration Sequencing
⚠️ **AuthBoss added complexity for simple use case**
- ebjoy needed full user auth (signup/login/sessions)
- Spotube is single-user → simpler auth model possible
- Consider if full AuthBoss is needed or if OAuth tokens + session cookies suffice

**Lesson for Spotube:** Re-evaluate auth needs; you may not need bcrypt/password hashing.

#### Background Jobs
⚠️ **PocketBase cron was convenient; replacement not trivial**
- Need to evaluate alternatives: `robfig/cron`, manual goroutines, external scheduler
- Job coordination (analysis vs executor) requires care
- Error handling and retry logic must be explicit

**Lesson for Spotube:** Plan job scheduling strategy early; consider `robfig/cron` or similar.

#### Testing
⚠️ **In-memory SQLite had subtle differences from file-based**
- Foreign key enforcement behavior
- Transaction isolation
- Recommend using unique named in-memory DBs per test (`file:testname?mode=memory&cache=shared`)

**Lesson for Spotube:** Use named in-memory DBs; test with same pragmas as production.

### 9.3 Critical Pitfalls to Avoid

Based on ebjoy's migration experience, watch out for these common issues:

**❌ SQLite Concurrency Issues**
- Must enforce WAL mode + `SetMaxOpenConns(1)` initially to avoid locking
- Jet queries should use transaction context where needed
- Test with concurrent operations to catch race conditions early

**❌ Query Convention Inconsistencies**
- Stick to Option A (`page`, `per_page`, `sort`, `order`) throughout
- Whitelist allowed sort fields; return 422 for invalid combinations
- Inconsistent conventions lead to frontend confusion and bugs

**❌ Error Hygiene Violations**
- Never expose: DB errors, stack traces, file paths, internal details
- Always sanitize error responses with stable error codes
- Map all errors to predictable codes for frontend error boundaries

**❌ CSRF Handling for SPAs**
- SPAs on different ports need `/api/csrf` helper endpoint
- HTTP client must grab cookie or fetch token before mutations
- Don't skip CSRF middleware on state-changing routes

**❌ Jet Codegen Sequencing**
- Always run migrations before codegen to avoid stale models
- Integrate into make targets and CI pipeline
- Stale models cause runtime errors that compile-time checks miss

**❌ Test Isolation Problems**
- Use temporary DB per test suite to avoid cross-test contamination
- Avoid shared global state in tests
- Real SQLite in tests catches issues mocks would miss

**❌ File Upload Security**
- Always validate MIME type, size, and file count
- Ensure storage paths match expected layout for serving
- Never trust client-provided file metadata

### 9.4 Recommendations for Spotube

#### Do Copy These Patterns
✅ Incremental RFC-driven phases  
✅ Epoch timestamps (UTC) for all date/time  
✅ Explicit query params (Option A)  
✅ Sanitized error model (code + message only)  
✅ Jet codegen for type safety  
✅ httptest + real SQLite for tests  
✅ Zerolog structured logging  
✅ Graceful shutdown with signal handling  

#### Adapt These for Spotube Context
🔄 **Auth:** Spotube doesn't need user accounts → simplify or skip AuthBoss  
🔄 **Jobs:** Need alternative to PocketBase cron → evaluate `robfig/cron`  
🔄 **OAuth:** Preserve token refresh logic from ebjoy's unified auth system  
🔄 **Makefile:** Keep similar structure but remove deployment targets (per user request)  

#### Skip or Defer
❌ **Deployment:** User explicitly doesn't want Docker/deployment in Spotube  
❌ **Rate limiting:** Can defer to post-migration if not critical  
❌ **Metrics/observability beyond logging:** Keep it simple for local dev  

---

## 10. Key Files to Reference

When implementing Spotube migration, refer to these ebjoy files for patterns:

### Backend Core
- `backend/cmd/server/main.go` – Server setup, route registration, graceful shutdown
- `backend/internal/config/config.go` – Environment-based configuration
- `backend/internal/http/server.go` – Echo setup + middleware
- `backend/internal/logging/logger.go` – Zerolog initialization

### Database
- `backend/migrations/*.sql` – Goose migration examples
- `backend/internal/migrate/migrate.go` – Goose wrapper
- `backend/internal/sqliteconn/sqliteconn.go` – SQLite with pragmas
- `backend/internal/db/` – Jet-generated models/tables

### Handlers
- `backend/internal/handlers/auth.go` – Auth handler pattern (if needed)
- `backend/internal/handlers/events.go` – CRUD with Jet queries
- `backend/internal/handlers/guest.go` – Query param parsing, pagination
- `backend/internal/handlers/health.go` – Simple health check

### Testing
- `backend/internal/handlers/*_test.go` – httptest + temp DB patterns
- `backend/internal/migrate/*_test.go` – Migration testing

### Frontend Integration
- `frontend/src/lib/api/client.ts` – HTTP client with CSRF handling
- `frontend/src/lib/api/events.ts` – Typed API wrapper example

### Makefiles
- Root `Makefile` – Orchestration
- `backend/Makefile` – Backend targets
- `frontend/Makefile` – Frontend targets

---

## 11. RFC Template for Spotube

Use this structure for each phase RFC:

```markdown
# RFC-XXX: [Phase Name]

**Status:** Draft | Active | Done
**Branch:** `rfc/XXX-[short-name]`
**Related Docs:** [References to discovery docs]

## 1. Goal
*Concrete deliverables for this phase*

## 2. Background & Context
*Why this phase? What does it build on?*

## 3. Technical Design
*Detailed breakdown with file paths, schemas, endpoints*

### 3.1 [Component A]
### 3.2 [Component B]

## 4. Dependencies
*New packages/libs needed*

## 5. Checklist
*Task-by-task breakdown with test cases*

- [ ] Task 1: Description
  - **Test Cases:**
    - [ ] Test case 1a
    - [ ] Test case 1b

## 6. Definition of Done
*Criteria for phase completion*

## Implementation Notes / Summary
*Updated after each task: files changed, commands run, issues encountered*
```

---

## Appendix: Command Quick Reference

### Backend (ebjoy)
```bash
# Development
make backend/dev            # Start server (migrations + codegen + run)
make backend/test           # Run tests
make backend/lint           # Run linters

# Database
make backend/db/create NAME=xyz   # Create migration
make backend/db/up          # Apply migrations
make backend/db/down        # Roll back one migration
make backend/db/gen         # Run Jet codegen

# Build
make backend/build          # Build binary to dist/
make backend/clean          # Clean artifacts
```

### Frontend (ebjoy)
```bash
# Development
make frontend/dev           # Start Vite dev server
make frontend/test          # Run tests
make frontend/lint          # Run linters
make frontend/type-check    # Run TypeScript checker

# Build
make frontend/build         # Build for production
make frontend/preview       # Preview production build
```

### Root (ebjoy)
```bash
make help                   # Show all commands
make install                # Install all dependencies
make dev                    # Start backend + frontend
make test                   # Run all tests
make build                  # Build all
make lint                   # Lint all
make clean                  # Clean all
```

---

**End of Reference Document**

