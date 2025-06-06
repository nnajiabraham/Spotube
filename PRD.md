# YouTube ⇆ Spotify Playlist Sync – Product Requirements Document (MVP)

## 1. Introduction
A lightweight self-hosted application that keeps a user's YouTube Music and Spotify playlists in continuous, bi-directional sync.  The app is designed to run as a single-user PocketBase service bundled into a single Go binary (plus an embedded React frontend).  After an initial one-time setup (providing Spotify and Google API credentials), the user authenticates to both music services, defines playlist mappings, and the backend takes care of keeping titles and tracks aligned on a schedule.

## 2. Core Value Proposition
Eliminate the manual effort of recreating or updating playlists across Spotify and YouTube Music.  With one small self-hosted service you:
• Keep playlists mirrored automatically in both platforms.
• Control what syncs (name only or full track list) per mapping.
• Avoid vendor lock-in while fully owning your data and credentials.

## 3. Key Features (MVP)
1. **Environment Setup Wizard** – On first run, the web UI prompts for required secrets (`SPOTIFY_ID`, `SPOTIFY_SECRET`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`).  Values are stored in a secure PocketBase collection so subsequent launches skip this step.
2. **Dashboard**
   • Spotify panel – OAuth2 login (+ token refresh) using the [zmb3/spotify] Go client.
   • YouTube panel – OAuth2 login via the [google-api-go-client] YouTube Data API v3 client.
3. **Playlist Mapping Management**
   • List user playlists from each service.
   • Create mappings (Spotify ⇆ YouTube) with options:
     – Sync name (on/off)
     – Sync tracks (on/off)
   • Optionally create a brand-new playlist in the opposite service during mapping.
4. **Scheduled Sync Jobs**
   • PocketBase Job Scheduler analyses mappings at a chosen interval.
   • Generates a work queue to add/remove/rename tracks or titles respecting API rate limits.
   • Tracks skipped items (blacklist with reason + skip counter).
5. **Conflict & Rate-Limit Handling**
   • Separate "analysis" and "execution" jobs coordinated via a shared collection flag to avoid overlap.
6. **Logging & Status**
   • Sync history, per-mapping status, and detailed job progress visible in dashboard.
7. **Single Binary Deployment**
   • Dockerfile produces a minimal image containing the statically-linked PocketBase/Go binary that also serves the built Vite assets.

## 4. Technical Stack
• **Backend**: Go 1.24+, PocketBase framework
  – Libraries: `github.com/zmb3/spotify`, `google.golang.org/api/youtube/v3`, `github.com/samber/lo` (utility), `github.com/rs/zerolog` (logging)
• **Database**: Embedded SQLite (managed by PocketBase)
• **Frontend**: React 19 + TypeScript, Vite, Tailwind CSS, TanStack Router, TanStack Query, Zod
• **Testing**: Vitest (unit/integration tests), Playwright (E2E tests), MSW (API mocking)
• **Build Tooling**: Go modules, Vite (with TypeScript checking), Docker (multi-stage build)
• **Deployment**: Any container runtime (tested on Unraid).  All configuration via environment variables.

## 5. Resources & References
• PocketBase docs – [Framework](https://pocketbase.io/docs/) | [Go Overview](https://pocketbase.io/docs/go-overview) | [Migrations](https://pocketbase.io/docs/go-migrations) | [Routing](https://pocketbase.io/docs/go-routing) | [Database](https://pocketbase.io/docs/go-database) | [Collections](https://pocketbase.io/docs/go-collections/) | [Records](https://pocketbase.io/docs/go-records/) | [Jobs Scheduling](https://pocketbase.io/docs/go-jobs-scheduling/) | [REST API](https://pocketbase.io/docs/api-records/)
• Spotify Web API – [Auth Code Flow](https://developer.spotify.com/documentation/web-api/tutorials/code-flow) | [Playlists Concepts](https://developer.spotify.com/documentation/web-api/concepts/playlists) | [Get Playlist](https://developer.spotify.com/documentation/web-api/reference/get-playlist) | [Get Playlist Tracks](https://developer.spotify.com/documentation/web-api/reference/get-playlists-tracks)
• YouTube Data API v3 – [Registering an application](https://developers.google.com/youtube/registering_an_application) | [Playlists List](https://developers.google.com/youtube/v3/docs/playlists/list) | [Playlists Insert](https://developers.google.com/youtube/v3/docs/playlists/insert) | [PlaylistItems](https://developers.google.com/youtube/v3/docs/playlistItems) | [PlaylistItems List](https://developers.google.com/youtube/v3/docs/playlistItems/list)
• Go SDKs – [zmb3/spotify](https://github.com/zmb3/spotify) | [google-api-go-client](https://github.com/googleapis/google-api-go-client)
• Reference architecture – [longhabit project](https://github.com/s-petr/longhabit)

## 6. Architecture Overview
The service is delivered as a **single statically-linked Go binary** that embeds a PocketBase instance _and_ the compiled React application:

1. **PocketBase Core (Go)**
   • Provides database (SQLite) and REST-ish API surface.  
   • Custom Go routes and event hooks extend PocketBase for OAuth callbacks, playlist retrieval, and job scheduling.  
   • Two dedicated scheduled job types: _analysis_ (detects deltas) and _execution_ (applies changes).
2. **Embedded React Frontend**
   • Built with Vite and placed into the `/pb_public` directory at build time, letting PocketBase serve the static assets automatically.  
   • Communicates with backend exclusively through PocketBase's API endpoints.
3. **Collections**
   • `settings` – singleton record storing Spotify/Google credentials.  
   • `oauth_tokens` – per-provider access & refresh tokens.  
   • `playlists` – normalized playlist metadata (service, playlist_id, title, etc.).  
   • `mappings` – join table defining sync relations & options.  
   • `sync_items` – queue of pending track/name operations with status/attempt counters.  
   • `logs` – sync history & diagnostic messages (TTL configurable).
4. **Jobs & Queue**
   • Each `mappings.interval_minutes` defines how often that pair should sync.  
   • A _global_ interval in the `settings` record (optional) overrides per-mapping values.  
   • Analysis job enqueues work for mappings whose interval timer elapsed.  
   • Execution job processes `sync_items` with exponential back-off when rate-limited.
5. **Local Persistence**
   • All state lives under `pb_data/` making backup & restore trivial.

*(A simple component diagram will be added once drawings are finalized.)*

## 7. Deployment Strategy
• **Multi-stage Dockerfile**  
  1. Build Go/PocketBase binary (`CGO_ENABLED=0 go build -tags embed`)  
  2. Build React assets via Vite (`npm run build`).  
  3. Copy assets into `/app/pb_public` before final `FROM scratch`/`distroless` stage.
• **Entrypoint:** `./app serve --http 0.0.0.0:${PORT:-8090}`
• **Volumes:** Mount `pb_data/` to persist database & uploaded files.
• **Healthcheck:** `GET /api/health` (custom route returning 200).
• **Recommended Runtime:** Docker Compose / Unraid template example will map `8090:8090` and a named volume for `pb_data`.

## 8. Environment Variables & Secrets
| Variable | Required | Description |
|----------|----------|-------------|
| `SPOTIFY_ID` | ✅ | Spotify app client ID |
| `SPOTIFY_SECRET` | ✅ | Spotify app client secret |
| `GOOGLE_CLIENT_ID` | ✅ | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | ✅ | Google OAuth client secret |
| `LOG_LEVEL` | ➖ | `debug`, `info`, `warn`, `error` (default `info`) |

*A `.env.example` file will ship with all of the above. For local development, create a `.env` file with actual credentials - if all required values are present, the app skips the setup wizard and goes directly to the OAuth flow.*

## 9. Non-Functional Requirements / Design Considerations
• **Rate-Limit Resilience:** Detect 429/Quota errors and retry with back-off, never dropping user data.  
• **Performance:** Sync execution should process ~100 track diffs per minute without blocking other API calls.  
• **Security:** All secrets stored in PocketBase are encrypted at rest using PB's built-in record encryption.  TLS is expected to be handled by the user's reverse proxy (e.g., Traefik or Caddy).  
• **Portability:** Runs on x86_64 & ARM64; no external dependencies beyond the binary & SQLite file.
• **Observability:** Zerolog structured logs, optional Sentry integration stubbed for future RFC.
• **Accessibility:** Frontend adheres to WCAG 2.1 AA via Tailwind defaults and focus management utilities.
• **Testing Strategy:** Unit tests via Vitest, E2E tests via Playwright, API mocking via MSW for isolated frontend testing.
• **Agent Tooling:** Implementer agents have access to [Playwright MCP](https://github.com/microsoft/playwright-mcp) for validating UI implementations during RFC development.

## 10. Planned RFC Roadmap
| RFC | Title | Purpose (Key Deliverables) | Highlight Test Cases |
|-----|-------|---------------------------|----------------------|
| **RFC-001** | Repo Bootstrapping & Toolchain | Establish `backend` & `frontend` dirs (mirroring [longhabit]), init Go module, scaffold Vite + Tailwind React app, add Makefile & CI workflow. | • `go test ./...` passes.  • Vite dev server starts (`npm run dev`). |
| **RFC-002** | PocketBase Foundation & Migrations Framework | Wire up PocketBase app (`main.go`), add migration CLI harness, create baseline collections (`settings`). Set up Vitest + Playwright testing infrastructure. | • `go run cmd/server migrate up` creates DB schema.  • `npm test` runs Vitest.  • `npm run test:e2e` runs Playwright. |
| **RFC-003** | Environment Setup Wizard (FE + BE) | FE form wizard; BE route to write `settings` record; skip wizard when `.env` has credentials or no record exists. UI: responsive form with validation. | • Wizard shows when no `.env` + no DB record.  • Form validation via Zod.  • Playwright test covers wizard flow. |
| **RFC-004** | Spotify OAuth Integration | Implement `/auth/spotify/*` routes, store tokens, list playlists. UI: OAuth callback page, playlist display cards, loading states. | • Refresh token stored.  • `GET /api/spotify/playlists` returns >0 items.  • Playwright tests cover OAuth flow. |
| **RFC-005** | YouTube OAuth Integration | Similar flow for Google OAuth + playlist listing. UI: consistent with Spotify flow, error handling. | • Refresh token stored.  • Playwright tests verify YouTube OAuth + playlist listing. |
| **RFC-006** | Playlist Mapping Collections & UI | CRUD endpoints + React pages to create/edit mappings. UI: responsive forms, drag-drop mapping, interval controls. | • Creating a mapping persists `mappings` record; UI reflects list.  • Playwright tests cover mapping CRUD flow. |
| **RFC-007** | Sync Analysis Job | Scheduled detection of playlist diffs, populate `sync_items`. UI: job progress indicators, sync queue visualization. | • After adding a track in Spotify, analysis job queues item.  • Vitest tests cover job scheduling logic. |
| **RFC-008** | Sync Execution Job | Worker processes `sync_items`, updates target service, handles errors. UI: real-time sync status updates, error notifications. | • Queued item marked `done` and track appears on other platform.  • E2E test verifies full sync cycle. |
| **RFC-009** | Conflict & Blacklist Handling | Schema for blacklisted tracks, skip logic, UI to manage blacklist. UI: track conflict resolution modal, blacklist management table. | • Blacklisted track skipped and counter increments.  • Playwright tests cover conflict resolution UI. |
| **RFC-010** | Logging & Status Dashboard | Visualize job runs, per-mapping status, log tail. UI: real-time status cards, log filtering, sync history charts. | • UI shows last N runs with success indicator.  • Playwright tests verify dashboard interactions. |
| **RFC-011** | Docker & Release Pipeline | Multi-stage Dockerfile, GitHub Actions to build/push image, TypeScript type checking in CI. | • `docker run` starts server, exposes PB dashboard at `/_/`.  • `tsc --noEmit` passes in CI. |
| **RFC-012** | Comprehensive E2E Testing Suite | Complete Playwright test suite covering all user flows: setup wizard → OAuth → mappings → sync execution. MSW setup for API mocking. | • Full user journey works end-to-end.  • All Playwright tests pass with 90%+ coverage. |
| **RFC-013** | Documentation & README | Author detailed README, update env descriptions, add architecture diagram, testing strategy docs. | • `markdownlint` passes.  • README includes testing commands and contribution guidelines. |

*(Additional RFCs may be added as scope evolves.)*

## 11. Resources & References  
(Previously Section 5 – renumbered for clarity.)
• PocketBase docs – [Framework](https://pocketbase.io/docs/) | [Go Overview](https://pocketbase.io/docs/go-overview) | [Migrations](https://pocketbase.io/docs/go-migrations) | [Routing](https://pocketbase.io/docs/go-routing) | [Database](https://pocketbase.io/docs/go-database) | [Collections](https://pocketbase.io/docs/go-collections/) | [Records](https://pocketbase.io/docs/go-records/) | [Jobs Scheduling](https://pocketbase.io/docs/go-jobs-scheduling/) | [REST API](https://pocketbase.io/docs/api-records/)  
• Spotify Web API – [Auth Code Flow](https://developer.spotify.com/documentation/web-api/tutorials/code-flow) | [Playlists Concepts](https://developer.spotify.com/documentation/web-api/concepts/playlists) | [Get Playlist](https://developer.spotify.com/documentation/web-api/reference/get-playlist) | [Get Playlist Tracks](https://developer.spotify.com/documentation/web-api/reference/get-playlists-tracks)  
• YouTube Data API v3 – [Registering an application](https://developers.google.com/youtube/registering_an_application) | [Playlists List](https://developers.google.com/youtube/v3/docs/playlists/list) | [Playlists Insert](https://developers.google.com/youtube/v3/docs/playlists/insert) | [PlaylistItems](https://developers.google.com/youtube/v3/docs/playlistItems) | [PlaylistItems List](https://developers.google.com/youtube/v3/docs/playlistItems/list)  
• Go SDKs – [zmb3/spotify](https://github.com/zmb3/spotify) | [google-api-go-client](https://github.com/googleapis/google-api-go-client)  
• Testing Tools – [Vitest](https://vitest.dev/) | [Playwright](https://github.com/microsoft/playwright) | [MSW](https://github.com/mswjs/msw) | [Playwright MCP](https://github.com/microsoft/playwright-mcp)  
• Reference architecture – [longhabit project](https://github.com/s-petr/longhabit)

*(Further sections such as Architecture, RFC list, Deployment strategy, etc., will be added after stakeholder review and approval of the core scope above.)* 