# RFC-101: Local Continuation Plan for OAuth E2E Hardening and MVP Simplification

**Status:** In Progress  
**Branch:** `cursor/cleanup-auth-playlists-370e`  
**PR:** https://github.com/nnajiabraham/Spotube/pull/4  
**Depends On:** RFC-100 migration baseline, current PR #4 changes

---

## 1. Goal

Provide a single, execution-ready plan for local continuation after PR #4 so development can:

1. validate real browser OAuth round-trip behavior across split origins,
2. resolve remaining auth/provider integration issues,
3. continue MVP simplification work without reintroducing migration-era complexity.

This RFC is intentionally operational: it records what is already done, what was debated, what was validated, what remains, and how to execute next.

---

## 2. Context Summary

The project has already migrated from PocketBase to Echo + SQLite + React/Vite, with working CRUD surfaces for mappings/blacklist/logs and setup/auth plumbing. Current focus is not sync-engine completion; it is a clean, stable MVP for:

- connecting Spotify + YouTube,
- viewing playlists,
- creating/editing playlist mappings.

During this phase, multiple contract and UX issues were fixed in PR #4 (detailed below), and runtime validation exposed a remaining high-priority provider configuration blocker (`redirect_uri_mismatch` on Google OAuth).

---

## 3. Alternative Research Notes and Decisions

This section captures comparisons against alternative analysis and what was adopted vs rejected.

### 3.1 Adopted from alternative analysis

- Add missing `GET /api/collections/sync_items/records/:id` endpoint for logs modal contract parity.
- Remove dead frontend CSRF fallback call (`/api/csrf`) that backend does not implement.
- Keep security fix to session secret handling (dedicated secret key, not cookie name).

### 3.2 Adapted (same problem, improved implementation)

- **OAuth callback target**:
  - Rejected bare backend-relative `/dashboard` redirect.
  - Implemented redirect to `${FRONTEND_URL}/dashboard?...` with safe URL builder and fallback behavior.

- **Playlist data strategy**:
  - Rejected "minimal backend payload + degraded frontend placeholders".
  - Implemented backend enrichment so UI can render intended playlist cards (images/counts/owner where available).

### 3.3 Explicitly not adopted (for this phase)

- Large route renames or broad API path rewrites (`/api/collections/...` to new route families) during active stabilization.
- Removing logs/blacklist pages entirely; kept them accessible but de-emphasized as advanced.
- Sync-engine completion in same iteration as auth/playlist stabilization.

---

## 4. Implemented Checklist (PR #4 and linked changes)

## 4.1 Completed items

- [x] Remove legacy PocketBase frontend artifact and dependency.
- [x] Introduce `SESSION_SECRET` and use it for session signing.
- [x] Add `SameSite=Lax` cookie behavior for session cookies.
- [x] Add read-only sync item detail endpoint for logs modal.
- [x] Implement frontend-target callback redirects for OAuth success:
  - Spotify -> `${FRONTEND_URL}/dashboard?spotify=connected`
  - YouTube -> `${FRONTEND_URL}/dashboard?youtube=connected`
- [x] Add redirect URL builder tests for callback target composition and fallback.
- [x] Enrich Spotify playlist payload (`description`, `images`, `track_count`, `public`, `owner`).
- [x] Enrich YouTube playlist payload (`title`, `description`, `itemCount`).
- [x] Add shared app shell navigation and de-emphasize logs as advanced.
- [x] Remove dead `SpotifyPlaylists.tsx` component.
- [x] Consolidate env example source-of-truth by removing root `.env.example`.
- [x] Validate backend + frontend lint/test/build suites on cloud runner.

## 4.2 Still pending

- [ ] Full real-provider OAuth success verification end-to-end for both providers on local machine.
- [ ] Resolve Google `redirect_uri_mismatch` in OAuth client configuration (or app config drift causing URI mismatch).
- [ ] Replace dashboard console-only auth success/error signaling with visible UI feedback.
- [ ] Confirm provider-connected state and playlist rendering from real tokens (not mocked responses).

---

## 5. Runtime Validation Findings (what is proven vs not)

## 5.1 Proven

- Split-origin app boot works (`frontend:5173`, `backend:8090`).
- OAuth login endpoints redirect browser to provider domains.
- OAuth state/session cookies are set before provider redirect.
- Callback error-path behavior is correct for:
  - state mismatch -> `401`
  - invalid code exchange -> `502 token exchange failed`

## 5.2 Not yet proven

- Successful provider code exchange for both providers in local browser session.
- Final callback UX + connected card update + playlist visibility in same manual run.

## 5.3 High-priority observed blocker

- Google OAuth returns `redirect_uri_mismatch` for attempted callback URIs.  
  This blocks true YouTube success round-trip until OAuth console configuration is corrected.

---

## 6. Local Execution Plan

## Phase A: Environment and provider alignment

1. Verify backend env:
   - `PUBLIC_URL=http://localhost:8090`
   - `FRONTEND_URL=http://localhost:5173`
2. Verify Google/Spotify OAuth redirect URIs include exact callback values used by backend.
3. Confirm local host variant consistency (`localhost` vs `127.0.0.1`).

**Acceptance criteria:**
- OAuth provider no longer shows redirect mismatch error page.

## Phase B: True local E2E OAuth verification

1. Start app on split origins.
2. Complete real Spotify login/consent; verify dashboard callback query + connected state + playlist page data.
3. Complete real YouTube login/consent; verify same.
4. Capture logs and request traces for any divergence.

**Acceptance criteria:**
- Both services can connect and playlist pages show non-error states in same run.

## Phase C: UX hardening for auth outcomes

1. Replace console-only status messaging with visible in-app success/error surfaces.
2. Preserve query-param based callback status decoding.
3. Add targeted tests for callback UX rendering.

**Acceptance criteria:**
- User receives visible confirmation/error feedback after callback without opening devtools.

---

## 7. Risks and Tradeoffs

- **Risk:** Large refactors during auth stabilization can mask root cause of provider integration issues.
  - **Mitigation:** Keep changes small and isolated; postpone route-wide renames.

- **Risk:** Provider console misconfiguration can be mistaken for backend OAuth bug.
  - **Mitigation:** Treat redirect URI exact-match checks as first debugging step.

- **Risk:** Declaring "all green" from test suites without browser/provider verification gives false confidence.
  - **Mitigation:** Keep validation language explicit: unit/build green is not equivalent to provider E2E proof.

---

## 8. Reporting Template for Local Continuation

For each local run, report:

1. **Commit/branch used**
2. **Env snapshot** (`PUBLIC_URL`, `FRONTEND_URL`, host variant)
3. **Spotify flow result**
4. **YouTube flow result**
5. **Playlist endpoint status after auth**
6. **User-visible dashboard outcome**
7. **Delta changes + tests run**

This keeps local and PR discussion aligned and prevents repeated rediscovery.

---

## 9. References

- PR #4: current implementation baseline.
- `docs/handoffs/LOCAL_AGENT_HANDOFF_PR4.md`: operator-level handoff checklist.
- Existing RFCs under `docs/rfcs/` for migration and feature history.

---

*End of RFC-101*
