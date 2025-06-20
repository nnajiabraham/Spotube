# RFC-009b: OAuth Flow and Migration Standardization Fixes

**Status:** Completed  
**Branch:** `master` (applied directly)
**Related Issues:** Fixes issues discovered during manual testing of RFC-009.

## 1. Goal

To retroactively document a series of critical fixes applied to the OAuth authentication flow for both Spotify and Google, and to standardize the database migration file format for improved consistency and maintainability.

## 2. Background & Context

During and after the implementation of RFC-009, several issues were discovered through manual testing that prevented the application from functioning correctly:
1.  **Google OAuth `redirect_uri_mismatch`**: The redirect URI sent to Google did not exactly match the one configured in the Google Cloud Console, often due to `localhost` vs `127.0.0.1` discrepancies.
2.  **Spotify Authentication Expiration**: Spotify sessions would expire, and the token refresh mechanism was not correctly saving the new token, forcing users to re-authenticate frequently.
3.  **YouTube API 403 Forbidden Error**: After authenticating with Google, calls to the YouTube Data API failed with a "Method doesn't allow unregistered callers" error, indicating an issue with authentication scopes and identity establishment.
4.  **Inconsistent Migration Files**: Several older migration files used raw JSON strings for schema definitions, while newer ones used Go struct helpers, leading to maintenance difficulties and potential errors.
5.  **Incorrect PKCE Implementation**: The PKCE code challenge for Spotify was not being generated correctly, which could lead to authentication failures.

This RFC documents the fixes for these issues to ensure the system is robust and follows consistent architectural patterns.

## 3. Technical Design

### 3.1 OAuth Flow and Token Refresh Fixes
- **Standardized Redirect URI Logic**: Both `spotifyauth.go` and `googleauth.go` were updated to derive the backend-facing redirect URI from the `PUBLIC_URL` environment variable, while a new `FRONTEND_URL` variable was introduced to handle redirects back to the UI after the OAuth callback is processed. This ensures the URI sent to the provider always matches the one registered in their respective developer consoles.
- **Corrected PKCE Challenge Generation**: The `generateCodeChallenge` function in `spotifyauth.go` was fixed to compute a proper SHA256 hash of the verifier and encode it using Base64 URL encoding without padding, as per the PKCE specification.
- **Fixed Token Expiry Persistence**: The `saveTokenToDatabase` function (now `SaveTokenWithScopes`) in `auth/common.go` was updated to correctly handle the `time.Time` to `types.DateTime` conversion that PocketBase requires. Previously, it was storing a zero-value time, causing tokens to appear instantly expired.
- **Expanded Google OAuth Scopes**: To resolve the 403 error and establish proper user identity, the Google OAuth flow was updated to include the `userinfo.profile` and `userinfo.email` scopes in addition to the `youtube.readonly` scope.

### 3.2 Architectural Refactoring
- **Simplified YouTube Authentication**: The authentication flow in `googleauth.go` was refactored to be as direct and clean as the Spotify implementation. Unnecessary layers of indirection and confusingly named functions (`withGoogleClient`, `withGoogleClientCustom`, `GetYouTubeService`) were removed and consolidated into a single, clear `WithGoogleClient` function that mirrors Spotify's `withSpotifyClient`.
- **Centralized Token Saving**: The duplicated `saveSpotifyTokens` function was removed from `spotifyauth.go`. Both auth handlers now use the public `auth.SaveTokenWithScopes` function from the common auth package, ensuring consistent token persistence logic.

### 3.3 Migration Standardization
- The following migration files were refactored from using raw JSON strings to using native Go `schema.SchemaField` structs for defining collections:
  - `backend/migrations/1750298622_create_sync_items_collection.go`
  - `backend/migrations/1750298769_add_analysis_fields_to_mappings.go`
  - `backend/migrations/1750363691_add_execution_fields_to_sync_items.go`
- This change improves type safety, readability, and maintainability of the database schema.

### 3.4 Added Debugging and Testing
- **Enhanced Logging**: Added detailed `log.Printf` statements throughout the Spotify and Google OAuth flows to provide visibility into state changes, token exchanges, and potential errors.
- **New Test Cases**: Added `oauth_settings_integration_test.go` files for both `spotifyauth` and `googleauth` to specifically test the credential loading priority (Settings Collection vs. Environment Variables).
- **Corrected Existing Tests**: All broken tests were fixed to align with the new, corrected authentication and redirect logic.

## 4. Checklist

