# RFC-003: Environment Setup Wizard (Frontend + Backend)

**Status:** Done  
**Branch:** `rfc/003-setup-wizard`  
**Related Issues:** _n/a_  
**Depends On:** RFC-002 (PocketBase foundation)

---

## 1. Goal

Provide a first-run "Environment Setup Wizard" that collects the four required third-party credentials — `SPOTIFY_ID`, `SPOTIFY_SECRET`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` — and persists them in the `settings` singleton collection created in RFC-002.  When all four values are already present (either via **environment variables** _or_ an existing `settings` record) the wizard is skipped and the app boots directly to the dashboard.

## 2. Background & Context

The product cannot operate until Spotify & Google OAuth credentials are configured.  Self-hosted users will usually pass them via a `.env` file or container env vars, but many will prefer an in-browser guided flow.  Persisting secrets in PocketBase (encrypted at rest¹) lets subsequent launches omit the wizard even if the container restarts.

Design principles:

* **Single-user instance** – no multi-tenant auth required.
* **Idempotent** – running wizard twice should overwrite credentials safely.
* **Secure** – secrets should never be returned once stored (write-only endpoint on BE).
* **Skip Logic** – server sets `X-Setup-Required: true/false` header so FE can decide.

> ¹ PocketBase encrypts field values when a collection is marked `System: true`.  The `settings` collection satisfies this.

## 3. Technical Design

### 3.1 Backend (PocketBase Go Hooks + Routes)

#### 3.1.1 New internal package
```
backend/internal/pbext/setupwizard/
 ├── routes.go   # custom REST endpoint(s)
 └── hooks.go    # enforce write-only semantics
```
`pbapp.SetupApp()` (RFC-002) will call `setupwizard.Register(app)`.

#### 3.1.2 REST Contract
| Method | URL | Auth | Description |
|--------|-----|------|-------------|
| `GET`  | `/api/setup/status` | none | Returns `{ required: boolean }` where `required` = true iff any of the 4 keys missing in env+DB |
| `POST` | `/api/setup` | none | Body: `{ spotify_id, spotify_secret, google_client_id, google_client_secret }` – writes/updates record; returns `204 No Content`. Fails with 409 if setup already completed & ENV locked (see below). |

* _Security_: The POST route is only allowed when **(a)** no credentials exist in DB _and_ env vars are unset **OR** an `UPDATE_ALLOWED=true` env var is present (for credential rotation).
* The route writes to the singleton `settings` record (id =`settings`) via DAO.
* Secrets never echoed back.

##### Implementation sketch (routes.go)
```go
func Register(app *pocketbase.PocketBase) {
    app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
        e.Router.GET("/api/setup/status", statusHandler(app))
        e.Router.POST("/api/setup", postHandler(app))
        return nil
    })
}
```

Hooks (`hooks.go`) ensure `settings` collection cannot be read via generic `/api/collections/settings/records`.

```go
app.OnRecordBeforeListRequest("settings").Add(denyAll)
app.OnRecordBeforeViewRequest("settings").Add(denyAll)
```

### 3.2 Frontend (React 19 + TanStack Router)

#### 3.2.1 Route Tree
```
src/routes/
 ├── __setup.tsx        # loader decides redirect
 ├── setup/             # wizard steps
 │    ├── index.lazy.tsx    # step-1 credentials form
 │    └── success.lazy.tsx  # confirmation
 └── _authenticated/...     # existing private routes
```
* Root loader calls `/api/setup/status`; if `required` is true it renders `__setup` layout, else forwards to `_authenticated` layout.
* Wizard uses **React Hook Form** + **Zod** schema:
```ts
const Schema = z.object({
  spotifyId: z.string().nonempty(),
  spotifySecret: z.string().nonempty(),
  googleClientId: z.string().nonempty(),
  googleClientSecret: z.string().nonempty(),
});
```
* POST to `/api/setup`; on success navigate to `/login` (Spotify OAuth flow will be next RFC).

#### 3.2.2 UI Components
* Use Shadcn/ui `Input`, `Label`, `Card`, `Button`, `Alert`.
* Responsive – single column < 640 px, two columns otherwise.

#### 3.2.0 Router Plugin Installation
TanStack Router v1 now ships a **Vite plugin** that auto-generates the file-based route tree.  Install it as a **dev dependency** and register it _before_ the React plugin inside `vite.config.ts` ([docs](https://tanstack.com/router/latest/docs/framework/react/routing/installation-with-vite)):
```bash
npm install -D @tanstack/router-plugin
```

```ts title="frontend/vite.config.ts"
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';
import { TanStackRouterVite } from '@tanstack/router-plugin/vite';

