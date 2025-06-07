# RFC-001: Repository Bootstrapping & Initial Toolchain

**Status:** Draft  
**Branch:** `rfc/001-repo-bootstrapping`  
**Related Issues:** _n/a_

---

## 1. Goal

Establish the foundational repository structure, language toolchains, and core developer experience for the **YouTube ⇆ Spotify Playlist Sync** project.  This RFC boots the monorepo, creates separate backend & frontend workspaces, enforces code-quality tooling, and sets up continuous integration so that subsequent feature RFCs have a predictable, reproducible environment.

## 2. Background & Context

The MVP outlined in the [Product Requirements Document](../PRD.md) delivers a single **statically-linked Go binary** (PocketBase-powered API + embedded React frontend).  Before implementing domain features we need:

* A clean directory layout that mirrors the runtime architecture (`backend/` + `frontend/`).
* Language toolchains pinned and reproducible (Go ≥ 1.24 via `go.mod`; Node ≥ 20.x via `package.json` & `.nvmrc`).
* Modern React 19 + Vite scaffold with Tailwind CSS.
* Linting, formatting, and unit-test harnesses in both tiers.

Getting this right early avoids "infrastructure debt" and gives future implementer agents deterministic commands (e.g. `make dev`, `make test`).

## 3. Technical Design

### 3.1 Repository Layout
```
.
├── backend/                # Go / PocketBase source
│   ├── cmd/server/         # main.go entrypoint
│   └── go.mod              # Go module definition
├── frontend/               # React 19 + Vite SPA
│   ├── src/
│   ├── vite.config.ts
│   ├── tsconfig.json
│   └── package.json
├── docker/
│   └── Dockerfile         # multi-stage builder (Go + node build)
├── Makefile               # canonical dev & CI commands
├── .editorconfig
├── .golangci.yml          # Go linters config
├── eslint.config.js       # Frontend lint rules (flat-config)
├── .prettierrc
└── .nvmrc
```

### 3.2 Backend Scaffold
* Initialise module: `go mod init github.com/<org>/spotube` (placeholder import path).
* Target Go 1.24; use `go.work` later if modules split.
* Add baseline dependencies:
  * `github.com/pocketbase/pocketbase` – **NOT imported yet** (integration handled in RFC-002).
  * `github.com/rs/zerolog` – structured logging.
* `cmd/server/main.go` just prints "hello world" & exits with 0 to satisfy CI.
* Configure **golangci-lint** (fast-fail on vet, staticcheck, unused, revive).  Stored in `.golangci.yml`.

### 3.3 Frontend Scaffold
* Use Vite 5 (or latest) with React 19 template: `npm create vite@latest frontend -- --template react-ts`.
* Tailwind CSS setup via `npx tailwindcss init -p` (adds `tailwind.config.ts`, `postcss.config.js`).
* Install core dependencies:
  * `react@19`, `react-dom@19` (automatic via template).
  * `@tanstack/react-router`, `@tanstack/react-query`, `zod`, `clsx`.
* Dev experience:
  * **ESLint** flat-config with `eslint-plugin-react`, `eslint-plugin-react-hooks`, `typescript-eslint`.
  * **Prettier** for formatting.
  * **Vitest** for unit tests (`npm exec vitest`).

### 3.4 Makefile Targets (developer ergonomics)
```
# Backend (Go) hot-reload via Air (https://github.com/air-verse/air)
#   - first time: go install github.com/air-verse/air@latest
#   - configuration: default settings are fine; can customize later with .air.toml if needed
# Frontend (Vite) runs on :5173 by default.
# Concurrency handled via npm-run-all (or your shell) – adjust if you prefer docker-compose, tmux, etc.

make dev            # concurrently: air (backend) + vite dev server (frontend)
make test           # go test ./... && npm run test --workspace frontend
make lint           # golangci-lint run && npm run lint --workspace frontend
make build-image    # docker build -f docker/Dockerfile -t spotube:dev .
```

