# RFC-002: PocketBase Foundation & Migrations Framework

**Status:** Draft  
**Branch:** `rfc/002-pocketbase-foundation`  
**Related Issues:** _n/a_

---

## 1. Goal

Embed a PocketBase instance inside the Go backend, expose the default Admin UI on a dedicated port during development, and introduce an opinionated Go-based migration workflow so that future RFCs can evolve the database schema safely.  This lays the groundwork for collections (`settings`, etc.) and scheduled jobs described in the PRD without yet implementing any domain logic.

## 2. Background & Context

RFC-001 bootstrapped the repository with separate `backend/` (Go) and `frontend/` (React) workspaces.  The runtime architecture calls for a **single statically-linked binary** that bundles PocketBase (API + SQLite) and the compiled React assets.  Up to now the Go application is a stub.

We must now:

1. Initialise PocketBase (`pocketbase.New()`) in `backend/cmd/server/main.go`.
2. Ensure migrations are tracked in VCS (`backend/pb_migrations/`).
3. Provide developer UX: `make migrate-up` & hot-reload (`air`) automatically run migrations on launch.
4. Expose PocketBase Admin UI at `http://localhost:8090/_/` (dev only).
5. Keep future production image on a single port later (still 8090).

PocketBase docs:
* Overview – <https://pocketbase.io/docs/>
* Go SDK Overview – <https://pocketbase.io/docs/go-overview/>
* Go Migrations – <https://pocketbase.io/docs/go-migrations/>
* Job Scheduler – <https://pocketbase.io/docs/go-jobs-scheduling/>

## 3. Technical Design

### 3.1 Dependencies
```bash
# inside backend/
go get github.com/pocketbase/pocketbase@v0.21.0   # current stable
# structured logging (already zerolog in RFC-001)
```

> NOTE: PocketBase pulls in Fiber v2 internally; no direct dependency required in our code.

### 3.2 Directory Layout Additions
```
backend/
 ├── cmd/server/main.go      # updated to start PB
 ├── pb_data/                # runtime SQLite + uploads  (git-ignored)
 └── pb_migrations/          # Go migration files        (tracked)
```

Add to `.gitignore`:
```
backend/pb_data/
```

### 3.3 `main.go` Bootstrap (dev port 8090)
```go
package main

import (
    "log"
    "os"
    "strings"

    "github.com/pocketbase/pocketbase"
    "github.com/pocketbase/pocketbase/plugins/migratecmd"
)

func main() {
    app := pocketbase.New()

    // Register `pb migrate` sub-command so we can run `go run ./cmd/server migrate up`.
    isGoRun := strings.HasPrefix(os.Args[0], os.TempDir())
    migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
        Automigrate: isGoRun, // Dev: auto-generate migrations when using Admin UI
    })

    // Serve PocketBase (defaults to :8090) – production port defined via ENV PORT.
    if err := app.Start(); err != nil {
        log.Fatal(err)
    }
}
```

### 3.4 Makefile Enhancements
```makefile
PB_DEV_PORT ?= 8090

# run backend w/ Air (live reload) – PB auto-migrates on each restart
backend-dev:
	cd backend && PORT=$(PB_DEV_PORT) air

# run migrations manually (e.g. CI)
make migrate-up:
	cd backend && go run cmd/server migrate up
```

### 3.5 First Migration: create `settings` collection
Use PocketBase CLI generator:
```bash
cd backend && go run cmd/server migrate create "init_settings_collection"
```
This produces `pb_migrations/1660000000_init_settings_collection.go`.  Edit Up/Down to:
```go
collection := &models.Collection{
    Name: "settings",
    Type: models.CollectionTypeBase,
    System: true,             // singleton
    Schema: schema.NewSchema(
        &schema.SchemaField{Name: "spotify_client_id",  Type: schema.FieldTypeText, Required: false},
        &schema.SchemaField{Name: "spotify_client_secret", Type: schema.FieldTypeText, Required: false},
        &schema.SchemaField{Name: "google_client_id",  Type: schema.FieldTypeText, Required: false},
        &schema.SchemaField{Name: "google_client_secret", Type: schema.FieldTypeText, Required: false},
    ),
}
```
Down() simply deletes the collection.

### 3.6 Admin UI & First-Run Flow
Developers navigate to `/_/` → create super-admin user.  Document in README; credentials live only in local `pb_data/`.

### 3.7 Future-proofing
* **Automigrate** is enabled only when running via `air` (detected via `go run`).  In production builds migrations are compiled but not auto-applied; we run `server migrate up` in Dockerfile build stage.
* Custom API routes and hooks will live in `backend/internal/pbext/` (created by later RFCs).

## 4. Dependencies
* `github.com/pocketbase/pocketbase` – MIT
* (dev) `github.com/pocketbase/pocketbase/plugins/migratecmd` – included above

## 5. Checklist
- [ ] **F1** Add PocketBase dependency & commit `go.mod`/`go.sum`.
- [ ] **F2** Create `pb_data/` dir, add to `.gitignore`.
- [ ] **F3** Add `pb_migrations/` dir; commit initial `init_settings_collection.go` migration.
- [ ] **F4** Update `backend/cmd/server/main.go` with PocketBase bootstrap + migratecmd.
- [ ] **F5** Enhance Makefile: `backend-dev` (Air) & `make migrate-up` targets.
- [ ] **F6** Verify `make backend-dev` opens Admin UI at `/_/` and can create super-admin.
- [ ] **F7** Confirm `make migrate-up` on clean workspace creates `settings` collection.
- [ ] **F8** Update root README with PocketBase dev flow.

## 6. Definition of Done
* `make backend-dev` hot-reloads; Admin UI reachable on localhost.
* `pb_migrations` compiles and first migration applies without error.
* No existing backend tests fail (currently none).
* README includes instructions for first-run admin creation.

## Implementation Notes / Summary
* PocketBase version pinned to `v0.21.x`; verify changelog each quarter.
* `Automigrate` helps devs iterate but shouldn't run in prod; we guard by detecting `go run` path.  Alternate approach: env var toggle – revisit later.
* Singleton `settings` collection stores OAuth secrets encrypted at rest (PB handles encryption when `System: true`).

## Resources & References
* PocketBase Go Overview – https://pocketbase.io/docs/go-overview/
* PocketBase Go Migrations – https://pocketbase.io/docs/go-migrations/
* PocketBase Admin UI – https://pocketbase.io/docs/admin-panel/
* Air live-reload – https://github.com/air-verse/air

---

*End of RFC-002* 