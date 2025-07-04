# Spotube - YouTube ⇆ Spotify Playlist Sync

A lightweight self-hosted application that keeps your YouTube Music and Spotify playlists in continuous, bi-directional sync.

NOTE: THIS WAS VIBE CODED WITH AI. SEE THE [RFCs](./rfcs) FOR THE PLAN USED WITH LLM TO BUILD THIS.

## Quick Start

### Prerequisites

- Go 1.24+
- Node.js 20+ (recommended: use `.nvmrc` with `nvm use`)
- Docker (optional)

### Development Setup

1. **Clone and install dependencies:**
   ```bash
   git clone <repository-url>
   cd Spotube
   
   # Install frontend dependencies
   cd frontend && npm install && cd ..
   ```

2. **Start development servers:**
   ```bash
   make dev
   ```
   This will start:
   - Backend (PocketBase) server at http://localhost:8090
   - Frontend (Vite) server at http://localhost:5173

   **Or start backend only with live reload:**
   ```bash
   make backend-dev
   ```

3. **Initialize database (first time only):**
   ```bash
   make migrate-up
   ```

4. **First-run setup:**
   
   When you first visit http://localhost:5173, you'll be guided through the **Environment Setup Wizard** to configure your OAuth credentials:
   
   - **Spotify OAuth**: Create an app at https://developer.spotify.com/dashboard and get your Client ID and Client Secret
   - **Google OAuth**: Set up a project at https://console.cloud.google.com/ and create OAuth 2.0 credentials
   
   The wizard will save these credentials securely in the database. You can also provide them via environment variables or a `.env` file:
   
   **Option 1: Environment Variables**
   ```bash
   export SPOTIFY_CLIENT_ID="your-spotify-client-id"
   export SPOTIFY_CLIENT_SECRET="your-spotify-client-secret"
   export GOOGLE_CLIENT_ID="your-google-client-id"
   export GOOGLE_CLIENT_SECRET="your-google-client-secret"
   ```
   
   **Option 2: .env File (Recommended for Development)**
   ```bash
   # Copy the example and edit with your values
   cp backend/env.example backend/.env
   # Edit backend/.env with your actual OAuth credentials
   ```
   
   **Note**: If environment variables or .env file credentials are set, the setup wizard will be skipped automatically. The app loads .env files automatically on startup for development convenience.

### PocketBase Development Flow

The backend uses **PocketBase** as the foundation, providing:
- Built-in SQLite database with migrations
- Admin UI at http://localhost:8090/_/ (first-time setup required)
- REST API for collections and authentication
- File uploads and OAuth integrations

**First-time setup:**
1. Run `make backend-dev` or `make migrate-up`
2. Visit http://localhost:8090/_/ to create admin account
3. Explore the admin interface to see collections and settings

### Available Commands

- `make dev` - Start development servers (backend + frontend)
- `make backend-dev` - Start backend with Air (live reload)
- `make backend-workers` - Start backend with continuous analysis+executor workers (dev)
- `make migrate-up` - Run database migrations manually
- `make test` - Run all tests
- `make lint` - Run all linters
- `make build-image` - Build Docker image
- `make clean` - Clean build artifacts
- `make help` - Show all available targets

### Development Status

✅ **Completed RFCs:**
- RFC-001: Repository initialization with Go backend and React frontend
- RFC-002: PocketBase integration with migrations framework
- RFC-003: Environment setup wizard for OAuth credentials
- RFC-004: Spotify OAuth integration with PKCE flow
- RFC-005: YouTube OAuth integration with PKCE flow
- RFC-006: Playlist mapping collections & UI
- RFC-007: Sync analysis job (scheduled detection)
- RFC-008: Sync execution job (worker processing queue)
- RFC-009: Conflict & blacklist handling system

**Current Features:**
- Monorepo structure with separate backend/frontend workspaces
- PocketBase embedded with Admin UI (port 8090)
- Go-based migrations system for database schema evolution  
- Environment setup wizard for first-time configuration
- Settings collection for storing OAuth credentials
- Spotify OAuth2 authentication with PKCE security
- YouTube OAuth2 authentication with PKCE security
- Spotify playlists API proxy endpoint
- YouTube playlists API proxy endpoint
- Frontend dashboard with connection status for both services
- Playlist mappings management (CRUD operations)
- Mapping creation wizard with 4-step flow
- Configurable sync options (name, tracks, interval)
- Two-phase sync system: analysis job (detection) + execution job (processing)
- Worker pool with concurrent processing and rate limiting
- Exponential backoff retry logic with error classification
- YouTube quota tracking with daily limits
- Automatic blacklist system for failed tracks with conflict resolution UI
- Color-coded blacklist management with per-mapping track exclusions
- MSW-powered testing infrastructure
- Full test coverage for OAuth flows, mappings UI, sync analysis, execution workers, and blacklist functionality

