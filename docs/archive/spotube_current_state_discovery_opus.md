# Spotube Current State Discovery & Context

## 1. Overview

**What Spotube is:** A music playlist synchronization application that bidirectionally syncs playlists between Spotify and YouTube Music services.

**Architecture:**
- **Backend:** Go application using PocketBase framework
- **Frontend:** React/TypeScript SPA with Vite
- **Database:** PocketBase (SQLite-based with admin interface at /_/)
- **Development:** Uses Air for hot reload and Make for orchestration

## 2. Current Data Model (PocketBase Collections)

### 2.1 settings
- System-wide settings collection
- Singleton pattern for global configuration

### 2.2 oauth_tokens
```go
{
  provider: select ["spotify", "google"],
  access_token: text,
  refresh_token: text,
  expiry: date,
  scopes: text
}
```
- Stores OAuth tokens for Spotify and Google/YouTube services
- Currently single-user setup (future: add user relation)

### 2.3 mappings
```go
{
  spotify_playlist_id: text (required),
  youtube_playlist_id: text (required),
  spotify_playlist_name: text,
  youtube_playlist_name: text,
  sync_name: bool,
  sync_tracks: bool,
  interval_minutes: number (min: 5),
  // Analysis fields
  last_analysis_at: date,
  next_analysis_at: date
}
```
- Core entity linking Spotify and YouTube playlists
- Unique composite index on (spotify_playlist_id, youtube_playlist_id)
- Access rules: authenticated users only (currently disabled for single-user)

### 2.4 sync_items
```go
{
  mapping_id: relation->mappings (required),
  service: select ["spotify", "youtube"],
  action: select ["add_track", "remove_track", "rename_playlist"],
  payload: json,
  status: select ["pending", "running", "done", "error", "skipped"],
  attempts: number (min: 0),
  last_error: text,
  // Executor fields
  next_attempt_at: date,
  attempt_backoff_secs: number,
  completed_at: date,
  // Track details (BF3)
  source_track_id: text,
  source_track_title: text,
  source_service: select ["spotify", "youtube"],
  destination_service: select ["spotify", "youtube"]
}
```
- Work queue for synchronization tasks
- Multiple indexes for efficient querying
- Unique composite index prevents duplicate pending items

### 2.5 blacklist
```go
{
  mapping_id: relation->mappings,
  service: select ["spotify", "youtube"],
  track_id: text (required),
  reason: text,
  skip_counter: number (default: 0),
  last_skipped_at: date
}
```
- Tracks that should be excluded from sync
- Can be mapping-specific or global (null mapping_id)

### 2.6 activity_logs
```go
{
  level: select ["info", "warn", "error"],
  message: text (required),
  sync_item_id: text,
  job_type: select ["analysis", "execution", "system"]
}
```
- System activity and job execution logs
- Used for dashboard recent runs display

## 3. Core Features and Jobs

### 3.1 Analysis Job (jobs/analysis.go)
- Runs every minute via cron
- For each mapping:
  - Fetches tracks from both Spotify and YouTube
  - Performs bidirectional diff
  - Filters out blacklisted tracks
  - Enqueues sync_items for differences
  - Updates next_analysis_at based on interval_minutes
- Deduplication logic prevents duplicate pending items

### 3.2 Executor Job (jobs/executor.go)
- Runs every 30 seconds
- Processes pending sync_items:
  - Picks items ready for execution (next_attempt_at <= now)
  - Executes actions (add_track, remove_track, rename_playlist)
  - Handles retries with exponential backoff
  - Respects YouTube API quota limits
  - Updates item status and logs results

### 3.3 OAuth Authentication
- Spotify OAuth flow (internal/pbext/spotifyauth)
- Google OAuth flow (internal/pbext/googleauth)
- Both use standard OAuth2 flow with PocketBase custom routes
- Tokens stored in oauth_tokens collection

### 3.4 Setup Wizard
- First-run setup flow
- Guides through OAuth setup for both services
- Creates initial admin user

## 4. API Endpoints

### 4.1 PocketBase Standard Collection APIs
- `/api/collections/mappings/*` - CRUD operations
- `/api/collections/blacklist/*` - CRUD operations
- `/api/collections/sync_items/*` - Read operations
- `/api/collections/activity_logs/*` - Read operations

### 4.2 Custom API Routes

#### Dashboard
- `GET /api/dashboard/stats` - Returns statistics:
  ```json
  {
    "mappings": { "total": 5 },
    "queue": {
      "pending": 10,
      "running": 2,
      "errors": 1,
      "skipped": 0,
      "done": 50
    },
    "recent_runs": [...],
    "youtube_quota": { "used": 500, "limit": 10000 }
  }
  ```

#### OAuth
- `GET /api/setup/status` - Check if setup is required
- Spotify OAuth:
  - `GET /api/spotify/authorize` - Initiate OAuth flow
  - `GET /api/spotify/callback` - OAuth callback
  - `GET /api/spotify/playlists` - Fetch user playlists
