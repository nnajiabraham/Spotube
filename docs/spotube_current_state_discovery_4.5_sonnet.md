# Spotube Current State Discovery – PocketBase Backend

**Document Version:** 1.0  
**Created:** October 12, 2025  
**Purpose:** Document the current state of the Spotube application before migrating from PocketBase to Echo + Goose + Jet stack

---

## 1. Executive Summary

Spotube is a lightweight self-hosted application that keeps a user's YouTube Music and Spotify playlists in continuous bi-directional sync. The app currently uses:
- **Backend:** Go + PocketBase framework with custom handlers and scheduled jobs
- **Frontend:** React 19 + TypeScript + Vite with TanStack Router/Query and PocketBase JS SDK
- **Database:** SQLite (managed by PocketBase)
- **Development:** Air for live-reload, Makefile for orchestration

The application is single-user (no multi-tenancy), designed to run as a self-hosted service with OAuth integration to both Spotify and YouTube Music.

---

## 2. Product Overview & Core Features

### 2.1 Product Value
- Eliminates manual effort of recreating/updating playlists across Spotify and YouTube Music
- Keeps playlists mirrored automatically with configurable sync intervals
- Controls what syncs (name only or full track list) per mapping
- Avoids vendor lock-in while fully owning data and credentials

### 2.2 Key Features (Implemented)
1. **Environment Setup Wizard** – First-run UI prompts for Spotify/Google credentials; stores in PocketBase `settings` collection
2. **OAuth Integration** – Spotify and YouTube authentication with token refresh
3. **Dashboard** – View system status, mapping counts, queue statistics, YouTube quota
4. **Playlist Mapping** – Create/manage mappings between Spotify ↔ YouTube playlists
5. **Scheduled Sync Jobs** – Analysis job detects changes; executor job applies them with rate limiting
6. **Conflict Handling** – Blacklist system for tracks that fail to sync
7. **Activity Logging** – Detailed sync history visible in dashboard

---

## 3. Current Architecture

### 3.1 Technology Stack

**Backend:**
- Go 1.24.2
- PocketBase 0.21.0 framework
- Libraries:
  - `github.com/zmb3/spotify/v2` – Spotify API client
  - `google.golang.org/api/youtube/v3` – YouTube Data API v3
  - `github.com/samber/lo` – Utility functions
  - `golang.org/x/oauth2` – OAuth2 framework
  - `golang.org/x/sync` – Goroutine primitives
  - PocketBase SDK for database and routing

**Frontend:**
- React 19 + TypeScript
- Vite build tool
- TanStack Router (file-based routing)
- TanStack Query (data fetching/caching)
- Tailwind CSS v4 (styling)
- Zod (validation)
- PocketBase JS SDK (API communication)
- Testing: Vitest + MSW (mocking) + Playwright (E2E)

**Development Tools:**
- Air (live-reload for Go)
- Makefile (orchestration)
- golangci-lint (backend linting)
- ESLint (frontend linting)

### 3.2 System Architecture

**Visual Architecture Diagram:**

```
                    ┌──────────────────────────────────────────────┐
                    │   Frontend (React SPA - Port :5173)         │
                    │                                              │
                    │  • TanStack Router (file-based routing)     │
                    │  • TanStack Query (data fetching/caching)   │
                    │  • PocketBase JS SDK (API communication)    │
                    │  • Tailwind CSS v4 (styling)               │
                    └──────────────┬───────────────────────────────┘
                                   │
                           HTTP + PocketBase SDK
                                   │
                    ┌──────────────▼───────────────────────────────┐
                    │   Backend (Go + PocketBase - Port :8090)    │
                    │                                              │
                    │  ┌─────────────────────────────────────┐   │
                    │  │      PocketBase Core                │   │
                    │  │                                     │   │
                    │  │  • SQLite DB + Collections          │   │
                    │  │  • Auto-generated REST API          │   │
                    │  │  • Admin UI at /_/                  │   │
                    │  │  • Built-in cron scheduler          │   │
                    │  └─────────────────────────────────────┘   │
                    │                                              │
                    │  ┌─────────────────────────────────────┐   │
                    │  │   Custom Go Extensions              │   │
                    │  │                                     │   │
                    │  │  • Setup Wizard (routes + hooks)    │   │
                    │  │  • Spotify OAuth (login/callback)   │   │
                    │  │  • YouTube OAuth (login/callback)   │   │
                    │  │  • Dashboard Stats (aggregation)    │   │
                    │  │  • Mappings Hooks (validation)      │   │
                    │  └─────────────────────────────────────┘   │
                    │                                              │
                    │  ┌─────────────────────────────────────┐   │
                    │  │   Background Jobs (PB Cron)         │   │
                    │  │                                     │   │
                    │  │  • Analysis Job (every 1 minute)    │   │
                    │  │  • Executor Job (every 30 seconds)  │   │
                    │  └─────────────────────────────────────┘   │
                    └──────────────┬───────────────────────────────┘
                                   │
                           OAuth2 + REST APIs
                                   │
                    ┌──────────────▼───────────────────────────────┐
                    │         External Services                    │
                    │                                              │
                    │  • Spotify Web API (OAuth2 + PKCE)          │
                    │  • YouTube Data API v3 (OAuth2)             │
                    └──────────────────────────────────────────────┘
```

