# Local Agent Handoff (PR #4)

## Purpose

This note is the bootstrap context for continuing work locally with an agent.  
Scope is specifically: run app locally, validate real OAuth/browser behavior, and fix issues already identified.

## Canonical Repo State

- **Branch:** `cursor/cleanup-auth-playlists-370e`
- **PR:** https://github.com/nnajiabraham/Spotube/pull/4
- **Base:** `master`
- **Latest commit:** `70500ac` (`Simplify shell and enrich OAuth playlist flows`)

If your local checkout differs, first sync to this branch/commit before further debugging.

---

## What is already implemented on this branch

### Security and auth contract fixes

1. **Session signing key fix**
   - Added dedicated `SESSION_SECRET` config and env handling.
   - Session store now uses secret key, not cookie name.
   - Cookie options include `SameSite=Lax`.

2. **OAuth callback redirect target fix**
   - Backend callbacks now redirect to **frontend dashboard** (not backend path):
     - Spotify: `${FRONTEND_URL}/dashboard?spotify=connected`
     - YouTube: `${FRONTEND_URL}/dashboard?youtube=connected`
   - Safe URL builder added with fallback behavior:
     - `backend/internal/handlers/oauth_redirect.go`
     - `backend/internal/handlers/oauth_redirect_test.go`

3. **Read-only sync item endpoint added**
   - `GET /api/collections/sync_items/records/:id` implemented for logs modal usage.

### Frontend/backend contract cleanup

4. **Legacy PocketBase artifact removal**
   - Removed `frontend/src/lib/pocketbase.ts`
   - Removed `pocketbase` dependency
   - Updated tests/mocks accordingly.

5. **Playlist payload enrichment from backend**
   - Spotify payload now includes `description`, `images`, `track_count`, `public`, `owner`.
   - YouTube payload now includes `name`, `title`, `description`, `itemCount` (via `contentDetails`).

6. **App shell/nav simplification for MVP flow**
   - Shared nav in root route:
     - Dashboard, Mappings, Spotify, YouTube
     - Logs retained but labeled **"Logs (advanced)"**
   - Setup routes hide nav.

7. **Dead code/env cleanup**
   - Deleted unused `frontend/src/components/SpotifyPlaylists.tsx`
   - Removed root `.env.example` to keep backend env as single source (`backend/env.example`)

---

## Validation already completed (and exact scope)

### Build/unit/integration checks (passed)

- `make backend/lint`
- `LOG_LEVEL= make backend/test`
- Frontend:
  - `npm run lint`
  - `npm run test`
  - `npm run build`

### Runtime OAuth checks completed on cloud VM

- FE and BE run on split origins (frontend port `5173`, backend port `8090`).
- Browser-level navigation reaches providers:
  - Spotify login redirects to `accounts.spotify.com`
  - YouTube login redirects to `accounts.google.com`
- OAuth state cookies are set before redirect (`spotify_oauth`, `youtube_oauth`, `HttpOnly`, `SameSite=Lax`).
- Callback error behavior verified:
  - Invalid state => `401 state mismatch`
  - Fake code with valid state => `502 token exchange failed`

### Not fully validated yet

- Full provider round-trip success for both providers in a real browser with actual login/consent.
- Final UX confirmation after callback on local machine (toast/user-visible state + playlist visibility).

---

## Known blockers and high-signal findings

1. **YouTube OAuth currently blocked by Google `redirect_uri_mismatch`**
   - Observed on callback URI variants derived from backend public URL and host variants.
   - Fix is in Google Cloud OAuth client config (Authorized redirect URIs must exactly match backend callback URI).

2. **OAuth success UX currently logs to console**
   - Dashboard currently prints success/error via `console.log`/`console.error` for query params.
   - Consider replacing with visible toast/banner for local/manual testing clarity.

3. **Sync engine remains intentionally out of current scope**
   - Analysis/executor/scheduler hardening is not completed in this branch.
   - Current work focuses on auth + playlist visibility + simplification.

---

## Local bootstrap checklist

### 1) Toolchain

- Go **1.24.2** (repo expectation)
- Node **20.12.2** (from `.nvmrc`)
- Make

### 2) Environment

From repo root:

```bash
cp backend/env.example backend/.env
```

Set/verify at minimum in `backend/.env`:

- `PUBLIC_URL=<backend-public-base-url>`
- `FRONTEND_URL=<frontend-public-base-url>`
- `CORS_ALLOW_ORIGINS=<frontend-origin>`
- `SPOTIFY_CLIENT_ID=...`
- `SPOTIFY_CLIENT_SECRET=...`
- `GOOGLE_CLIENT_ID=...`
- `GOOGLE_CLIENT_SECRET=...`
- `SESSION_SECRET=<long-random-secret>`

### 3) OAuth provider console settings

#### Spotify app

Add Authorized Redirect URI:

- `<backend-public-base-url>/api/auth/spotify/callback`

#### Google OAuth client

Add Authorized Redirect URI(s):

- `<backend-public-base-url>/api/auth/youtube/callback`
- Optional host variant callback if you switch hostnames during local testing.

### 4) Install + run

```bash
make install
make backend/dev
make frontend/dev
```

Open your configured frontend URL.

---

## Manual verification script (local)

1. Load `/dashboard`.
2. Click **Connect Spotify**.
3. Complete provider login/consent.
4. Confirm callback lands on frontend dashboard URL with `?spotify=connected`.
5. Verify:
   - dashboard connection card reflects connected state
   - `/settings/spotify` shows playlists and enriched fields (cover, counts, owner/public where available)
6. Repeat for YouTube with `?youtube=connected`.
7. Verify `/settings/youtube` shows title/description/itemCount values.

If failure occurs, capture:

- browser URL after callback
- backend log lines around `/api/auth/*/callback`
- response payload and status for `/api/auth/*/playlists`

---

## Next priorities for the local continuation session

1. **Close YouTube redirect mismatch**
   - Expected outcome: provider returns to `/api/auth/youtube/callback` without mismatch page.

2. **Complete true E2E auth success verification**
   - Expected outcome: both providers show connected state and playlist pages populated.

3. **Improve OAuth success/error UX**
   - Replace console-only messages with visible user feedback.

4. **Stabilize/expand Playwright E2E for real flows where possible**
   - Keep provider-login segments manual; automate pre/post callback assertions.

---

## Suggested bootstrap prompt for your local coding agent

Use this verbatim if helpful:

> Continue from branch `cursor/cleanup-auth-playlists-370e` (PR #4). Read `docs/handoffs/LOCAL_AGENT_HANDOFF_PR4.md` and `docs/rfcs/RFC-101-local-continuation-and-oauth-e2e-hardening.md` first. Validate true local OAuth round-trip for Spotify and YouTube on split origins (frontend port `5173`, backend port `8090`), fix any callback/redirect/playlists visibility issues, and keep sync-engine work deferred. Commit in small logical units, run backend/frontend tests after each fix, and update PR #4.