export default defineConfig({
  plugins: [
    TanStackRouterVite({ target: 'react', autoCodeSplitting: true }),
    react(),
  ],
});
```

### 3.3 Validation & Edge Cases
* **Env Override**: If the server finds env vars set it returns `required:false`; wizard never shows.
* **Concurrent Calls**: Second POST while first still processing returns 409.
* **Field Encryption**: Fields stored exactly as env variable names for clarity; PB encryption at rest.

### 3.4 Makefile / Dev Flow
* `make backend-dev` already starts PB which exposes the new routes.
* Frontend dev server proxy (`vite.config.ts`) already forwards `/api/*` to 8090 – no extra config.

### 3.5 Tests
* **Backend** – Go test `setupwizard_test.go` covering:
  * status endpoint before/after setup
  * successful POST writes record
  * POST forbidden when env vars present.
* **Frontend** – Vitest component test verifying Zod validation; Playwright E2E with MSW for mocking api calls.

## 4. Dependencies
* **Backend**: none (relies on PocketBase & DAO API)
* **Frontend**:
  * `react-hook-form@latest` – form state
  * `@hookform/resolvers@latest` – Zod integration
  * `zod@latest` – validation (already in stack)

## 5. Checklist
- [X] **W1** Implement `setupwizard` routes & hooks; register in `pbapp`.
- [X] **W2** Deny list/view access to `settings` via collection rules or hooks.
- [X] **W3** Migration: ensure a singleton `settings` record exists (id `settings`).
- [X] **W4** Implement frontend route guard & wizard pages with validation.
- [X] **W5** Add Vitest unit tests for Zod schema and Playwright E2E with MSW for mocking api calls.
- [X] **W6** Add Go tests for status & post routes.
- [X] **W7** Update README with first-run instructions.

## 6. Definition of Done
* Fresh clone → `make dev` opens browser, wizard prompts for keys, saving succeeds, redirect happens, subsequent reload skips wizard.
* If `.env` provides all keys, wizard never displays.
* Backend tests green, frontend unit tests green.

## Implementation Notes / Summary
* We considered using the generic PocketBase REST API (`/api/collections/settings/records/settings`), but a dedicated route keeps secrets out of generic listing endpoints and allows stricter auth.
* Future RFC-004 (Spotify OAuth) requires these secrets; wizard must complete first.
* Migration ensures there is always exactly one `settings` record, simplifying DAO operations.

**W1 Implementation Details (Completed):**
- Created `backend/internal/pbext/setupwizard/routes.go` - implements GET `/api/setup/status` and POST `/api/setup` endpoints
  - `isSetupRequired()` function checks both environment variables and database for existing credentials
  - `statusHandler()` returns `{required: boolean}` indicating if setup is needed
  - `postHandler()` validates and saves all four credentials (spotify_id, spotify_secret, google_client_id, google_client_secret)
  - Supports `UPDATE_ALLOWED=true` environment variable for credential rotation
  - Returns HTTP 409 if setup already completed and updates not allowed
- Created `backend/internal/pbext/setupwizard/hooks.go` - prevents direct API access to settings collection
  - Blocks listing, viewing, creating, updating, and deleting settings records via generic PocketBase API
  - Forces all settings operations to go through the dedicated setup wizard endpoints
- Updated `backend/cmd/server/main.go` - registered setupwizard package with PocketBase app
  - Added import for setupwizard package
  - Called `setupwizard.Register(app)` to register routes
  - Called `setupwizard.RegisterHooks(app)` to register access denial hooks
- Backend successfully builds and compiles with new setupwizard implementation

**W2 Implementation Details (Completed):**
- Access denial was already implemented in `backend/internal/pbext/setupwizard/hooks.go` during W1
- `RegisterHooks()` function blocks all direct API access to settings collection:
  - `OnRecordsListRequest("settings")` - prevents listing settings records
  - `OnRecordViewRequest("settings")` - prevents viewing individual settings records  
  - `OnRecordBeforeCreateRequest("settings")` - prevents creating settings via API
  - `OnRecordBeforeUpdateRequest("settings")` - prevents updating settings via API
  - `OnRecordBeforeDeleteRequest("settings")` - prevents deleting settings via API
- All operations return HTTP 403 Forbidden with descriptive error messages
- This ensures settings can only be managed through the dedicated setup wizard endpoints

**W3 Implementation Details (Completed):**
- Created `backend/pb_migrations/1660000001_create_settings_singleton.go` - ensures singleton settings record exists
- Migration checks if settings record with id "settings" already exists before creating
- If record doesn't exist, creates new record with id "settings" and empty credential fields
- Record will be populated by setup wizard when user provides credentials
- Migration includes proper rollback functionality to delete the singleton record
- Backend successfully builds and compiles with new migration

**W4 Implementation Details (Completed):**
- Installed required dependencies: `@tanstack/router-plugin`, `react-hook-form`, `@hookform/resolvers`
- Updated `frontend/vite.config.ts` - added TanStack Router plugin and API proxy to localhost:8090
- Created `frontend/src/routes/__root.tsx` - root route with setup status check and redirect logic
- Created `frontend/src/routes/setup.lazy.tsx` - setup layout route component
- Created `frontend/src/routes/setup/index.lazy.tsx` - main setup wizard form with:
  - React Hook Form integration for form state management
  - Zod schema validation for all four credential fields (spotify_id, spotify_secret, google_client_id, google_client_secret)
  - Responsive UI with Tailwind CSS styling
  - Error handling and loading states
  - Form submission to POST /api/setup endpoint
- Created `frontend/src/routes/setup/success.lazy.tsx` - success confirmation page with auto-redirect
- Created `frontend/src/routes/dashboard.lazy.tsx` - basic dashboard placeholder
- Updated `frontend/src/main.tsx` - integrated TanStack Router with RouterProvider
- TanStack Router plugin automatically generated `frontend/src/routeTree.gen.ts` with proper type safety
- Frontend successfully builds and compiles with complete setup wizard flow

**W5 Implementation Details (Completed):**
- Installed testing dependencies: `msw` for API mocking, `@playwright/test` for E2E testing
- Created `frontend/src/test/setup-schema.test.ts` - comprehensive Vitest unit tests for Zod schema:
  - Tests valid credential validation
  - Tests individual field validation (empty spotify_id, spotify_secret, google_client_id, google_client_secret)
  - Tests missing fields validation
  - Tests all-empty fields validation
  - All 7 unit tests pass successfully
- Created `frontend/e2e/setup-wizard.spec.ts` - Playwright E2E test with MSW API mocking:
  - Tests setup wizard display when setup is required
  - Tests form validation for required fields
  - Tests successful credential submission flow
  - Tests API error handling
  - Tests redirect behavior when setup is not required
  - Tests loading state during form submission
  - Uses MSW to mock GET /api/setup/status and POST /api/setup endpoints
- Updated `frontend/vitest.config.ts` to exclude e2e directory from Vitest test runs
- All Vitest unit tests pass successfully (7/7)

**W6 Implementation Details (Completed):**
- Created `backend/internal/pbext/setupwizard/routes_test.go` - comprehensive Go unit tests:
  - `TestSetupRequestValidation` - tests validation logic for all credential fields
  - `TestEnvironmentVariableChecking` - tests environment variable detection logic
  - `TestUpdateAllowedFlag` - tests UPDATE_ALLOWED environment variable behavior
- Tests cover validation scenarios including:
  - Valid credential requests
  - Missing individual fields (spotify_id, spotify_secret, google_client_id, google_client_secret)
  - All fields empty
  - Environment variable presence/absence detection
  - UPDATE_ALLOWED flag behavior with various values
- All Go tests pass successfully
- Tests use testify/assert for clean assertions and proper test structure

**W7 Implementation Details (Completed):**
- Updated `README.md` with comprehensive first-run setup instructions
- Added section 4 "First-run setup" with detailed wizard flow explanation
- Documented OAuth credential requirements for Spotify and Google
- Included environment variable alternative with specific variable names
- Updated Development Status section to reflect RFC-003 completion
- Added TanStack Router and test coverage implementation notes
- Documented automatic wizard skip behavior when environment variables are present

## Resources & References
* PocketBase DAO example – https://pocketbase.io/docs/go-records/
* Zod validation – https://github.com/colinhacks/zod
* React Hook Form docs – https://react-hook-form.com/
* Shadcn/ui – https://ui.shadcn.com/
* Air live reload – https://github.com/air-verse/air
* TanStack Router – Vite installation guide – https://tanstack.com/router/latest/docs/framework/react/routing/installation-with-vite

---

*End of RFC-003* 