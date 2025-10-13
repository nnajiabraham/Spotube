# RFC-100: Spotube Backend Migration from PocketBase to Echo

**Status:** Draft
**Branch:** `rfc/100-backend-migration-echo`
**Related Docs:** 
- [Spotube Current State Discovery](../spotube_current_state_discovery_opus.md)
- [EBJoy Migration Reference](../ebjoy_migration_reference_opus.md)

## 1. Goal

Migrate Spotube's backend from PocketBase to Echo framework while:
- Preserving all existing functionality (OAuth, sync jobs, API endpoints)
- Maintaining frontend compatibility with minimal changes
- Improving maintainability with explicit control over the stack
- Following the proven patterns from EBJoy's successful migration

## 2. Background & Context

Spotube currently uses PocketBase as its backend framework, which provides:
- Built-in SQLite database with admin UI
- Authentication and session management
- File storage and serving
- Collection-based APIs with access rules
- Job scheduling via cron

While PocketBase works well, migrating to Echo provides:
- **Explicit control** over authentication, database, and API design
- **Better testability** with standard Go testing patterns
- **Type safety** with Jet SQL generation
- **Flexibility** for future features and scaling
- **Alignment** with EBJoy's tech stack for consistency

This RFC outlines a phased migration approach based on EBJoy's successful migration (RFC-020 through RFC-029).

## 3. Technical Design

### 3.1 Technology Stack

| Component | Current (PocketBase) | Target (Echo) |
|-----------|---------------------|---------------|
| Web Framework | PocketBase | Echo v4 |
| Database | SQLite (PB managed) | SQLite (modernc.org/sqlite) |
| Migrations | PB migrations (Go) | Goose v3 (SQL) |
| Query Builder | PocketBase DAO | Jet v2 |
| Authentication | PB Auth | gorilla/sessions + bcrypt |
| Job Scheduler | PB Cron | robfig/cron |
| OAuth | Custom PB routes | Echo handlers + oauth2 |
| File Storage | PB managed | Custom filesystem |
| Logging | PB + custom | Zerolog |
| Config | godotenv | godotenv + typed config |

### 3.2 Project Structure

```
Spotube/
├── Makefile                    # Root orchestrator
├── backend/
│   ├── Makefile               # Backend-specific targets
│   ├── cmd/
│   │   ├── migrate/           # Migration CLI
│   │   └── server/            # Main server
│   ├── internal/
│   │   ├── config/           # Configuration
│   │   ├── db/               # Jet generated models
│   │   │   ├── model/        # Generated models
│   │   │   └── table/        # Generated tables
│   │   ├── handlers/         # HTTP handlers
│   │   │   ├── auth.go      # Auth endpoints
│   │   │   ├── dashboard.go # Dashboard stats
│   │   │   ├── mappings.go  # Mappings CRUD
│   │   │   ├── blacklist.go # Blacklist management
│   │   │   ├── oauth.go     # OAuth flows
│   │   │   └── setup.go     # Setup wizard
│   │   ├── http/            # Server setup
│   │   ├── jobs/            # Background jobs
│   │   │   ├── analysis.go  # Sync analysis
│   │   │   └── executor.go  # Sync executor
│   │   ├── logging/         # Logger setup
│   │   ├── migrate/         # Migration helpers
│   │   ├── oauth/           # OAuth clients
│   │   │   ├── spotify.go   # Spotify client
│   │   │   └── youtube.go   # YouTube client
│   │   └── sqliteconn/      # SQLite connection
│   └── migrations/          # SQL migrations
└── frontend/
    ├── Makefile            # Frontend-specific targets
    └── src/
        └── lib/
            ├── api/        # HTTP client (replacing PB SDK)
            │   ├── client.ts
            │   ├── auth.ts
            │   ├── mappings.ts
            │   ├── dashboard.ts
            │   └── types.ts
            └── utils/
                └── dates.ts # Timestamp conversion
```

### 3.3 Database Schema (SQL)