- [X] **F1** Fix PKCE `generateCodeChallenge` function in `spotifyauth.go`.
- [X] **F2** Standardize OAuth redirect URI handling for Spotify and Google.
- [X] **F3** Add `FRONTEND_URL` environment variable for post-auth redirects.
- [X] **F4** Correctly handle `types.DateTime` for token expiry persistence.
- [X] **F5** Add `userinfo.profile` and `userinfo.email` scopes to Google OAuth.
- [X] **F6** Refactor and simplify YouTube authentication flow to match Spotify's.
- [X] **F7** Centralize token saving logic by removing `saveSpotifyTokens`.
- [X] **F8** Add comprehensive tests for OAuth credential loading priority.
- [X] **F9** Standardize `create_sync_items` migration to use Go schema helpers.
- [X] **F10** Standardize `add_analysis_fields_to_mappings` migration.
- [X] **F11** Standardize `add_execution_fields_to_sync_items` migration.
- [X] **F12** Verify all backend and frontend tests pass after changes.
- [X] **F13** Update `README.md` to document `.env` file usage and correct env var names.

## 5. Definition of Done
* All items on the checklist are completed.
* The OAuth flow for both Spotify and Google works reliably from the frontend.
* Refresh tokens are correctly stored and used, preventing the need for frequent re-authentication.
* All database migration files use a consistent, maintainable format.
* All backend and frontend tests pass without errors.
* The application runs successfully locally with the new changes.

## 6. Implementation Notes / Summary

**F1-F3, F5-F7, F8 COMPLETED** - OAuth Flow and Architecture Refactoring:
* **PKCE Fixed**: Corrected the `generateCodeChallenge` in `spotifyauth.go` to use SHA256 hashing.
* **Redirects Fixed**: Separated `PUBLIC_URL` (for backend callbacks) and `FRONTEND_URL` (for user-facing redirects). Logic was added to both `spotifyauth.go` and `googleauth.go` to ensure the backend URI is used for the OAuth provider, and the frontend URI is used to redirect the user back to the app after success.
* **Google Scopes Fixed**: Added `userinfo.profile` and `userinfo.email` scopes to the Google OAuth configuration, using the `googleoauth2` package constants. This resolved the "unregistered callers" error by properly establishing user identity.
* **YouTube Auth Simplified**: Refactored `googleauth.go` to remove the multiple layers of indirection (`withGoogleClient`, `withGoogleClientCustom`, etc.) and created a single, clear `WithGoogleClient` function that mirrors the working pattern from the Spotify implementation.
* **Token Saving Centralized**: Removed the local `saveSpotifyTokens` function and updated the Spotify callback to use the shared `auth.SaveTokenWithScopes`, ensuring consistent logic for both providers.
* **New Tests Added**: Created `oauth_settings_integration_test.go` for both auth packages to assert the correct credential loading priority (settings collection > environment variables).

**F4 COMPLETED** - Token Expiry Persistence:
* **The Root Cause**: The `expiry` field, a `time.Time` object, was being saved directly to the PocketBase record. The database layer did not correctly interpret this, resulting in a zero-value timestamp (`0001-01-01`).
* **The Fix**: Updated `loadTokenFromDatabase` in `auth/common.go` to use `types.ParseDateTime(record.Get("expiry"))`, which correctly handles PocketBase's internal date string format.
* **Test Added**: Added `TestTokenExpiry` to `common_test.go` to explicitly verify that expiry dates are saved and retrieved correctly, preventing future regressions.

**F9-F11 COMPLETED** - Migration Standardization:
* **The Problem**: Inconsistent schema definitions in migration files (some using raw JSON, others using Go structs).
* **The Fix**: Refactored the following files to exclusively use the `schema.SchemaField` struct pattern for defining collections:
    - `1750298622_create_sync_items_collection.go`
    - `1750298769_add_analysis_fields_to_mappings.go`
    - `1750363691_add_execution_fields_to_sync_items.go`
* **Result**: This resolved the `UNIQUE constraint failed: _collections.name` error when running migrations on a fresh database and makes the schema definitions more maintainable.

**F12-F13 COMPLETED** - Verification and Documentation:
* **All Tests Pass**: Confirmed that the entire backend and frontend test suites pass after the fixes.
* **README Updated**: The main `README.md` was updated to reflect the new `FRONTEND_URL` environment variable and to clarify the use of `.env` files for local development.

These changes collectively resolved all identified authentication and database issues, resulting in a stable, robust, and more maintainable system. 