**Data Flow:**
1. User interacts with React frontend
2. Frontend makes API calls via PocketBase SDK
3. Backend processes requests (custom routes + collection APIs)
4. Background jobs sync playlists between services
5. External APIs queried for playlist data and modifications

---

## 4. Database Schema (PocketBase Collections)

PocketBase uses a collection-based data model with automatic REST API generation. Current collections:

### 4.1 `settings` (System Collection)
Single-record collection storing OAuth credentials.

**Fields:**
- `spotify_client_id` (text, optional)
- `spotify_client_secret` (text, optional)
- `google_client_id` (text, optional)
- `google_client_secret` (text, optional)

**Purpose:** Store API credentials; fallback to env vars if not set

**Migration:** `1660000000_init_settings_collection.go`, `1660000001_create_settings_singleton.go`

### 4.2 `oauth_tokens` (Base Collection)
Stores OAuth tokens for Spotify and YouTube.

**Fields:**
- `provider` (select: 'spotify' | 'google', required)
- `access_token` (text)
- `refresh_token` (text)
- `expiry` (date)
- `scopes` (text)

**Indexes:** None explicitly (future: unique on provider for single-user)

**Migration:** `1749362310_create_oauth_tokens.go`, `1749396880_oauth_tokens_access_rules.go`

### 4.3 `mappings` (Base Collection)
Defines playlist sync mappings between Spotify and YouTube.

**Fields:**
- `spotify_playlist_id` (text, required)
- `youtube_playlist_id` (text, required)
- `spotify_playlist_name` (text)
- `youtube_playlist_name` (text)
- `sync_name` (bool, default true)
- `sync_tracks` (bool, default true)
- `interval_minutes` (number, default 60)
- `last_analysis_at` (date, optional) – RFC-007
- `tracks_count` (number, default 0) – RFC-007

**Migration:** `1749414389_create_mappings_collection.go`, `1750298769_add_analysis_fields_to_mappings.go`, `1750461462_update_mappings_access_rules.go`

### 4.4 `sync_items` (Base Collection)
Queue of pending sync operations.

**Fields:**
- `mapping_id` (relation → mappings, required)
- `operation` (select: 'add' | 'remove' | 'rename', required)
- `service` (select: 'spotify' | 'youtube', required)
- `track_id` (text)
- `track_title` (text)
- `track_artist` (text)
- `status` (select: 'pending' | 'running' | 'done' | 'error' | 'skipped', default 'pending')
- `error_message` (text)
- `attempt_count` (number, default 0)
- `last_attempt_at` (date)

**Indexes:** Unique composite on `(mapping_id, service, operation, track_id)`

**Migration:** `1750298622_create_sync_items_collection.go`, `1750363691_add_execution_fields_to_sync_items.go`, `1750474958_prevent_duplicate_sync_items.go`, `1750518227_add_track_details_to_sync_items.go`, `1750523308_add_unique_composite_index_to_sync_items.go`

### 4.5 `blacklist` (Base Collection)
Tracks that failed to sync and should be skipped.

**Fields:**
- `mapping_id` (relation → mappings, required)
- `service` (select: 'spotify' | 'youtube', required)
- `track_id` (text, required)
- `reason` (text)
- `skip_counter` (number, default 0)
- `last_skipped_at` (date)

**Migration:** `1750377370_create_blacklist_collection.go`

### 4.6 `activity_logs` (Base Collection)
System activity and sync job events for dashboard display.

**Fields:**
- `level` (select: 'info' | 'warn' | 'error', required)
- `message` (text, required)
- `mapping_id` (text, optional) – reference to mapping
- `job_type` (select: 'analysis' | 'executor' | 'system', required)

**TTL:** Configurable (for log rotation)

**Migration:** `1750550511_create_activity_logs_collection.go`

---

## 5. Backend Components

### 5.1 Entry Point
**File:** `backend/cmd/server/main.go`

**Responsibilities:**
- Load `.env` file
- Initialize PocketBase app
- Register custom routes and hooks
- Register scheduled jobs
- Start server (default :8090)

**Key Initialization Sequence:**
1. Load env vars with `godotenv.Load()`
2. Create PocketBase app: `pocketbase.New()`
3. Register extensions:
   - Setup wizard routes & hooks
   - Spotify OAuth routes
   - YouTube OAuth routes
   - Dashboard routes
   - Mappings hooks
4. Register jobs:
   - Analysis job (every minute)
   - Executor job (continuous)
5. Register PocketBase migration command
6. Start server: `app.Start()`

### 5.2 Custom Routes (API Extensions)