```sql
-- oauth_tokens
CREATE TABLE IF NOT EXISTS oauth_tokens (
  id TEXT PRIMARY KEY,
  provider TEXT NOT NULL CHECK (provider IN ('spotify', 'google')),
  access_token TEXT,
  refresh_token TEXT,
  expiry INTEGER,
  scopes TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX idx_oauth_tokens_provider ON oauth_tokens(provider);

-- mappings
CREATE TABLE IF NOT EXISTS mappings (
  id TEXT PRIMARY KEY,
  spotify_playlist_id TEXT NOT NULL,
  youtube_playlist_id TEXT NOT NULL,
  spotify_playlist_name TEXT,
  youtube_playlist_name TEXT,
  sync_name INTEGER NOT NULL DEFAULT 1,
  sync_tracks INTEGER NOT NULL DEFAULT 1,
  interval_minutes INTEGER DEFAULT 60 CHECK (interval_minutes >= 5),
  last_analysis_at INTEGER,
  next_analysis_at INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX idx_mappings_playlists ON mappings(spotify_playlist_id, youtube_playlist_id);

-- sync_items
CREATE TABLE IF NOT EXISTS sync_items (
  id TEXT PRIMARY KEY,
  mapping_id TEXT NOT NULL REFERENCES mappings(id) ON DELETE CASCADE,
  service TEXT NOT NULL CHECK (service IN ('spotify', 'youtube')),
  action TEXT NOT NULL CHECK (action IN ('add_track', 'remove_track', 'rename_playlist')),
  payload TEXT,
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'done', 'error', 'skipped')),
  attempts INTEGER NOT NULL DEFAULT 0,
  last_error TEXT,
  next_attempt_at INTEGER,
  attempt_backoff_secs INTEGER DEFAULT 30,
  completed_at INTEGER,
  source_track_id TEXT,
  source_track_title TEXT,
  source_service TEXT CHECK (source_service IN ('spotify', 'youtube')),
  destination_service TEXT CHECK (destination_service IN ('spotify', 'youtube')),
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX idx_sync_items_mapping_id ON sync_items(mapping_id);
CREATE INDEX idx_sync_items_status ON sync_items(status);
CREATE INDEX idx_sync_items_next_attempt ON sync_items(next_attempt_at);
CREATE UNIQUE INDEX idx_sync_items_unique ON sync_items(mapping_id, service, action, payload);

-- blacklist
CREATE TABLE IF NOT EXISTS blacklist (
  id TEXT PRIMARY KEY,
  mapping_id TEXT REFERENCES mappings(id) ON DELETE CASCADE,
  service TEXT NOT NULL CHECK (service IN ('spotify', 'youtube')),
  track_id TEXT NOT NULL,
  reason TEXT,
  skip_counter INTEGER NOT NULL DEFAULT 0,
  last_skipped_at INTEGER,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE INDEX idx_blacklist_mapping_service ON blacklist(mapping_id, service);
CREATE UNIQUE INDEX idx_blacklist_unique ON blacklist(COALESCE(mapping_id, ''), service, track_id);

-- activity_logs
CREATE TABLE IF NOT EXISTS activity_logs (
  id TEXT PRIMARY KEY,
  level TEXT NOT NULL CHECK (level IN ('info', 'warn', 'error')),
  message TEXT NOT NULL,
  sync_item_id TEXT,
  job_type TEXT CHECK (job_type IN ('analysis', 'execution', 'system')),
  created_at INTEGER NOT NULL
);
CREATE INDEX idx_activity_logs_created ON activity_logs(created_at DESC);
CREATE INDEX idx_activity_logs_job_type ON activity_logs(job_type);

-- settings (for future use)
CREATE TABLE IF NOT EXISTS settings (
  id TEXT PRIMARY KEY,
  key TEXT NOT NULL UNIQUE,
  value TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
```

### 3.4 API Endpoints

#### Authentication (Not needed for current single-user setup)
- Reserved for future multi-user support

#### Dashboard
- `GET /api/dashboard/stats` → Same response shape as current

#### OAuth
- `GET /api/setup/status` → Check setup status
- `GET /api/oauth/spotify/authorize` → Initiate Spotify OAuth
- `GET /api/oauth/spotify/callback` → Spotify OAuth callback
- `GET /api/oauth/google/authorize` → Initiate Google OAuth
- `GET /api/oauth/google/callback` → Google OAuth callback

#### Playlists
- `GET /api/spotify/playlists?limit=50&offset=0` → Fetch Spotify playlists
- `GET /api/youtube/playlists` → Fetch YouTube playlists

#### Mappings
- `GET /api/mappings?page=1&per_page=30&sort=created_at&order=desc` → List mappings
- `GET /api/mappings/:id` → Get single mapping
- `POST /api/mappings` → Create mapping
- `PATCH /api/mappings/:id` → Update mapping
- `DELETE /api/mappings/:id` → Delete mapping