## Spotify OAuth Setup

To use Spotify integration, you'll need to:

1. **Create a Spotify App:**
   - Go to [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)
   - Click "Create App"
   - Fill in app details
   - Add redirect URIs (see below)

2. **Configure Redirect URIs:**
   Add these redirect URIs in your Spotify app settings:
   - Development: `http://localhost:8090/api/auth/spotify/callback`
   - Production: `https://your-domain.com/api/auth/spotify/callback`

3. **Set Credentials:**
   Either through the setup wizard (http://localhost:8090/setup) or environment variables:
   ```bash
   export SPOTIFY_CLIENT_ID="your-client-id"
   export SPOTIFY_CLIENT_SECRET="your-client-secret"
   export PUBLIC_URL="http://localhost:8090"  # or your production URL
   ```

4. **Connect Your Account:**
   - Navigate to the dashboard
   - Click "Connect Spotify"
   - Authorize the app
   - You'll be redirected back with your playlists accessible

## YouTube/Google OAuth Setup

To use YouTube integration, you'll need to:

1. **Create a Google Cloud Project:**
   - Go to [Google Cloud Console](https://console.cloud.google.com/)
   - Create a new project or select an existing one
   - Enable the YouTube Data API v3

2. **Configure OAuth Consent Screen:**
   - In the Google Cloud Console, go to "APIs & Services" > "OAuth consent screen"
   - Choose "External" user type (unless using Google Workspace)
   - Fill in required fields:
     - App name: Spotube (or your preferred name)
     - User support email: Your email
     - Developer contact information: Your email
   - Add scopes: `https://www.googleapis.com/auth/youtube.readonly`
   - Add test users if in development/testing phase

3. **Create OAuth 2.0 Credentials:**
   - Go to "APIs & Services" > "Credentials"
   - Click "Create Credentials" > "OAuth client ID"
   - Choose "Web application"
   - Add authorized redirect URIs:
     - Development: `http://localhost:8090/api/auth/google/callback`
     - Production: `https://your-domain.com/api/auth/google/callback`
   - Copy the Client ID and Client Secret

4. **Set Credentials:**
   Either through the setup wizard (http://localhost:8090/setup) or environment variables:
   ```bash
   export GOOGLE_CLIENT_ID="your-client-id"
   export GOOGLE_CLIENT_SECRET="your-client-secret"
   export PUBLIC_URL="http://localhost:8090"  # or your production URL
   ```

5. **Connect Your Account:**
   - Navigate to the dashboard
   - Click "Connect YouTube"
   - Authorize the app with your Google account
   - You'll be redirected back with your YouTube playlists accessible

**Note:** Google requires HTTPS for production OAuth redirects (except for localhost). Make sure your production deployment uses HTTPS.

## Unified OAuth Authentication System

Spotube uses a unified OAuth authentication system (RFC-008b) that eliminates code duplication and provides consistent authentication across all components:

### Credential Loading Priority

The system loads OAuth credentials in this priority order:
1. **Settings Collection** - Credentials stored in the database via the setup wizard
2. **Environment Variables** - Fallback to env vars if database credentials are missing

### Authentication Contexts

The unified system supports two execution contexts:

**Background Jobs Context:**
- Used by sync analysis and execution jobs
- Loads credentials from settings collection with environment fallback
- Handles token refresh automatically during job execution

**API Handler Context:**
- Used by OAuth callback endpoints and playlist API proxies
- Same credential loading as jobs but integrated with Echo HTTP context
- Maintains session state for web-based OAuth flows

### Settings Collection Integration

OAuth credentials are stored securely in the `settings` collection:
- `spotify_client_id` and `spotify_client_secret`
- `google_client_id` and `google_client_secret`

These can be configured through:
- **Setup Wizard UI** at http://localhost:5173/setup
- **Environment Variables** as fallback

### Token Management

- **Automatic Refresh:** Tokens are refreshed automatically when expired (30-second buffer)
- **Database Persistence:** Refreshed tokens are saved to the `oauth_tokens` collection
- **Thread Safety:** Token operations are thread-safe for concurrent job execution
- **Error Handling:** Comprehensive error handling for token refresh failures

### Backward Compatibility

The unified system maintains full backward compatibility:
- All existing OAuth endpoints continue to work unchanged
- API signatures remain identical for external consumers
- Frontend integration requires no changes

This unified approach reduces maintenance burden and ensures consistent OAuth behavior across all Spotube components.

## Playlist Mappings

After connecting both your Spotify and YouTube accounts, you can create playlist mappings to keep them synchronized:

### Creating a Mapping

1. **Navigate to Mappings:**
   - From the dashboard, click "View Mappings"
   - Or navigate directly to `/mappings`

2. **Create New Mapping:**
   - Click "Add mapping" to start the creation wizard
   - **Step 1:** Select a Spotify playlist to sync
   - **Step 2:** Select a YouTube playlist to sync with
   - **Step 3:** Configure sync options:
     - **Sync Name:** Keep playlist titles synchronized between platforms
     - **Sync Tracks:** Keep track lists synchronized (add/remove songs)
     - **Sync Interval:** How often to check for changes (5-720 minutes)
   - **Step 4:** Review and save your mapping

### Managing Mappings

- **View All Mappings:** The mappings list shows all your configured sync pairs with their current settings
- **Edit Mapping:** Click the edit icon to modify sync options and interval (playlist selection cannot be changed)
- **Delete Mapping:** Click the trash icon to remove a mapping (requires confirmation)

### Sync Behavior

- **Bi-directional:** Changes on either platform will be synced to the other
- **Scheduled:** Syncs run automatically at the configured interval
- **Duplicate Prevention:** You cannot create multiple mappings for the same playlist pair
- **Validation:** Minimum sync interval is 5 minutes to respect API rate limits

### Technical Details

- Mappings are stored in the `mappings` collection with a unique constraint on playlist pairs
- Cached playlist names are displayed for better UX (refreshed during sync)
- All operations require authentication
- Sync execution is handled by scheduled jobs (see Sync Analysis & Processing section)

## Blacklist Management

When synchronizing playlists between Spotify and YouTube, some tracks may fail to sync due to various issues like regional restrictions, content not available on the target platform, or API errors. Spotube includes a comprehensive blacklist system to handle these conflicts gracefully.

### Automatic Blacklisting

**How It Works:**
- When a track consistently fails to sync with a fatal error, it's automatically added to the blacklist
- This prevents infinite retry loops and reduces API quota consumption
- Blacklisted tracks are excluded from future sync analysis until manually removed

**Error Categories:**
- **Not Found (404):** Track doesn't exist or isn't available on the target platform
- **Forbidden (403):** Regional restrictions or content blocked in your location  
- **Unauthorized (401):** Permission issues or expired credentials
- **Invalid:** Malformed track IDs or corrupted data
- **Error:** Other unrecoverable errors

### Managing Blacklisted Tracks

**Viewing Blacklist:**
1. Navigate to your mapping edit page (`/mappings/{id}/edit`)
2. Click "View Blacklist" to see all blacklisted tracks for that mapping
3. The blacklist shows:
   - **Service:** Which platform (Spotify/YouTube) the track failed on
   - **Track ID:** The unique identifier for the failed track
   - **Reason:** Why the track was blacklisted (color-coded)
   - **Skip Count:** How many times the track has been skipped
   - **Last Skipped:** When the track was most recently blacklisted

**Un-blacklisting Tracks:**
- Click the trash icon next to any blacklist entry
- Confirm the removal when prompted
- The track will be retried in the next sync analysis cycle
- Useful when regional restrictions are lifted or content becomes available

### Blacklist Behavior

**Per-Mapping Scope:**
- Blacklists are specific to each playlist mapping
- A track blacklisted in one mapping won't affect other mappings
- Allows fine-grained control over sync conflicts

**Automatic Prevention:**
- Analysis job filters out blacklisted tracks before creating sync items
- Executor job creates blacklist entries when fatal errors occur
- System maintains skip counters for tracking repeated failures

**Manual Override:**
- Users can remove any blacklist entry to retry problematic tracks
- Useful for temporary issues that may have been resolved
- No bulk operations currently supported (individual track management)

### Common Blacklist Scenarios

**Regional Content:**
- Music videos not available in your country on YouTube
- Tracks geo-blocked on Spotify in certain regions
- **Solution:** Content may become available later; retry by un-blacklisting

**Platform Exclusives:**
- Podcast episodes only on Spotify
- YouTube-specific content (covers, remixes) not on Spotify
- **Solution:** Expected behavior; these remain blacklisted

**API Changes:**
- Track IDs that change due to platform updates
- Content that gets removed and re-added with new IDs
- **Solution:** Remove old blacklist entries and let sync detect new versions

### Technical Implementation

- Blacklist entries stored in the `blacklist` collection
- Composite unique index on `(mapping_id, service, track_id)`
- Integration with both analysis filtering and executor error handling
- Color-coded UI with service badges for easy visual identification

## Sync Analysis & Processing

Spotube uses a two-phase approach for playlist synchronization:

1. **Analysis Phase (RFC-007):** A background job routinely inspects mappings and generates work items
2. **Execution Phase (RFC-008):** A separate job processes work items with proper rate limiting and quota management

### Development Worker Mode

For development, you can run the backend with continuous workers using:

```bash
make backend-workers
```

This starts the backend with both analysis and execution jobs running continuously, using Air for live reload when job code changes.

### Analysis Job Schedule

- **Frequency:** Runs every minute via cron scheduler (`*/1 * * * *`)
- **Per-Mapping Interval:** Each mapping has its own `interval_minutes` setting (default: 60 minutes)
- **Smart Scheduling:** Only analyzes mappings where `next_analysis_at` has passed

### Analysis Process

For each eligible mapping, the analysis job:

1. **Fetches Current State:** Retrieves track lists from both Spotify and YouTube
2. **Bidirectional Diff:** Calculates what tracks need to be added to each platform
3. **Name Sync:** Checks for playlist title differences (if `sync_name=true`)
4. **Work Queue:** Creates `sync_items` records for the execution phase
5. **Timestamp Update:** Sets `last_analysis_at` and `next_analysis_at` for the mapping

### Execution Job Schedule

- **Frequency:** Runs every 5 seconds via cron scheduler (`*/5 * * * * *`)
- **Batch Processing:** Processes up to 50 pending items per execution cycle
- **Concurrent Workers:** Uses worker pool with maximum 5 concurrent operations
- **Smart Queuing:** Only processes items where `next_attempt_at` has passed

### Execution Process

For each eligible sync item, the execution job:

1. **Status Update:** Marks item as `running` and increments attempt counter
2. **Action Dispatch:** Routes to appropriate handler based on `service:action` combination
3. **Rate Limiting:** Respects Spotify rate limits (10 requests/second) and YouTube quota
4. **Error Classification:** Handles rate limits, fatal errors, and temporary errors differently
5. **Retry Logic:** Implements exponential backoff for retryable errors

### Generated Work Items

The analysis creates work items in the `sync_items` collection:

- **Track Additions:** `add_track` actions with target service and track ID
- **Playlist Renames:** `rename_playlist` actions when titles drift
- **Status Tracking:** Each item has status (`pending`, `running`, `done`, `error`, `skipped`)
- **Retry Control:** Items include `next_attempt_at` and `attempt_backoff_secs` for scheduling
- **Execution History:** Tracks `attempts` count and `last_error` for debugging

### Error Handling & Retry Logic

The execution job implements sophisticated error handling:

**Rate Limit Errors:**
- HTTP 429, "rate limit", "too many requests"
- **Action:** Retry with exponential backoff
- **Backoff Formula:** `min(2^attempts * 30, 3600)` seconds (30s to 1 hour cap)

**Fatal Errors:**
- HTTP 404, 403, 401, "invalid", "forbidden", "unauthorized"
- **Action:** Mark as `error` status, no retry
- **Use Case:** Deleted playlists, revoked permissions, invalid IDs

**Temporary Errors:**
- Network timeouts, 5xx server errors, other transient issues
- **Action:** Retry with exponential backoff
- **Backoff Formula:** Same as rate limits

**YouTube Quota Management:**
- **Daily Limit:** 10,000 quota units per day (resets at UTC midnight)
- **Track Addition Cost:** 50 units per operation
- **Playlist Rename Cost:** 1 unit per operation
- **Quota Exhausted:** Items marked as `skipped` with `last_error="quota"`
- **Automatic Reset:** Quota tracker resets usage at UTC midnight

### Worker Pool Configuration

The execution job uses configurable constants (currently hardcoded):

- **`BATCH_SIZE = 50`** - Maximum items processed per execution cycle
- **`MAX_CONCURRENCY = 5`** - Maximum concurrent worker threads
- **`SPOTIFY_RATE_LIMIT = 10`** - Spotify API requests per second (conservative)
- **`YOUTUBE_DAILY_QUOTA = 10000`** - YouTube quota units per day
- **`YOUTUBE_ADD_TRACK_COST = 50`** - Quota cost for adding tracks

### Configuration

Currently, no environment variables are needed for sync jobs. All configuration is either:
- **Mapping-level:** `interval_minutes` configurable via UI (5-720 minutes)
- **System-level:** Hardcoded constants in the executor implementation

Future releases may expose worker configuration via environment variables.

### Monitoring

**Analysis Job:**
- Check PocketBase logs for analysis job activity
- Monitor mapping timestamps (`last_analysis_at`, `next_analysis_at`) for job health

**Execution Job:**
- Check PocketBase logs for executor job activity and worker pool operations
- View the `sync_items` collection in admin UI for work queue status
- Monitor item status distribution (`pending`, `running`, `done`, `error`, `skipped`)
- Track retry attempts and error patterns via `last_error` fields
- YouTube quota usage logged with daily reset notifications

**Key Log Patterns:**
```
Starting sync executor job...
Found X pending sync items to process
Processing sync item: service=spotify, action=add_track
YouTube quota consumed: used=150/10000 (cost=50)
Retrying item abc123 in 60 seconds (attempt 2)
Successfully processed sync item abc123
```

## Tech Stack

- **Backend:** Go 1.24, PocketBase (embedded SQLite), Air (live reload)
- **Database:** SQLite via PocketBase with Go-based migrations
- **Frontend:** React 19, TypeScript, Vite, Tailwind CSS, TanStack Router/Query
- **Testing:** Vitest, Playwright (planned)
- **Build:** Docker, Make

## Contributing

This project follows an RFC-driven development workflow. See `rfcs/` directory for planned features and implementation details.

## Testing

### Shared Test Helpers

The project includes a comprehensive testing infrastructure with shared helpers to ensure consistent and maintainable tests across all packages.

#### Available Test Helpers

**Backend Helpers (`backend/internal/testhelpers/`):**

- **`testhelpers.SetupTestApp(t)`** - Creates a PocketBase test instance with all standard collections
- **`testhelpers.SetupOAuthTokens(t, testApp)`** - Creates fake OAuth tokens for Spotify and Google
- **`testhelpers.CreateTestMapping(testApp, properties)`** - Helper to easily create mapping records with defaults
- **`testhelpers.SetupAPIHttpMocks(t)`** - Configures HTTP mocks for Spotify and YouTube APIs
- **`testhelpers.SetupIdenticalPlaylistMocks(t)`** - Special mocks for testing no-change scenarios

#### Usage Example

```go
func TestYourFeature(t *testing.T) {
    // Setup test environment
    testApp := testhelpers.SetupTestApp(t)
    defer testApp.Cleanup()
    
    // Add OAuth tokens for API testing
    testhelpers.SetupOAuthTokens(t, testApp)
    
    // Mock HTTP calls
    testhelpers.SetupAPIHttpMocks(t)
    defer httpmock.DeactivateAndReset()
    
    // Create test data
    mapping := testhelpers.CreateTestMapping(testApp, map[string]interface{}{
        "spotify_playlist_id": "test_playlist",
        "sync_tracks": true,
    })
    
    // Test your function
    err := yourFunction(testApp, mapping)
    assert.NoError(t, err)
}
```

#### Key Testing Principles

- **Test Real Implementation:** All unit tests call actual implementation functions with PocketBase integration
- **No Mocked Logic:** Tests validate real behavior, not simulated logic
- **Consistent Setup:** Shared helpers ensure all tests use the same database schema and OAuth patterns
- **Proper Isolation:** Each test runs with a clean database and HTTP mock environment
- **PocketBase Integration:** Tests use real PocketBase operations, not isolated database mocking

#### Running Tests

```bash
# Backend tests
make test-backend
# or
cd backend && go test ./...

# Frontend tests  
make test-frontend
# or
cd frontend && npm test

# All tests
make test
``` 