#### 5.2.1 Setup Wizard (`internal/pbext/setupwizard`)
**Routes:**
- `POST /api/setup/save` – Save OAuth credentials to settings collection
- `GET /api/setup/required` – Check if setup is needed

**Hooks:**
- Auto-generate settings singleton if missing

**Purpose:** First-run configuration UI

#### 5.2.2 Spotify OAuth (`internal/pbext/spotifyauth`)
**Routes:**
- `GET /api/auth/spotify/login` – Initiate OAuth flow with PKCE
- `GET /api/auth/spotify/callback` – Handle OAuth callback, store tokens
- `GET /api/spotify/playlists` – List user's Spotify playlists

**Auth Strategy:**
- Loads credentials from settings collection (fallback to env)
- Uses unified auth system from `internal/auth`
- Stores tokens in `oauth_tokens` collection

**OAuth Scopes:**
- `user-read-private`
- `user-read-email`
- `playlist-read-private`
- `playlist-read-collaborative`
- `playlist-modify-public`
- `playlist-modify-private`

#### 5.2.3 YouTube OAuth (`internal/pbext/googleauth`)
**Routes:**
- `GET /api/auth/youtube/login` – Initiate OAuth flow
- `GET /api/auth/youtube/callback` – Handle OAuth callback, store tokens
- `GET /api/youtube/playlists` – List user's YouTube Music playlists

**Auth Strategy:**
- Same pattern as Spotify
- Stores tokens in `oauth_tokens` collection with provider='google'

**OAuth Scopes:**
- `https://www.googleapis.com/auth/youtube`
- `https://www.googleapis.com/auth/youtube.force-ssl`

#### 5.2.4 Dashboard Stats (`internal/pbext/dashboard`)
**Routes:**
- `GET /api/dashboard/stats` – Return system statistics (unauthenticated)

**Response Shape:**
```json
{
  "mappings": { "total": 5 },
  "queue": {
    "pending": 10,
    "running": 2,
    "errors": 1,
    "skipped": 3,
    "done": 45
  },
  "recent_runs": [
    {
      "timestamp": "2024-...",
      "job_type": "analysis",
      "status": "success",
      "message": "Processed 3 mappings"
    }
  ],
  "youtube_quota": {
    "used": 1500,
    "limit": 10000
  }
}
```

**Purpose:** Near real-time dashboard for monitoring sync health

### 5.3 Collection Hooks (`internal/pbext/mappings`)

**Mappings Collection Hooks:**
- On create: Initialize default values for `sync_name`, `sync_tracks`, `interval_minutes`
- On update: Validate playlist IDs
- On delete: Clean up related `sync_items` and `blacklist` entries

### 5.4 Unified OAuth System (`internal/auth`)

**Purpose:** Centralized credential loading and OAuth client creation for both handlers and jobs

**Key Functions:**
- `LoadCredentialsFromSettings(dbProvider, provider)` – Load from settings collection or env
- `GetSpotifyClient(ctx, app)` – Create authenticated Spotify client
- `GetYouTubeService(ctx, app)` – Create authenticated YouTube client

**Features:**
- Automatic token refresh
- Shared between API handlers and background jobs
- Fallback to environment variables if settings not configured

### 5.5 Background Jobs (`internal/jobs`)

#### 5.5.1 Analysis Job (`jobs/analysis.go`)
**Scheduler:** PocketBase cron – runs every minute

**Algorithm:**
```
1. Query all mappings
2. For each mapping that needs analysis (based on interval):
   a. Fetch playlists from Spotify and YouTube
   b. Compare track lists
   c. Detect differences:
      - Tracks in Spotify but not YouTube → queue 'add' to YouTube
      - Tracks in YouTube but not Spotify → queue 'add' to Spotify
      - Playlist name differences → queue 'rename' if sync_name enabled
   d. Create sync_items for each detected change
   e. Update mapping.last_analysis_at
3. Log activity to activity_logs collection
```

**Key Functions:**
- `AnalyseMappings(app, ctx)` – Main entry point
- `analyzeMapping(app, mapping, now)` – Per-mapping analysis
- `shouldAnalyzeMapping(mapping, now)` – Check if interval elapsed

**Dependencies:**
- Spotify API client
- YouTube API client
- Unified OAuth system

#### 5.5.2 Executor Job (`jobs/executor.go`)
**Scheduler:** PocketBase cron – runs continuously (separate config file `.air.workers.toml`)

**Algorithm:**
```
1. Query pending/error sync_items (ordered by created, limited batch size)
2. For each item:
   a. Check blacklist – skip if blacklisted
   b. Determine target service (opposite of item.service)
   c. Execute operation:
      - 'add': Search for track, add to playlist
      - 'remove': Remove track from playlist
      - 'rename': Update playlist title
   d. Handle errors:
      - Rate limit (429): Exponential backoff, requeue
      - Not found: Add to blacklist
      - Other errors: Increment attempt_count, retry with backoff
   e. Update item.status ('done' | 'error' | 'skipped')
   f. Log activity
3. Track YouTube API quota usage
4. Sleep between batches to respect rate limits
```