#### Blacklist
- `GET /api/blacklist?mapping_id=xxx&page=1&per_page=30` → List blacklist entries
- `POST /api/blacklist` → Add to blacklist
- `DELETE /api/blacklist/:id` → Remove from blacklist

#### Activity Logs
- `GET /api/activity-logs?page=1&per_page=50&level=error&job_type=analysis` → List logs

#### Sync Items (Read-only)
- `GET /api/sync-items/:id` → Get sync item details

### 3.5 Makefile Structure

Root Makefile:
```makefile
# Include sub-makefiles
-include backend/Makefile
-include frontend/Makefile

.PHONY: help dev test build lint clean install

help: ## Show help
	@echo "Available commands..."

dev: ## Start development servers
	@$(MAKE) -j2 backend/dev frontend/dev

test: ## Run all tests
	@$(MAKE) backend/test
	@$(MAKE) frontend/test

# ... other targets
```

Backend Makefile:
```makefile
.PHONY: backend/dev backend/test backend/build backend/lint
.PHONY: backend/db/create backend/db/up backend/db/down backend/db/gen

backend/dev: ## Start backend with migrations
	@$(MAKE) backend/db/up
	@$(MAKE) backend/db/gen
	@cd backend && go run ./cmd/server/main.go

backend/db/create: ## Create migration NAME=xxx
	@cd backend && goose -dir migrations create $(NAME) sql

backend/db/up: ## Apply migrations
	@cd backend && go run ./cmd/migrate up

backend/db/gen: ## Generate Jet models
	@cd backend && jet -source=sqlite -dsn=file://$(pwd)/$(DB_PATH) -path=internal/db

# ... other targets
```

## 4. Dependencies

### Backend Dependencies (go.mod)
```go
require (
    github.com/labstack/echo/v4 v4.11.4
    github.com/go-jet/jet/v2 v2.11.0
    github.com/pressly/goose/v3 v3.15.1
    modernc.org/sqlite v1.27.0
    github.com/gorilla/sessions v1.2.2
    github.com/joho/godotenv v1.5.1
    github.com/rs/zerolog v1.31.0
    github.com/robfig/cron/v3 v3.0.1
    golang.org/x/oauth2 v0.15.0
    golang.org/x/crypto v0.17.0
    github.com/zmb3/spotify/v2 v2.4.0
    google.golang.org/api v0.154.0
    github.com/google/uuid v1.5.0
)
```

### Frontend Changes
- Remove `pocketbase` package
- Add date utility functions for timestamp conversion
- Update API client to use fetch with session cookies

## 5. Checklist

### Phase 0: Project Setup & Foundation

- [ ] **Task 0.1: Create Makefile structure**
    - **Test Cases**:
        - [ ] Root Makefile includes backend/frontend makefiles
        - [ ] `make help` displays all available commands
        - [ ] `make dev` starts both servers in parallel
        - [ ] Backend runs on port 8091, frontend on port 5173

- [ ] **Task 0.2: Setup Echo server with basic middleware**
    - **Test Cases**:
        - [ ] Server starts on configured port
        - [ ] Request ID middleware adds unique ID to requests
        - [ ] Logger middleware logs requests with latency
        - [ ] Recovery middleware handles panics gracefully
        - [ ] CORS allows frontend origin

- [ ] **Task 0.3: Implement health endpoint**
    - **Test Cases**:
        - [ ] `GET /api/health` returns 200 with correct JSON shape
        - [ ] Response includes status, timestamp, version, service
        - [ ] Database ping is performed and status reflected

- [ ] **Task 0.4: Setup configuration management**
    - **Test Cases**:
        - [ ] Environment variables loaded from .env file
        - [ ] Config struct populated with defaults
        - [ ] Required configs cause startup failure if missing
        - [ ] Test and production configs load correctly

### Phase 1: Database & Migrations

- [ ] **Task 1.1: Setup SQLite connection with PRAGMAs**
    - **Test Cases**:
        - [ ] SQLite opens with WAL mode enabled
        - [ ] Foreign keys enforced
        - [ ] Connection pool limited to 1 for writes
        - [ ] Database file created if not exists

- [ ] **Task 1.2: Integrate Goose for migrations**
    - **Test Cases**:
        - [ ] `make backend/db/create NAME=test` creates new migration
        - [ ] `make backend/db/up` applies all migrations
        - [ ] `make backend/db/down` rolls back one migration
        - [ ] Migration CLI works standalone

