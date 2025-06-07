# RFC-003: Environment Setup Wizard (Frontend + Backend)

**Status:** Draft  
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
- [ ] **W1** Implement `setupwizard` routes & hooks; register in `pbapp`.
- [ ] **W2** Deny list/view access to `settings` via collection rules or hooks.
- [ ] **W3** Migration: ensure a singleton `settings` record exists (id `settings`).
- [ ] **W4** Implement frontend route guard & wizard pages with validation.
- [ ] **W5** Add Vitest unit tests for Zod schema and Playwright E2E with MSW for mocking api calls.
- [ ] **W6** Add Go tests for status & post routes.
- [ ] **W7** Update README with first-run instructions.

## 6. Definition of Done
* Fresh clone → `make dev` opens browser, wizard prompts for keys, saving succeeds, redirect happens, subsequent reload skips wizard.
* If `.env` provides all keys, wizard never displays.
* Backend tests green, frontend unit tests green.

## Implementation Notes / Summary
* We considered using the generic PocketBase REST API (`/api/collections/settings/records/settings`), but a dedicated route keeps secrets out of generic listing endpoints and allows stricter auth.
* Future RFC-004 (Spotify OAuth) requires these secrets; wizard must complete first.
* Migration ensures there is always exactly one `settings` record, simplifying DAO operations.

## Resources & References
* PocketBase DAO example – https://pocketbase.io/docs/go-records/
* Zod validation – https://github.com/colinhacks/zod
* React Hook Form docs – https://react-hook-form.com/
* Shadcn/ui – https://ui.shadcn.com/
* Air live reload – https://github.com/air-verse/air
* TanStack Router – Vite installation guide – https://tanstack.com/router/latest/docs/framework/react/routing/installation-with-vite

---

*End of RFC-003* 