**Key Functions:**
- `ExecuteSyncItems(app, ctx)` – Main entry point
- `executeItem(app, item)` – Per-item execution
- `handleRateLimit(item, err)` – Exponential backoff logic

**Rate Limiting:**
- YouTube API: Daily quota tracking (10,000 units/day)
- Exponential backoff on 429 errors
- Configurable batch size and sleep intervals

### 5.6 Activity Logger (`internal/activitylogger`)

**Purpose:** Centralized logging to `activity_logs` collection for dashboard display

**Functions:**
- `New(app)` – Create logger instance
- `Record(level, message, mappingID, jobType)` – Write activity log

**Levels:** info, warn, error

---

## 6. Frontend Architecture

### 6.1 Structure
**Directory:** `frontend/src/`

**Key Directories:**
- `routes/` – TanStack Router file-based routes
- `components/` – React components
- `lib/` – Utilities and API clients
- `test/` – Test setup and mocks

### 6.2 Routing (TanStack Router)
**File-based routes:**
- `/` → `routes/dashboard.lazy.tsx` – Main dashboard
- `/setup` → `routes/setup/index.lazy.tsx` – Setup wizard
- `/setup/success` → `routes/setup/success.lazy.tsx` – Setup complete
- `/settings/spotify` → `routes/settings/spotify.lazy.tsx` – Spotify OAuth
- `/settings/youtube` → `routes/settings/youtube.lazy.tsx` – YouTube OAuth
- `/_authenticated/mappings` → `routes/_authenticated/mappings/index.lazy.tsx` – Mappings list
- `/_authenticated/mappings/new` → `routes/_authenticated/mappings/new.lazy.tsx` – Create mapping
- `/_authenticated/mappings/$mappingId/edit` → Edit mapping
- `/_authenticated/mappings/$mappingId/blacklist` → View/manage blacklist
- `/_authenticated/logs` → `routes/_authenticated/logs.lazy.tsx` – Activity logs

### 6.3 API Integration (PocketBase SDK)
**File:** `frontend/src/lib/pocketbase.ts`

**PocketBase Client Setup:**
```typescript
import PocketBase from 'pocketbase';

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8090';
export const pb = new PocketBase(API_BASE_URL);
pb.autoCancellation(false); // Use React Query for cancellation
```

**Type Definitions:**
- `SetupStatus`, `SpotifyPlaylist`, `YouTubePlaylist`, `Mapping`, `BlacklistEntry`
- Response types with pagination: `MappingsResponse`, `BlacklistResponse`

**API Calls:**
- Collections API: `pb.collection('mappings').getList()`
- Custom routes: Direct fetch via `pb.send()` or custom API wrappers

### 6.4 Data Fetching (TanStack Query)
**Patterns:**
- Query keys organized by domain (e.g., `['mappings'], ['mappings', id]`)
- Mutations with optimistic updates and cache invalidation
- Stale time and garbage collection configured per query

**Example:**
```typescript
const { data: mappings } = useQuery({
  queryKey: ['mappings'],
  queryFn: () => pb.collection('mappings').getList(1, 50)
});
```

### 6.5 Components

**Key Components:**
- `DashboardStatsCards.tsx` – Display dashboard metrics
- `SpotifyConnectionCard.tsx` – Spotify OAuth status and connect button
- `YoutubeConnectionCard.tsx` – YouTube OAuth status and connect button
- `SpotifyPlaylists.tsx` – List Spotify playlists for mapping
- `YoutubeLogo.tsx` – Custom YouTube logo component

### 6.6 Testing

**Unit Tests (Vitest + MSW):**
- File: `frontend/src/test/setup.ts`
- Mock handlers: `frontend/src/test/mocks/handlers.ts`
- MSW intercepts PocketBase API calls for isolated testing
- Coverage: Components, routes, hooks

**E2E Tests (Playwright):**
- File: `frontend/e2e/*.spec.ts`
- Tests full user flows: setup → OAuth → mappings → sync
- Requires backend running on :8090

**Test Commands:**
- `npm run test` – Unit tests (watch mode)
- `npm run test:run` – Unit tests (CI mode)
- `npm run test:e2e` – E2E tests

---

## 7. Build System (Makefile)

**Location:** Root `Makefile`

### 7.1 Common Commands

**Development:**
- `make help` – Show available targets
- `make dev` – Start backend (Air) + frontend (Vite) concurrently
- `make backend-dev` – Backend only (PocketBase + Air live reload)
- `make backend-workers` – Backend with continuous jobs (separate Air config)
- `make frontend-dev` – Frontend only (Vite dev server)

**Testing:**
- `make test` – Run backend + frontend tests
- `make test-backend` – Go tests only
- `make test-frontend` – Frontend unit tests only
- `make test-e2e` – Frontend E2E tests (requires backend running)

**Linting:**
- `make lint` – Backend (golangci-lint) + frontend (ESLint)

**Building:**
- `make build-image` – Build Docker image

