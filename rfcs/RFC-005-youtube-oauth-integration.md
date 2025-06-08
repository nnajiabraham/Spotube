# RFC-005: YouTube OAuth Integration

**Status:** Done  
**Branch:** `rfc/005-youtube-oauth`  
**Related Issues:** _n/a_  
**Depends On:**
* RFC-002 (PocketBase foundation & migrations)
* RFC-003 (Environment Setup Wizard – Google credentials present)
* RFC-004 (Spotify OAuth integration pattern & utilities)

---

## 1. Goal

Allow the user to connect their **YouTube Music** (Google) account, persist refresh tokens, and fetch the user's playlists via a simple REST endpoint.  This mirrors RFC-004's Spotify flow so that later sync logic can query both services.

## 2. Background & Context

Google APIs use **OAuth2 Authorization Code Flow** with optional PKCE.  Unlike Spotify, Google _requires_ HTTPS redirect URIs in production but permits `http://localhost` for dev.  We'll again offload token handling to the backend so the React app never sees client secrets.

Key docs:
* Google OAuth – <https://developers.google.com/identity/protocols/oauth2>
* YouTube Data API v3 playlists – <https://developers.google.com/youtube/v3/docs/playlists/list>
* Official Go client – <https://pkg.go.dev/google.golang.org/api/youtube/v3>

## 3. Technical Design

### 3.1 Re-use `oauth_tokens` Collection
The migration from RFC-004 already created a generic collection with `provider` select.  We will store
```
provider = "google"
access_token, refresh_token, expiry, scopes
```
*Rule*: Only one record per provider.

### 3.2 Backend Implementation (`backend/internal/pbext/googleauth`)

#### 3.2.1 Routes
| Method | URL | Description |
|--------|-----|-------------|
| `GET` | `/api/auth/google/login` | Redirect to Google consent screen. Stores PKCE verifier + state in HTTP-only cookie. |
| `GET` | `/api/auth/google/callback` | Exchanges code, saves tokens, redirects to `/dashboard?youtube=connected`. |
| `GET` | `/api/youtube/playlists` | Returns user playlists (id, title, itemCount). |

Return shape similar to Spotify endpoint for FE parity.

#### 3.2.2 OAuth Scopes & Config
* Scopes: `https://www.googleapis.com/auth/youtube.readonly`
* ClientID/Secret pulled from `settings` collection or env vars.
* Redirect URI assembled from `${PUBLIC_URL}/api/auth/google/callback`.
* Use `golang.org/x/oauth2` + `google.golang.org/api/option` to build YouTube service.

#### 3.2.3 Token Refresh Helper
Similar to Spotify helper but cached under key `google`.

#### 3.2.4 Playlist Fetch
```go
svc, err := youtube.NewService(ctx, option.WithTokenSource(ts))
call := svc.Playlists.List([]string{"id","snippet","contentDetails"}).Mine(true).MaxResults(50)
resp, _ := call.Do()
```
Map to JSON response `{ id, title, itemCount }`.

### 3.3 Frontend Updates
* Dashboard shows **"Connect YouTube"** card when Google token missing.
* Upon connect, toast appears and `/settings/youtube` page lists playlists.
* Components parallel Spotify ones for consistency.

### 3.4 Testing Strategy

#### 3.4.1 Backend
* `jarcoal/httpmock` to stub:
  * `https://oauth2.googleapis.com/token` (exchange & refresh)
  * `https://youtube.googleapis.com/youtube/v3/playlists` list call
* Validate:
  * Callback saves tokens.
  * Refresh path persists new expiry.
  * Playlist endpoint returns mapped JSON.

