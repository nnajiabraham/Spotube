# RFC-004: Spotify OAuth Integration

**Status:** Draft  
**Branch:** `rfc/004-spotify-oauth`  
**Related Issues:** _n/a_  
**Depends On:**
* RFC-002 (PocketBase foundation & migrations)
* RFC-003 (Environment Setup Wizard – ensures Spotify client credentials exist)

---

## 1. Goal

Enable users to authenticate their Spotify account, store long-lived **refresh tokens** in PocketBase, and list their playlists via a custom REST endpoint.  This unlocks later RFCs that create playlist mappings and sync jobs.

## 2. Background & Context

We will use the [Authorization Code Flow with PKCE](https://developer.spotify.com/documentation/web-api/tutorials/code-flow) because the app runs in a browser and we prefer not to embed client secret in the frontend.  However, since this is a self-hosted single-user service we _do_ control a backend that can hold the secret; both flows are possible.  For security and simplified CORS we will implement:

* **Frontend** opens `/api/auth/spotify/login` → server redirects to Spotify authorize URL with `code_challenge_method=S256`.
* On callback `/api/auth/spotify/callback` server exchanges code + verifier for access & refresh tokens using **client secret stored server-side**.
* Tokens persisted in PocketBase `oauth_tokens` collection (schema defined below).
* Frontend receives 302 to `/dashboard` with `?spotify=connected` toast.

We leverage Go library [`github.com/zmb3/spotify`](https://github.com/zmb3/spotify) (MIT) which already supports PKCE helpers.

## 3. Technical Design

### 3.1 New Collection: `oauth_tokens`
Migration file `pb_migrations/1670000000_create_oauth_tokens.go`
| field | type | notes |
|-------|------|-------|
| `provider` | `select` (`spotify`, `google`) | required, unique with user id (future-proof) |
| `access_token` | `text` | encrypted at rest |
| `refresh_token` | `text` | encrypted |
| `expiry` | `date` | token expiry time |
| `scopes` | `text` | space-separated |

Collection rules: only server hooks can list/view; deny all client requests.

### 3.2 Backend Routes (under `backend/internal/pbext/spotifyauth`)
| Method | URL | Description |
|--------|-----|-------------|
| `GET` | `/api/auth/spotify/login` | Redirects to Spotify auth URL, caches PKCE verifier in HTTP-only cookie (5 min TTL). |
| `GET` | `/api/auth/spotify/callback` | Exchanges code; stores tokens; redirects to frontend. |
| `GET` | `/api/spotify/playlists` | Proxy endpoint that calls Spotify Web API `/me/playlists` with stored token, handles refresh. |

#### 3.2.1 PKCE State Storage
* Generate `state` = random 16 bytes base64 – include in redirect and store in same cookie as verifier.
* Callback validates state equality.

#### 3.2.2 Token Persistence Helper
```go
func saveSpotifyTokens(dao *daos.Dao, at *oauth2.Token, scopes []string) error {
    rec, _ := dao.FindFirstRecordByFilter("oauth_tokens", "provider = 'spotify'", nil)
    if rec == nil {
        rec = models.NewRecord(coll)
        rec.Set("provider", "spotify")
    }
    rec.Set("access_token", at.AccessToken)
    rec.Set("refresh_token", at.RefreshToken)
    rec.Set("expiry", at.Expiry)
    rec.Set("scopes", strings.Join(scopes, " "))
    return dao.SaveRecord(rec)
}
```

#### 3.2.3 Refresh Middleware
Create helper `withSpotifyClient(c echo.Context) (*spotify.Client, error)` that:
1. Loads token record.
2. If expired (or within 30 seconds), refresh via `oauth2.Config.TokenSource`.
3. Saves new tokens.
4. Returns authenticated client.

### 3.3 Frontend Changes
* Add **"Connect Spotify"** card on `/dashboard` when token missing.
* Clicking calls `/api/auth/spotify/login` (via `window.location.href`).
* After callback redirect, FE shows toast using query param.
* Playlist list component (`/settings/spotify`) fetches `/api/spotify/playlists` – shows name, track count.

### 3.4 Environment & Redirect URIs
* Spotify dashboard App → Redirect URI: `http://localhost:8090/api/auth/spotify/callback` (dev) and `${PUBLIC_URL}/api/auth/spotify/callback` (prod behind reverse proxy).
* Expose env var `PUBLIC_URL` (defaults to `http://localhost:8090`). Wizard already collected client ID/secret.

### 3.5 Makefile Updates
No changes; routes will hot-reload with Air.

### 3.6 Tests
* **Backend** – Go tests using `httptest` + `github.com/jarcoal/httpmock` to stub Spotify token and playlist endpoints.  No traffic leaves the test runner.
* **Frontend**
  * **Vitest** unit tests for UI states (connected / disconnected) with **MSW** (`mswjs/browser`) intercepting `/api/setup/status`, `/api/auth/spotify/*`, and `/api/spotify/playlists`.
  * **Playwright** E2E: launch dev server with MSW enabled to mock backend responses, verify redirect → toast flow, playlist listing.

## 4. Dependencies
* `github.com/zmb3/spotify/v2` – MIT (PKCE helpers)
* `golang.org/x/oauth2` (transitive)
* **Backend Test:** `github.com/jarcoal/httpmock`
* **Frontend Test:** `msw@latest` – request mocking across Vitest & Playwright

## 5. Checklist
- [ ] **S1** Add migration for `oauth_tokens` collection.
- [ ] **S2** Implement `spotifyauth` routes & PKCE cookie handling.
- [ ] **S3** Helper to refresh & persist tokens.
- [ ] **S4** Implement `/api/spotify/playlists` proxy.
- [ ] **S5** Deny client access to `oauth_tokens` collection.
- [ ] **S6** Frontend: dashboard card + playlist page with MSW mocks.
- [ ] **S7** Backend tests for callback & refresh (httpmock); FE Vitest + Playwright tests with MSW.
- [ ] **S8** Update README with Spotify setup & redirect URI note.

## 6. Definition of Done
* User can click "Connect Spotify", complete consent, return, see playlists.
* Refresh token stored; subsequent API calls succeed without re-auth.
* Token auto-refresh persists new expiry.
* Backend & FE tests green.

## Implementation Notes / Summary
* Chose PKCE over implicit flow for increased security even though backend holds secret.
* PKCE verifier stored in HTTP-only cookie (safer than session store for single-user).
* In production behind reverse proxy, callback path preserved at `/api/auth/spotify/callback`.

## Resources & References
* Spotify Auth Code Flow – https://developer.spotify.com/documentation/web-api/tutorials/code-flow
* zmb3/spotify library – https://github.com/zmb3/spotify
* OAuth2 PKCE RFC – https://www.rfc-editor.org/rfc/rfc7636
* TanStack Router installation – https://tanstack.com/router/latest/docs/framework/react/routing/installation-with-vite  
* httpmock – https://github.com/jarcoal/httpmock  
* MSW (Mock Service Worker) – https://mswjs.io/  
* Vitest – https://vitest.dev/  
* Playwright – https://playwright.dev/

---

*End of RFC-004* 