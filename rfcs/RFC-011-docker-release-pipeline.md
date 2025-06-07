# RFC-011: Docker Build Pipeline & Type Checking

**Status:** Draft  
**Branch:** `rfc/011-docker-pipeline`  
**Depends On:**
* RFC-001 (initial Dockerfile skeleton)
* RFC-002 (PocketBase bootstrap)

---

## 1. Goal

Define the final, production-ready multi-stage `Dockerfile` that builds the single, statically-linked Go binary and embeds the compiled React frontend assets. Also, enforce TypeScript type checking as a mandatory step in the local linting process to ensure type safety before code is committed.

## 2. Background & Context

The PRD specifies a **single binary deployment**. This requires a Docker build process that:
1.  Builds the frontend static assets (`npm run build`).
2.  Builds the Go binary.
3.  Copies *both* into a minimal final runtime image.
4.  The Go binary serves the API at `/api/*` and the static assets from an embedded filesystem at `/`.

This RFC finalizes that process. It intentionally **omits CI/CD pipeline automation** (e.g., GitHub Actions to push to a registry), as that is out of scope for the MVP.

## 3. Technical Design

### 3.1 TypeScript Type Checking
The frontend lint command will be updated to include a TypeScript check.

**File**: `frontend/package.json`
```json
"scripts": {
  // ... other scripts
  "lint": "tsc --noEmit && eslint . --ext ts,tsx --report-unused-disable-directives --max-warnings 0",
  // ...
}
```
The `make lint` command in the root `Makefile` will now automatically run this check.

### 3.2 Final Multi-Stage Dockerfile
A single `Dockerfile` at the repository root will be used.

```dockerfile
# Stage 1: Build Frontend Assets
FROM node:20-alpine AS builder-frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Stage 2: Build Backend Binary
FROM golang:1.24-alpine AS builder-backend
WORKDIR /app/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# Build a static, production-ready binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/spotube ./cmd/server

# Stage 3: Final Runtime Image
FROM gcr.io/distroless/static:nonroot
WORKDIR /app

# Copy the static Go binary from the backend builder
COPY --from=builder-backend /app/spotube .

# Copy the compiled frontend assets from the frontend builder
# These will be served by the Go binary
COPY --from=builder-frontend /app/frontend/dist /app/pb_public

# Expose the port the Go app will listen on
EXPOSE 8090

# Set the entrypoint to our Go binary
ENTRYPOINT ["/app/spotube"]

# Default command to serve the application
CMD ["serve", "--http=0.0.0.0:8090"]
```

### 3.3 Go Application: Static Asset Serving
The Go binary needs to be modified to serve the static assets from the `/app/pb_public` directory when in production. PocketBase has built-in support for this.

**File**: `backend/cmd/server/main.go`
```go
package main

import (
    "log"
    "os"
    "path/filepath"
    "strings"

    "github.com/pocketbase/pocketbase"
    "github.com/pocketbase/pocketbase/plugins/migratecmd"
    // ... other imports
)

func main() {
    // Check if running from a compiled binary
    // In Go, os.Executable() returns the path to the running binary.
    // We can check if 'pb_public' exists relative to it.
    exePath, err := os.Executable()
    if err != nil {
        log.Fatal(err)
    }
    publicDir := filepath.Join(filepath.Dir(exePath), "pb_public")

    var app *pocketbase.PocketBase
    if _, err := os.Stat(publicDir); os.IsNotExist(err) {
        // 'pb_public' does not exist, run in dev mode (no embedded assets)
        app = pocketbase.New()
    } else {
        // 'pb_public' exists, run in production mode with embedded assets
        app = pocketbase.NewWithConfig(pocketbase.Config{
            DefaultDataDir: "pb_data",
            DefaultPublicDir: publicDir,
        })
    }

    // ... rest of the main function (migratecmd registration, app.Start(), etc.) ...
}
```
This logic allows the same binary to work in development (serving only the API) and in production (serving both API and the embedded frontend).

### 3.4 Makefile
The `make build-image` target in the root `Makefile` will use this Dockerfile.
```makefile
build-image:
	docker build -t spotube:latest .
```

## 4. Dependencies
*   Docker (for building the image)
*   Go toolchain (for building the binary)
*   Node.js/npm (for building the frontend)

## 5. Checklist
- [ ] **D1** Update `frontend/package.json` to include `tsc --noEmit` in the `lint` script.
- [ ] **D2** Finalize the multi-stage `Dockerfile` in the project root.
- [ ] **D3** Update `backend/cmd/server/main.go` to conditionally serve static assets from `pb_public`.
- [ ] **D4** Verify `make lint` now fails on TypeScript errors.
- [ ] **D5** Verify `make build-image` successfully builds the production image.
- [ ] **D6** Run the built image (`docker run -p 8090:8090 spotube:latest`) and confirm:
    *   The frontend loads at `http://localhost:8090`.
    *   API calls to `http://localhost:8090/api/...` work correctly.
    *   The PocketBase Admin UI loads at `http://localhost:8090/_/`.
- [ ] **D7** Update `README.md` with instructions on building and running the Docker image.

## 6. Definition of Done
*   A single `docker build` command produces a working, self-contained image.
*   The container serves both the React application and the Go API on a single port.
*   TypeScript type errors will fail the `make lint` command, enforcing type safety.

## Resources & References
*   PocketBase Go Docs: `NewWithConfig` – https://pocketbase.io/docs/go-overview/#app-initialization
*   Docker Multi-stage Builds – https://docs.docker.com/build/building/multi-stage/
*   Distroless Images – https://github.com/GoogleContainerTools/distroless

---

*End of RFC-011* 