**Database:**
- `make migrate-up` – Manually run PocketBase migrations

**Cleanup:**
- `make clean` – Remove build artifacts
- `make kill-dev` – Kill dev servers on ports 8090, 5173-5176

### 7.2 Environment Variables

**Backend (.env):**
```bash
# OAuth Credentials (optional - can use setup wizard)
SPOTIFY_CLIENT_ID=...
SPOTIFY_CLIENT_SECRET=...
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...

# Server
PUBLIC_URL=http://127.0.0.1:8090
FRONTEND_URL=http://localhost:5173
PORT=8090

# Logging
LOG_LEVEL=debug

# Development
DEV_MODE=true
```

**Frontend (.env):**
```bash
VITE_API_URL=http://localhost:8090
```

---

## 8. Testing Patterns

### 8.1 Backend Tests (Go)

**Test Files:**
- `internal/activitylogger/activitylogger_test.go`
- `internal/auth/common_test.go`, `spotify_test.go`, `youtube_test.go`, `integration_test.go`
- `internal/jobs/analysis_test.go`, `executor_test.go`
- `internal/pbext/dashboard/routes_test.go`
- `internal/pbext/googleauth/googleauth_test.go`, `oauth_settings_integration_test.go`
- `internal/pbext/spotifyauth/spotifyauth_test.go`, `oauth_settings_integration_test.go`
- `internal/pbext/setupwizard/routes_test.go`
- `internal/pbext/mappings/mappings_test.go`

**Patterns:**
- PocketBase test helpers: `internal/testhelpers/pocketbase.go`
- HTTP mocking: `internal/testhelpers/http_mocking.go` (using `jarcoal/httpmock`)
- Integration tests use temporary PocketBase instances
- Tests validate collection operations, hooks, and API routes

**Test Helpers:**
- `testhelpers.CreateTestApp()` – Create temporary PocketBase app with migrations
- `testhelpers.CleanupTestApp()` – Cleanup test database

### 8.2 Frontend Tests (Vitest + MSW)

**Test Files:**
- `src/components/DashboardStatsCards.test.tsx`
- `src/components/SpotifyConnectionCard.test.tsx`
- `src/components/YoutubeConnectionCard.test.tsx`
- `src/__tests__/routes/_authenticated/logs.test.tsx`
- `src/__tests__/routes/_authenticated/mappings/index.test.tsx`
- `src/__tests__/routes/_authenticated/mappings/$mappingId/blacklist.test.tsx`

**Patterns:**
- MSW handlers mock PocketBase API responses
- React Testing Library for component testing
- TanStack Query wrapper for testing hooks
- Time mocking with Vitest

**Mock Handlers Location:** `src/test/mocks/handlers.ts`

### 8.3 E2E Tests (Playwright)

**Test Files:**
- `e2e/setup.spec.ts` – Setup wizard flow
- `e2e/oauth.spec.ts` – OAuth flows
- `e2e/mappings.spec.ts` – Mapping CRUD
- `e2e/sync.spec.ts` – Full sync cycle

**Patterns:**
- Real backend required (not mocked)
- Tests full user journeys
- Validates UI interactions and data persistence

---

## 9. API Surface

### 9.1 PocketBase Auto-Generated REST API

**Collections accessible via REST:**
- `GET /api/collections/mappings/records` – List mappings
- `POST /api/collections/mappings/records` – Create mapping
- `GET /api/collections/mappings/records/:id` – Get mapping
- `PATCH /api/collections/mappings/records/:id` – Update mapping
- `DELETE /api/collections/mappings/records/:id` – Delete mapping
- Similar patterns for: `oauth_tokens`, `sync_items`, `blacklist`, `activity_logs`, `settings`

**Query Features:**
- Pagination: `?page=1&perPage=50`
- Sorting: `?sort=-created` (PocketBase convention: minus prefix for desc)
- Filtering: `?filter=(field='value')`

### 9.2 Custom Routes

**Setup:**
- `POST /api/setup/save` – Save credentials
- `GET /api/setup/required` – Check setup status

**Spotify:**
- `GET /api/auth/spotify/login` – OAuth login
- `GET /api/auth/spotify/callback` – OAuth callback
- `GET /api/spotify/playlists` – List playlists

**YouTube:**
- `GET /api/auth/youtube/login` – OAuth login
- `GET /api/auth/youtube/callback` – OAuth callback
- `GET /api/youtube/playlists` – List playlists

**Dashboard:**
- `GET /api/dashboard/stats` – System statistics (unauthenticated)

**Admin (PocketBase):**
- `/_/` – PocketBase admin UI (available in dev/prod)

---

## 10. Key Dependencies

### 10.1 Backend
```
github.com/pocketbase/pocketbase v0.21.0
github.com/zmb3/spotify/v2 v2.4.3
google.golang.org/api v0.236.0
github.com/labstack/echo/v5 (via PocketBase)
github.com/samber/lo v1.51.0
github.com/joho/godotenv v1.5.1
golang.org/x/oauth2 v0.30.0
golang.org/x/sync v0.14.0
github.com/stretchr/testify v1.10.0 (testing)
github.com/jarcoal/httpmock v1.4.0 (testing)
```