> **Implementation hint** – a simple `make dev` implementation:
> ```makefile
> dev:
> 	cd backend && air & \
> 	cd frontend && npm run dev
> ```
> (Use a task-runner like `forego`, `taskfile`, or `npm-run-all` if backgrounding with `&` is insufficient on your OS.)

### 3.5 Docker Skeleton
Multi-stage file (under `docker/` folder now but root COPY later).
1. **builder-go** – scratch + Go 1.24 slim → builds `backend/cmd/server`.
2. **builder-node** – node:lts-slim → `npm ci && npm run build` inside `frontend/`.
3. **runtime** – distroless/static copying Go binary & `frontend/dist` → binary serves static assets (actual embed done in later RFCs).

### 3.6 Developer Tooling Pinning
* `.nvmrc` → `20.12.2` (LTS at time of writing).
* `engines` field in root `package.json` to enforce Node 20.
* EditorConfig ensures LF, 2-space indent for YAML & JSON, gofmt for Go sources.

## 4. Dependencies

* **Backend**
  * Go ≥ 1.24 (install via GVM or `asdf`).
  * `github.com/rs/zerolog` (logging) – MIT
* **Frontend**
  * Node 20.x, npm 10.x (comes with Node)
  * Vite, React 19, Tailwind CSS, TanStack Router/Query
  * Vitest (unit test), ESLint, Prettier
* **Docker**
  * `golang:1.24-alpine` & `node:20-alpine` builder stages
  * `gcr.io/distroless/static:nonroot` runtime image
* **Dev-only**
  * `github.com/air-verse/air` (dev-only, live reload) – GPL-3.0 ([repo](https://github.com/air-verse/air))

## 5. Checklist

- [ ] **B1** Create directory structure (`backend/`, `frontend/`).
- [ ] **B2** Initialise Go module & commit minimal `main.go`.
- [ ] **B3** Scaffold React 19 + Vite app with Tailwind.
- [ ] **B4** Add ESLint, Prettier, Vitest config in `frontend/`.
- [ ] **B5** Add `.golangci.yml` & enable strict linters.
- [ ] **B6** Add Makefile with `dev`, `test`, `lint`, `build-image` targets.
- [ ] **B7** Add Dockerfile skeleton (multi-stage; no PocketBase yet).
- [ ] **B9** Add `.nvmrc`, `.editorconfig`, root `README` quick-start section.
- [ ] **B10** Ensure `make dev` spins up both servers concurrently (use `forego` or `npm-run-all`).
- [ ] **B11** All CI checks green on dedicated feature branch.

## 6. Definition of Done

* `make dev`, `make test`, `make lint`, `make build-image` all succeed locally on macOS & Linux.
* Docker image `spotube:dev` runs and serves backend "OK" on `:8090` and Vite build artefacts on `/` (placeholder HTML).
* No PocketBase or production logic introduced yet – strictly scaffold.
* README updated with bootstrap instructions.

## Implementation Notes / Summary

* Chose **Vite** over Nx/Turborepo to keep toolchain lightweight; monorepo orchestration handled via Makefile & scripts.
* PocketBase integration is deferred to **RFC-002 (PocketBase Foundation & Migrations Framework)** to keep change sets reviewable.
* Multi-stage Docker skeleton proves future single-binary image concept without yet embedding assets.
* CI pipeline intentionally simple; caching layers tuned in later RFCs.

## Resources & References

* Vite React TS guide – https://vitejs.dev/guide/
* Tailwind CSS installation with Vite – https://tailwindcss.com/docs/guides/vite
* Go 1.24 release notes – https://tip.golang.org/doc/go1.24
* Zerolog structured logging – https://github.com/rs/zerolog
* Distroless images – https://github.com/GoogleContainerTools/distroless
* Reference architecture inspiration – [longhabit project](https://github.com/s-petr/longhabit) (monorepo, Go + React)
* **Air live reload for Go** – https://github.com/air-verse/air

---

*End of RFC-001* 