#### 3.4.2 Frontend
* **MSW** handlers for:
  * `/api/auth/google/login` (responds with 302 but we'll just assert navigation)
  * `/api/youtube/playlists` sample payload.
* **Vitest** for component states.
* **Playwright** E2E with MSW to simulate full connect flow.

### 3.5 Dependencies
* `google.golang.org/api/youtube/v3` – Apache-2.0
* `golang.org/x/oauth2/google` – for Config helper
* **Backend Test:** `github.com/jarcoal/httpmock`
* **Frontend Test:** `msw`, `vitest`, `playwright` (already in repo)

### 3.6 Checklist
- [X] **G1** Implement `googleauth` routes (login, callback, playlists).
- [X] **G2** Update helper for token storage & refresh.
- [X] **G3** Frontend dashboard card + playlists page.
- [X] **G4** Backend tests with httpmock.
- [X] **G5** FE Vitest + Playwright tests with MSW.
- [X] **G6** README update: Google Cloud OAuth setup & redirect URI instructions.

## 4. Definition of Done
* User can link YouTube account, return, view playlists.
* Refresh token stored and used automatically.
* Tests pass.

## 5. Implementation Notes / Summary
* Google requires **consent screen** publishing; document in README.
* PKCE recommended but optional with backend secret; we reuse PKCE util for symmetry.
* For quota efficiency playlist endpoint caches response in memory for 60 s (simple `sync.Map`); future RFCs may add Redis.

**G1 & G2 COMPLETED** - Implemented Google OAuth routes and token handling:
* Created new package `backend/internal/pbext/googleauth` for all Google/YouTube related authentication logic.
* Added dependencies `google.golang.org/api/youtube/v3` and `golang.org/x/oauth2`.
* Implemented three routes in `googleauth.go`:
  - `GET /api/auth/google/login`: Initiates OAuth2 flow with PKCE. Generates and stores state and PKCE verifier in a secure, HTTP-only cookie (`google_auth_state`). Redirects user to the Google consent screen.
  - `GET /api/auth/google/callback`: Handles the callback from Google. Validates the `state`, exchanges the `code` for an OAuth token using the `verifier` from the cookie, and securely stores the token. Redirects user to `/dashboard?youtube=connected` on success.
  - `GET /api/youtube/playlists`: Fetches the user's YouTube playlists. It uses the `withGoogleClient` helper to ensure a valid, refreshed token is used for the API call.
* Implemented `saveGoogleTokens` helper to persist the `oauth2.Token` into the `oauth_tokens` collection with `provider = 'google'`. It reuses the collection created in RFC-004.
* Implemented `withGoogleClient` helper to provide an authenticated YouTube service client (`*youtube.Service`). This function handles loading the token from the database, checking for expiry, and automatically refreshing it using the refresh token if needed. The refreshed token is then saved back to the database.
* Registered the new routes in `backend/cmd/server/main.go` by calling `googleauth.Register(app)`.
* The implementation mirrors the existing `spotifyauth` flow for consistency, using environment variables (`GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`) for credentials.

**G3 COMPLETED** - Frontend dashboard card + playlists page:
* Added `YouTubePlaylist` and `YouTubePlaylistsResponse` interfaces to `frontend/src/lib/pocketbase.ts` for type safety.
* Added `getYouTubePlaylists` method to `frontend/src/lib/api.ts` to communicate with the new backend endpoint.
* Created a new `YoutubeLogo.tsx` component for the YouTube icon.
* Created the `YoutubeConnectionCard.tsx` component, which uses the `getYouTubePlaylists` endpoint to check connection status. It displays a "Connect" button or a "View Playlists" link depending on whether the user is authenticated.
* Added the `YoutubeConnectionCard` to the dashboard grid in `frontend/src/routes/dashboard.lazy.tsx`.
* Added logic to the dashboard to handle `youtube=connected` and `youtube=error` query parameters from the OAuth callback for user feedback.
* Created a new lazy-loaded route at `frontend/src/routes/settings/youtube.lazy.tsx` for viewing playlists.
* The route component, `YouTubePlaylistsComponent`, fetches and displays the user's YouTube playlists in a simple grid layout.

## Resources & References
* Google OAuth 2.0 – https://developers.google.com/identity/protocols/oauth2
* YouTube Data API playlists – https://developers.google.com/youtube/v3/docs/playlists/list
* Google Go client – https://pkg.go.dev/google.golang.org/api/youtube/v3
* httpmock – https://github.com/jarcoal/httpmock
* MSW – https://mswjs.io/
* Vitest – https://vitest.dev/
* Playwright – https://playwright.dev/

---

*End of RFC-005* 