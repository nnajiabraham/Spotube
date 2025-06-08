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
- MSW-powered testing infrastructure
- Full test coverage for OAuth flows

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

## Tech Stack

- **Backend:** Go 1.24, PocketBase (embedded SQLite), Air (live reload)
- **Database:** SQLite via PocketBase with Go-based migrations
- **Frontend:** React 19, TypeScript, Vite, Tailwind CSS, TanStack Router/Query
- **Testing:** Vitest, Playwright (planned)
- **Build:** Docker, Make

## Contributing

This project follows an RFC-driven development workflow. See `rfcs/` directory for planned features and implementation details. 