### 10.2 Frontend
```
react v19.1.0
@tanstack/react-router v1.120.18
@tanstack/react-query v5.80.6
@tanstack/react-table v8.21.3
pocketbase v0.26.1 (JS SDK)
zod v3.25.56
react-hook-form v7.57.0
@hookform/resolvers v5.1.0
lucide-react v0.513.0 (icons)
tailwindcss v4.1.8
vite v6.3.5
vitest v3.2.2 (testing)
msw v2.10.1 (mocking)
@playwright/test v1.52.0 (E2E)
```

---

## 11. Data Flow Examples

### 11.1 OAuth Flow (Spotify)
```
1. User clicks "Connect Spotify" in UI
2. Frontend navigates to: GET /api/auth/spotify/login
3. Backend:
   a. Loads credentials from settings or env
   b. Generates OAuth state + PKCE verifier
   c. Stores in session cookie
   d. Redirects to Spotify authorization URL
4. User authorizes on Spotify
5. Spotify redirects to: GET /api/auth/spotify/callback?code=...&state=...
6. Backend:
   a. Validates state
   b. Exchanges code for tokens using PKCE
   c. Stores tokens in oauth_tokens collection (provider='spotify')
   d. Redirects to frontend success page
7. Frontend displays connection status
```

### 11.2 Sync Flow (Analysis → Execution)
```
Analysis Job (every minute):
1. Query mappings where last_analysis_at is stale
2. For mapping with Spotify playlist A ↔ YouTube playlist B:
   a. Fetch Spotify playlist A tracks
   b. Fetch YouTube playlist B tracks
   c. Compare:
      - Track X in Spotify but not YouTube → Create sync_item: {
          operation: 'add',
          service: 'youtube',
          track_id: X,
          status: 'pending'
        }
      - Track Y in YouTube but not Spotify → Create sync_item: {
          operation: 'add',
          service: 'spotify',
          track_id: Y,
          status: 'pending'
        }
   d. Update mapping.last_analysis_at
3. Log activity to activity_logs

Executor Job (continuous):
1. Query sync_items where status IN ('pending', 'error') LIMIT 10
2. For each item:
   a. Check blacklist – skip if present
   b. If operation='add' AND service='youtube':
      - Search Spotify for track_title + track_artist
      - Add found track to YouTube playlist
      - Update item.status = 'done'
   c. If operation='add' AND service='spotify':
      - Similar for Spotify
   d. On error:
      - If 404: Add to blacklist, set status='skipped'
      - If 429: Exponential backoff, keep status='pending'
      - If other: Increment attempt_count, set status='error'
3. Log activity
4. Sleep 5 seconds, repeat
```

### 11.3 Dashboard Stats Aggregation
```
GET /api/dashboard/stats:
1. Query mappings collection: COUNT(*)
2. Query sync_items grouped by status: COUNT(*) GROUP BY status
3. Query activity_logs: ORDER BY created DESC LIMIT 10
4. Query YouTube quota tracking (custom logic in executor)
5. Return aggregated JSON
```

---

## 12. Gaps and Known Issues

### 12.1 Current Limitations
1. **Single-user only** – No multi-tenancy or user accounts
2. **No authentication** – All endpoints are unauthenticated (single-user assumption)
3. **OAuth tokens not auto-refreshed** – Manual re-auth required on expiry
4. **YouTube quota exhaustion** – No graceful degradation when quota exceeded
5. **No retry limits** – Executor may retry indefinitely on persistent errors
6. **Track matching heuristics** – Simple title+artist matching; no fuzzy search

### 12.2 Technical Debt
1. **Air dependency** – Live-reload requires external tool; not part of standard Go tooling
2. **PocketBase coupling** – Heavy reliance on PocketBase SDK and conventions
3. **Migration format** – Go-based migrations (not SQL) make schema less portable
4. **No typed queries** – Direct PocketBase DAO calls without compile-time safety
5. **Test coverage** – Some job logic has limited test coverage due to PocketBase mocking complexity

### 12.3 Migration Challenges (PocketBase → Echo)
1. **No standard user auth** – Spotube doesn't have user login system like ebjoy
2. **Background jobs** – Need to replace PocketBase cron with alternative scheduler
3. **OAuth state management** – PocketBase session cookies need migration to Echo sessions
4. **Collection API parity** – Frontend expects certain response shapes from PocketBase
5. **Admin UI** – Losing PocketBase admin UI (acceptable for this project)

---

## 13. Open Questions to Resolve During Migration

These strategic decisions will shape the migration approach and should be addressed early in the process:

### 13.1 Authentication & Authorization Model
**Question:** Does Spotube need a formal user authentication system like EBJoy, or can it remain single-user/anonymous?

**Current State:** PocketBase admin + tokens in collections; no user accounts for end users.