- [ ] **Task 1.3: Create initial schema migrations**
    - **Test Cases**:
        - [ ] All tables created with correct columns and types
        - [ ] Indexes created for performance
        - [ ] Foreign key constraints enforced
        - [ ] Check constraints validated
        - [ ] Unique constraints prevent duplicates

- [ ] **Task 1.4: Setup Jet code generation**
    - **Test Cases**:
        - [ ] `make backend/db/gen` generates Go models
        - [ ] Generated code compiles without errors
        - [ ] Models match database schema
        - [ ] Type-safe queries work correctly

### Phase 2: OAuth Implementation

- [ ] **Task 2.1: Implement OAuth token storage**
    - **Test Cases**:
        - [ ] Tokens saved to oauth_tokens table
        - [ ] Existing tokens updated on refresh
        - [ ] Provider uniqueness enforced
        - [ ] Token retrieval by provider works

- [ ] **Task 2.2: Implement Spotify OAuth flow**
    - **Test Cases**:
        - [ ] `/api/oauth/spotify/authorize` redirects to Spotify
        - [ ] Callback exchanges code for token
        - [ ] Tokens stored in database
        - [ ] Error handling for invalid callbacks
        - [ ] State parameter prevents CSRF

- [ ] **Task 2.3: Implement Google/YouTube OAuth flow**
    - **Test Cases**:
        - [ ] `/api/oauth/google/authorize` redirects to Google
        - [ ] Callback handles YouTube scope
        - [ ] Tokens stored in database
        - [ ] Refresh token preserved
        - [ ] Error handling works

- [ ] **Task 2.4: Create OAuth client factories**
    - **Test Cases**:
        - [ ] Spotify client created from stored tokens
        - [ ] YouTube service created from stored tokens
        - [ ] Token refresh handled automatically
        - [ ] Missing tokens return appropriate errors

### Phase 3: Core API Endpoints

- [ ] **Task 3.1: Implement dashboard stats endpoint**
    - **Test Cases**:
        - [ ] Returns correct counts for mappings
        - [ ] Queue statistics accurate
        - [ ] Recent runs pulled from activity logs
        - [ ] YouTube quota calculated correctly
        - [ ] Response shape matches current API

- [ ] **Task 3.2: Implement playlist fetching endpoints**
    - **Test Cases**:
        - [ ] Spotify playlists include all required fields
        - [ ] Pagination works for Spotify
        - [ ] YouTube playlists return all user playlists
        - [ ] OAuth errors handled gracefully
        - [ ] Response shapes match current API

- [ ] **Task 3.3: Implement mappings CRUD**
    - **Test Cases**:
        - [ ] List returns paginated results
        - [ ] Create validates required fields
        - [ ] Update preserves unchanged fields
        - [ ] Delete removes mapping and cascades
        - [ ] Unique playlist pairs enforced
        - [ ] Default values set correctly

- [ ] **Task 3.4: Implement blacklist management**
    - **Test Cases**:
        - [ ] List filters by mapping_id
        - [ ] Global entries (null mapping_id) included
        - [ ] Create prevents duplicate entries
        - [ ] Delete removes entries
        - [ ] Pagination works correctly

- [ ] **Task 3.5: Implement activity logs endpoint**
    - **Test Cases**:
        - [ ] Filtering by level works
        - [ ] Filtering by job_type works
        - [ ] Pagination works correctly
        - [ ] Sort by created_at desc
        - [ ] Response shape matches current

- [ ] **Task 3.6: Implement sync items read endpoint**
    - **Test Cases**:
        - [ ] Get by ID returns correct item
        - [ ] Includes all fields
        - [ ] 404 for non-existent items

### Phase 4: Background Jobs

- [ ] **Task 4.1: Setup cron scheduler**
    - **Test Cases**:
        - [ ] Cron starts with server
        - [ ] Jobs scheduled at correct intervals
        - [ ] Graceful shutdown works
        - [ ] Panics in jobs don't crash server

- [ ] **Task 4.2: Migrate analysis job**
    - **Test Cases**:
        - [ ] Runs every minute
        - [ ] Fetches tracks from both services
        - [ ] Calculates differences correctly
        - [ ] Filters blacklisted tracks
        - [ ] Creates sync items without duplicates
        - [ ] Updates next_analysis_at
        - [ ] Logs activities