- Google OAuth:
  - `GET /api/google/authorize` - Initiate OAuth flow
  - `GET /api/google/callback` - OAuth callback
  - `GET /api/youtube/playlists` - Fetch user playlists

## 5. Frontend Architecture

### 5.1 Tech Stack
- React 19 + TypeScript
- Vite for bundling
- TanStack Router (file-based routing)
- TanStack Query for data fetching
- Tailwind CSS for styling
- PocketBase SDK for API calls

### 5.2 Key Routes
- `/` - Dashboard (redirects to mappings)
- `/setup` - Initial setup wizard
- `/mappings` - List of playlist mappings
- `/mappings/new` - Create new mapping
- `/mappings/:id/edit` - Edit mapping
- `/mappings/:id/blacklist` - Manage blacklist
- `/logs` - Activity logs viewer

### 5.3 API Integration
- Uses PocketBase JavaScript SDK
- API client wrapper in `lib/api.ts`
- Type definitions in `lib/pocketbase.ts`
- React Query for caching and state management

## 6. Current Build & Development Process

### 6.1 Makefile Commands
```makefile
# Development
make dev              # Run both backend + frontend
make backend-dev      # Backend only (without Air)
make backend-workers  # Backend with Air hot reload
make frontend-dev     # Frontend only

# Testing
make test            # All tests
make test-backend    # Backend tests only
make test-frontend   # Frontend unit tests
make test-e2e        # E2E tests (requires backend)

# Build & Deploy
make build-image     # Docker image
make migrate-up      # Run migrations
make lint           # Run linters
make clean          # Clean artifacts
```

### 6.2 Environment Variables
Backend (.env):
```
LOG_LEVEL=debug
PUBLIC_URL=http://localhost:8090
SPOTIFY_CLIENT_ID=xxx
SPOTIFY_CLIENT_SECRET=xxx
GOOGLE_CLIENT_ID=xxx
GOOGLE_CLIENT_SECRET=xxx
```

Frontend (.env):
```
VITE_API_URL=http://localhost:8090
```

### 6.3 Development Workflow
1. Backend runs on port 8090 (PocketBase)
2. Frontend runs on port 5173 (Vite)
3. Air provides hot reload for backend development
4. PocketBase Admin UI available at `http://localhost:8090/_/`

## 7. Testing Strategy

### 7.1 Backend Tests
- Unit tests for jobs (analysis, executor)
- Integration tests for OAuth flows
- Mock PocketBase app for testing
- Test helpers for creating test data

### 7.2 Frontend Tests
- Vitest for unit tests
- MSW for mocking API calls
- Playwright for E2E tests
- Component tests with React Testing Library

## 8. Key Implementation Details

### 8.1 Job Scheduling
- Uses PocketBase's built-in cron functionality
- Analysis job: `*/1 * * * *` (every minute)
- Executor job: `*/30 * * * * *` (every 30 seconds)

### 8.2 YouTube Quota Management
- Tracks API usage in memory
- Executor respects quota limits
- Dashboard displays current usage

### 8.3 Duplicate Prevention
- Composite unique index on sync_items
- Application-level deduplication in analysis job
- Checks pending/running items before enqueueing

### 8.4 Error Handling
- Exponential backoff for failed sync items
- Max attempts before marking as error
- Detailed error logging in activity_logs

## 9. Migration Considerations

When migrating from PocketBase to Echo:

### 9.1 Data Model
- Convert PocketBase collections to SQL tables
- Preserve field names and relationships
- Maintain indexes for performance
- Keep timestamp fields as Unix epoch seconds

### 9.2 Authentication
- Replace PocketBase auth with session-based auth
- Migrate password hashes if keeping users
- Implement CSRF protection

### 9.3 API Surface
- Replace collection APIs with REST endpoints
- Maintain same response shapes for frontend
- Keep custom routes with same paths
- Preserve query parameters and filters

### 9.4 Jobs
- Replace PocketBase cron with Go cron library
- Keep job logic mostly intact
- Update database queries to use new ORM/query builder

### 9.5 File Storage
- PocketBase manages uploads in pb_data
- Need to implement file upload/storage
- Maintain same file serving URLs

### 9.6 Frontend Changes
- Replace PocketBase SDK with fetch/axios
- Update type definitions
- Maintain React Query integration
- Keep same route structure

## 10. Dependencies to Replace

### Backend
- pocketbase → echo (web framework)
- PocketBase migrations → goose (migrations)
- PocketBase ORM → jet (SQL query builder)
- PocketBase cron → robfig/cron
- PocketBase auth → gorilla/sessions + bcrypt

### Frontend
- pocketbase (SDK) → custom HTTP client
- Collection APIs → REST endpoints
- Realtime subscriptions → Not used currently

## 11. Features to Preserve

1. **OAuth flows** - Both Spotify and Google
2. **Job system** - Analysis and executor with same timing
3. **Dashboard stats** - Same data structure
4. **Blacklist management** - Per-mapping and global
5. **Activity logging** - For debugging and monitoring
6. **Setup wizard** - First-run experience
7. **Quota tracking** - YouTube API limits
8. **Deduplication** - Prevent duplicate sync items