**Options:**
- **Option A:** Skip AuthBoss entirely; use lightweight session management for OAuth state only
- **Option B:** Implement minimal bcrypt-based auth for future multi-user support
- **Recommendation:** Start with Option A (lightweight) since Spotube is designed as single-user self-hosted app

**Impact:** Determines whether to implement signup/login handlers and session-based access control

---

### 13.2 Job Scheduling Implementation
**Question:** Which cron library should replace PocketBase's built-in scheduler?

**Current State:** PocketBase cron with `*/1 * * * *` (analysis) and `*/30 * * * * *` (executor) patterns.

**Options:**
- **Option A:** `robfig/cron/v3` (most popular, good API, similar to PocketBase)
- **Option B:** Custom goroutine-based scheduler with time.Ticker
- **Recommendation:** Use `robfig/cron/v3` for familiarity and reliability

**Impact:** Affects job registration, graceful shutdown, and testing patterns

---

### 13.3 Schema Normalization
**Question:** Should we normalize PocketBase collection fields or preserve them exactly?

**Current State:** Some fields stored as JSON (sync_items.payload), mix of naming conventions.

**Options:**
- **Option A:** Preserve exact field names and types for frontend compatibility
- **Option B:** Normalize (e.g., break out payload JSON into structured columns)
- **Recommendation:** Option A initially; normalize in later phase if needed

**Impact:** Frontend migration complexity; backwards compatibility

---

### 13.4 OAuth Callback URLs & Configuration
**Question:** What port and URL structure for OAuth callbacks after migrating to Echo?

**Current State:** Callbacks to `http://localhost:8090/api/auth/spotify/callback` (PocketBase port).

**Options:**
- **Option A:** Keep port 8090 for backward compatibility with existing OAuth app registrations
- **Option B:** Move to port 8091 (EBJoy convention); update OAuth app settings
- **Recommendation:** Option A to avoid re-registering OAuth apps

**Impact:** OAuth app configuration in Spotify/Google consoles; redirect_uri validation

---

### 13.5 Activity Logging Strategy
**Question:** Should activity logging be purely DB-based or also integrate with structured logs?

**Current State:** `activity_logs` table for UI display; minimal structured logging.

**Options:**
- **Option A:** Dual logging (zerolog for ops + activity_logs table for UI)
- **Option B:** DB-only for simplicity
- **Recommendation:** Option A (dual) for better observability

**Impact:** Logging helper design, job implementation patterns

---

### 13.6 Data Migration Strategy
**Question:** Do we need a migration script from existing PocketBase data or clean start?

**Current State:** Users may have existing mappings, blacklist entries, oauth tokens.

**Options:**
- **Option A:** Provide data migration script (read PocketBase SQLite → write to new schema)
- **Option B:** Clean start; users re-setup OAuth and recreate mappings
- **Option C:** Hybrid: migrate OAuth tokens and mappings; let jobs rebuild queue
- **Recommendation:** Option C (hybrid) for best user experience

**Impact:** Migration timeline; user documentation; testing scope

---

### 13.7 Frontend API Client Architecture
**Question:** Should we build a typed API client layer or use raw fetch?

**Current State:** PocketBase SDK provides typed methods (pb.collection('mappings').getList()).

**Options:**
- **Option A:** Thin fetch wrapper with manual type annotations
- **Option B:** Typed API client with methods mimicking PocketBase SDK
- **Recommendation:** Option B for better DX and type safety

**Impact:** Frontend migration effort; type definition maintenance

---

### 13.8 Testing Strategy for Jobs
**Question:** How to test background jobs with external API dependencies?

**Current State:** Jobs tested with mocked external APIs using httpmock.

**Options:**
- **Option A:** Continue httpmock for Spotify/YouTube API mocking
- **Option B:** Interface-based mocking (inject mock clients)
- **Recommendation:** Option A (httpmock) for less refactoring

**Impact:** Test code patterns; job helper design

---

## 14. Migration Considerations

### 14.1 Must Preserve
- **OAuth Integration:** Spotify + YouTube OAuth flows with token refresh
- **Playlist Mapping:** CRUD for mappings with sync configuration
- **Job System:** Analysis and executor jobs with scheduling
- **Activity Logging:** Dashboard stats and activity history
- **Blacklist Management:** Track conflict resolution
- **Frontend API Contract:** Response shapes and endpoints used by frontend

### 13.2 Can Change
- **Database access pattern:** PocketBase DAO → Jet typed SQL
- **Migration format:** Go functions → Goose SQL migrations
- **Admin UI:** Can remove PocketBase admin UI
- **Session management:** PocketBase sessions → Echo sessions
- **Error model:** Align with Echo error handling patterns

### 13.3 Must Add (New in Echo Stack)
- **Health endpoint:** `/api/health` with DB ping (similar to ebjoy)
- **Migration CLI:** `make backend/db/up`, `make backend/db/down`, `make backend/db/create`
- **Jet codegen:** `make backend/db/gen` after migrations
- **Structured logging:** Zerolog with request-scoped fields
- **Central error handler:** Sanitized error responses