- [ ] **Task 4.3: Migrate executor job**
    - **Test Cases**:
        - [ ] Runs every 30 seconds
        - [ ] Picks ready items (next_attempt_at <= now)
        - [ ] Executes add_track actions
        - [ ] Executes rename_playlist actions
        - [ ] Handles errors with backoff
        - [ ] Respects YouTube quota
        - [ ] Updates item status
        - [ ] Logs activities

- [ ] **Task 4.4: Implement activity logger**
    - **Test Cases**:
        - [ ] Creates activity_logs entries
        - [ ] Links to sync_item_id when provided
        - [ ] Correct job_type set
        - [ ] Timestamps recorded

### Phase 5: Frontend Integration

- [ ] **Task 5.1: Create HTTP client to replace PocketBase SDK**
    - **Test Cases**:
        - [ ] Base client handles JSON responses
        - [ ] Errors parsed correctly
        - [ ] Network errors handled
        - [ ] Base URL configurable

- [ ] **Task 5.2: Migrate API calls in frontend**
    - **Test Cases**:
        - [ ] Dashboard stats load correctly
        - [ ] Playlist fetching works
        - [ ] Mappings CRUD operations work
        - [ ] Blacklist management works
        - [ ] Activity logs display
        - [ ] All type definitions updated

- [ ] **Task 5.3: Update frontend routing and guards**
    - **Test Cases**:
        - [ ] Setup wizard route works
        - [ ] Main app routes work
        - [ ] Navigation works correctly
        - [ ] Error boundaries handle API errors

- [ ] **Task 5.4: Update tests to mock new API**
    - **Test Cases**:
        - [ ] Unit tests mock HTTP client
        - [ ] Integration tests use MSW
        - [ ] All existing tests pass
        - [ ] No PocketBase imports remain

### Phase 6: Testing & Polish

- [ ] **Task 6.1: Write comprehensive backend tests**
    - **Test Cases**:
        - [ ] Handler tests cover all endpoints
        - [ ] Job tests verify business logic
        - [ ] OAuth tests use mocks
        - [ ] Database tests use temp files
        - [ ] Test coverage > 80%

- [ ] **Task 6.2: Ensure frontend tests pass**
    - **Test Cases**:
        - [ ] All unit tests pass
        - [ ] Component tests pass
        - [ ] MSW mocks match new API
        - [ ] No console errors

- [ ] **Task 6.3: Performance and reliability testing**
    - **Test Cases**:
        - [ ] API responses < 200ms for queries
        - [ ] Concurrent job execution works
        - [ ] Database locks handled
        - [ ] Memory usage stable

- [ ] **Task 6.4: Documentation updates**
    - **Test Cases**:
        - [ ] README updated with new setup
        - [ ] API documentation accurate
        - [ ] Environment variables documented
        - [ ] Migration guide created

### Phase 7: Cleanup

- [ ] **Task 7.1: Remove PocketBase code**
    - **Test Cases**:
        - [ ] All PocketBase imports removed
        - [ ] Migration files deleted
        - [ ] Old handlers removed
        - [ ] Dependencies cleaned up

- [ ] **Task 7.2: Remove Air and update Makefile**
    - **Test Cases**:
        - [ ] Air configuration removed
        - [ ] Makefile uses direct go run
        - [ ] Hot reload documentation updated
        - [ ] All make targets work

- [ ] **Task 7.3: Final testing and validation**
    - **Test Cases**:
        - [ ] Full sync flow works end-to-end
        - [ ] All frontend features functional
        - [ ] No regressions from PocketBase
        - [ ] Performance acceptable

## 6. Definition of Done

- All checklist items completed and tested
- Backend serves all required endpoints with Echo
- Frontend works without PocketBase SDK
- Background jobs run on schedule
- OAuth flows work for both providers  
- All tests pass (backend and frontend)
- Development workflow documented
- No PocketBase dependencies remain
- Makefile commands work as specified

## Implementation Notes / Summary

This section will be updated as implementation progresses with:
- Specific file paths created/modified
- Key decisions and deviations
- Issues encountered and solutions
- Performance observations
- Testing results

### References
- EBJoy backend structure: Similar handler patterns, middleware setup
- Timestamp handling: All timestamps as Unix epoch seconds (INTEGER)
- Response shapes: Maintain compatibility with current frontend
- Error handling: Sanitized messages, no internal details exposed
