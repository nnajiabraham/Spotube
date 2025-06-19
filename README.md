# Spotube - YouTube ⇆ Spotify Playlist Sync

A lightweight self-hosted application that keeps your YouTube Music and Spotify playlists in continuous, bi-directional sync.

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
   
   The wizard will save these credentials securely in the database. You can also provide them via environment variables:
   
   ```bash
   export SPOTIFY_ID="your-spotify-client-id"
   export SPOTIFY_SECRET="your-spotify-client-secret"
   export GOOGLE_CLIENT_ID="your-google-client-id"
   export GOOGLE_CLIENT_SECRET="your-google-client-secret"
   ```
   
   **Note**: If environment variables are set, the setup wizard will be skipped automatically.

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
- Sync analysis job with scheduled detection and work queue generation
- MSW-powered testing infrastructure
- Full test coverage for OAuth flows, mappings UI, and sync analysis

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

## Sync Analysis & Processing

Spotube uses a two-phase approach for playlist synchronization:

1. **Analysis Phase (RFC-007):** A background job routinely inspects mappings and generates work items
2. **Execution Phase (RFC-008):** A separate job processes work items with proper rate limiting

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

### Generated Work Items

The analysis creates work items in the `sync_items` collection:

- **Track Additions:** `add_track` actions with target service and track ID
- **Playlist Renames:** `rename_playlist` actions when titles drift
- **Status Tracking:** Each item has status (`pending`, `running`, `done`, `error`, `skipped`)

### Configuration

No environment variables are currently needed for the analysis job. All timing is controlled via the mapping's `interval_minutes` field, configurable through the UI (5-720 minutes).

### Monitoring

- Check PocketBase logs for analysis job activity
- View the `sync_items` collection in the admin UI for pending work
- Monitor mapping timestamps (`last_analysis_at`, `next_analysis_at`) for job health

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