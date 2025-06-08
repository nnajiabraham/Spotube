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
   - Backend (Go) server (exits immediately with "hello world" - placeholder)
   - Frontend (Vite) server at http://localhost:5173

### Available Commands

- `make dev` - Start development servers
- `make test` - Run all tests
- `make lint` - Run all linters
- `make build-image` - Build Docker image
- `make clean` - Clean build artifacts
- `make help` - Show all available targets

### Development Status

This is currently a **scaffold/bootstrap** implementation (RFC-001). The following foundation is complete:

- ✅ Go backend with zerolog structured logging
- ✅ React 19 + Vite frontend with Tailwind CSS
- ✅ ESLint, Prettier, Vitest configuration
- ✅ golangci-lint for Go code quality
- ✅ Multi-stage Docker build
- ✅ Makefile for development workflow

**Next Steps:**
- RFC-002: PocketBase foundation & migrations
- RFC-003: Environment setup wizard
- RFC-004+: OAuth integrations and sync functionality

## Tech Stack

- **Backend:** Go 1.24, PocketBase (planned), zerolog
- **Frontend:** React 19, TypeScript, Vite, Tailwind CSS, TanStack Router/Query
- **Testing:** Vitest, Playwright (planned)
- **Build:** Docker, Make

## Contributing

This project follows an RFC-driven development workflow. See `rfcs/` directory for planned features and implementation details. 