---

## 14. Endpoints Requiring Migration

The following endpoints must be re-implemented in Echo:

**Custom Routes (Priority 1):**
- `POST /api/setup/save`
- `GET /api/setup/required`
- `GET /api/auth/spotify/login`
- `GET /api/auth/spotify/callback`
- `GET /api/spotify/playlists`
- `GET /api/auth/youtube/login`
- `GET /api/auth/youtube/callback`
- `GET /api/youtube/playlists`
- `GET /api/dashboard/stats`

**Collection REST API (Priority 2):**
- Mappings CRUD: `GET/POST/PATCH/DELETE /api/collections/mappings/records`
- Blacklist CRUD: `GET/POST/DELETE /api/collections/blacklist/records`
- Activity Logs: `GET /api/collections/activity_logs/records`
- Sync Items: `GET /api/collections/sync_items/records` (for debugging)

**Background Jobs (Priority 3):**
- Analysis job (replace PocketBase cron)
- Executor job (replace PocketBase cron)

---

## 15. Test Cases Requiring Migration

### 15.1 Backend Tests
**Auth System:**
- Spotify OAuth flow (login → callback → token storage)
- YouTube OAuth flow (login → callback → token storage)
- Credential loading (settings collection → env fallback)
- Token refresh logic

**Jobs:**
- Analysis job: Mapping selection, playlist diff detection, sync_item creation
- Executor job: Item execution, error handling, rate limiting, blacklist enforcement

**Dashboard:**
- Stats aggregation queries
- Activity log retrieval

**Setup Wizard:**
- Credentials save/retrieve
- Setup status check

**Mappings Hooks:**
- Default value initialization
- Validation on create/update
- Cascade delete of related records

### 15.2 Frontend Tests
**Components:**
- DashboardStatsCards rendering
- SpotifyConnectionCard OAuth status
- YoutubeConnectionCard OAuth status

**Routes:**
- Logs page data fetching and display
- Mappings list with pagination
- Blacklist management UI

**API Integration:**
- MSW handlers for PocketBase SDK calls
- TanStack Query caching and invalidation

### 15.3 E2E Tests (Playwright)
- Setup wizard flow
- OAuth flows (Spotify and YouTube)
- Mapping CRUD operations
- Full sync cycle (analysis → execution → dashboard update)

---

## 16. Success Criteria for Migration

The migration will be considered successful when:

1. **All endpoints functional:** Custom routes + collection REST API parity
2. **Jobs working:** Analysis and executor jobs running on schedule
3. **Frontend integrated:** All frontend tests passing with new backend
4. **Test coverage maintained:** All existing tests migrated and passing
5. **Dev workflow preserved:** `make dev`, `make test`, `make lint` working
6. **No deployment changes:** Local dev only (no Docker/deployment needed)
7. **Documentation complete:** Updated README, env.example, and migration notes

---

## Appendix A: File Structure

```
Spotube/
├── backend/
│   ├── cmd/server/main.go          # PocketBase entry point
│   ├── go.mod                       # Go dependencies
│   ├── env.example                  # Environment template
│   ├── internal/
│   │   ├── activitylogger/         # Activity logging to DB
│   │   ├── auth/                   # Unified OAuth system
│   │   ├── jobs/                   # Analysis + executor jobs
│   │   ├── pbext/                  # PocketBase extensions
│   │   │   ├── dashboard/          # Stats endpoint
│   │   │   ├── googleauth/         # YouTube OAuth
│   │   │   ├── mappings/           # Mappings hooks
│   │   │   ├── setupwizard/        # Setup routes
│   │   │   └── spotifyauth/        # Spotify OAuth
│   │   └── testhelpers/            # Test utilities
│   ├── migrations/                 # PocketBase Go migrations
│   └── pb_data/                    # SQLite DB + logs
├── frontend/
│   ├── src/
│   │   ├── routes/                 # TanStack Router routes
│   │   ├── components/             # React components
│   │   ├── lib/                    # API client + utils
│   │   └── test/                   # Test setup + mocks
│   ├── package.json
│   └── vite.config.ts
├── Makefile                        # Orchestration
└── README.md
```

---

## Appendix B: Migration Phase Recommendations

Based on ebjoy's RFC-020 through RFC-029 pattern, suggested phases for Spotube:

**Phase 0:** Project skeleton + health endpoint  
**Phase 1:** Database migrations (Goose) + Jet codegen  
**Phase 2:** Settings + setup wizard endpoints  
**Phase 3:** OAuth routes (Spotify + YouTube) + token storage  
**Phase 4:** Mappings CRUD endpoints  
**Phase 5:** Blacklist + activity logs endpoints  
**Phase 6:** Dashboard stats endpoint  
**Phase 7:** Background jobs (analysis + executor) with new scheduler  
**Phase 8:** Frontend integration + testing

---

**End of Discovery Document**

