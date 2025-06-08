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

The project foundation is complete through **RFC-002**. Current implementation includes:

- ✅ **RFC-001**: Go backend scaffold with zerolog structured logging
- ✅ **RFC-001**: React 19 + Vite frontend with Tailwind CSS  
- ✅ **RFC-001**: ESLint, Prettier, Vitest configuration
- ✅ **RFC-001**: golangci-lint for Go code quality
- ✅ **RFC-001**: Multi-stage Docker build
- ✅ **RFC-001**: Makefile for development workflow
- ✅ **RFC-002**: PocketBase integration with embedded SQLite
- ✅ **RFC-002**: Database migrations framework
- ✅ **RFC-002**: Admin UI and development tooling (Air live reload)
- ✅ **RFC-002**: Settings collection for OAuth credentials

**Next Steps:**
- RFC-003: Environment setup wizard (frontend + backend)
- RFC-004: Spotify OAuth integration
- RFC-005+: YouTube OAuth and sync functionality

## Tech Stack

- **Backend:** Go 1.24, PocketBase (embedded SQLite), Air (live reload)
- **Database:** SQLite via PocketBase with Go-based migrations
- **Frontend:** React 19, TypeScript, Vite, Tailwind CSS, TanStack Router/Query
- **Testing:** Vitest, Playwright (planned)
- **Build:** Docker, Make

## Contributing

This project follows an RFC-driven development workflow. See `rfcs/` directory for planned features and implementation details. 