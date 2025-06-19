# RFC-007c: Comprehensive Test Integration Refactoring

**Status:** Draft  
**Branch:** `rfc/007c-test-integration-refactoring`  
**Depends On:**
* RFC-007 (sync analysis job implementation - testing patterns established)
* RFC-007b (test refactoring and shared helpers - assumed implemented)

---

## 1. Goal

Refactor all existing test suites in the project to use proper PocketBase integration testing patterns established in RFC-007, replacing isolated/mocked approaches with real PocketBase database operations and actual implementation function calls.

## 2. Background & Context

During RFC-007 implementation, we discovered that proper PocketBase integration testing requires specific patterns and provides significantly better validation than isolated unit tests. However, several existing test suites across the project still use older testing approaches that:

- Test isolated logic simulations rather than actual implementation functions
- Use incomplete mocking that doesn't validate real PocketBase integration
- Miss critical bugs that only surface with real database operations
- Don't follow established testing patterns for consistency

### Key Testing Issues Identified:

1. **`googleauth_test.go`** (RFC-005): Limited PocketBase integration, missing comprehensive collection setup
2. **`spotifyauth_test.go`** (RFC-004/004b): Partial mocking, doesn't test real PocketBase SDK integration
3. **`mappings_test.go`** (RFC-006): Tests document expected behavior but don't validate actual implementation
4. **`routes_test.go`** (RFC-003): Unit tests only, missing real PocketBase collection operations

### RFC-007 Integration Testing Patterns Established:

From RFC-007 implementation, we have proven patterns for:
- **PocketBase Test App Setup**: Using `tests.NewTestApp()` with real collections
- **HTTP Mocking**: Proper `httpmock` configuration for background jobs
- **Data Type Handling**: PocketBase-specific date formats, relation fields, filter syntax
- **Collection Schema Setup**: Proper field definitions, indexes, and relationships
- **OAuth Token Testing**: Real token storage and refresh validation

### RFC-007b Shared Helpers (Assumed Available):

Per RFC-007b, the following shared helpers are assumed to be implemented and available:
- `testhelpers.SetupTestApp(t)` - Standard test app creation
- `testhelpers.CreateOAuthTokensCollection(t, testApp)` - OAuth tokens collection
- `testhelpers.CreateMappingsCollection(t, testApp)` - Mappings collection  
- `testhelpers.CreateSyncItemsCollection(t, testApp)` - Sync items collection
- `testhelpers.SetupOAuthTokens(t, testApp)` - OAuth token test data
- `testhelpers.SetupAPIHttpMocks(t)` - HTTP mocking for APIs
- `testhelpers.CreateTestMapping(testApp, properties)` - Mapping creation helper

## 3. Technical Design

### 3.1 Testing Architecture Requirements

All refactored tests must follow RFC-007 established patterns:

**PocketBase Integration Requirements:**
- Use `testhelpers.SetupTestApp(t)` for real database operations
- Create actual collections with proper schema
- Test real implementation functions, not isolated logic
- Use proper PocketBase data types and formats

**HTTP Mocking Requirements:**
- Use `httpmock` with `http.DefaultTransport` configuration
- Mock OAuth endpoints (Spotify, Google) for token operations
- Use flexible regex patterns for API endpoint matching

**Data Format Requirements:**
- Dates: `2006-01-02 15:04:05.000Z` format for PocketBase
- Relations: Handle `[]string` arrays for relation field access
- Filters: Use `id != ''` instead of empty string for "all records"

### 3.2 Regression Testing Requirements

Following RFC-007 pattern, each task must include:
- Full test suite execution validation (`make test-backend`)
- Zero regression tolerance policy
- Validation that all existing tests continue to pass
- Documentation of any breaking changes requiring fixes

## 4. Implementation Tasks

### 4.1 Task T1: GoogleAuth Test Integration (RFC-005 Context)

**File:** `backend/internal/pbext/googleauth/googleauth_test.go`  
**Implementation:** `backend/internal/pbext/googleauth/googleauth.go`  
**Original RFC:** RFC-005 (YouTube OAuth Integration)

**Business Logic Context from RFC-005:**
- **OAuth Flow**: Authorization code flow with PKCE for YouTube OAuth
- **Token Management**: Stores tokens in `oauth_tokens` collection with `provider = 'google'`
- **API Integration**: YouTube Data API v3 for playlist access
- **Refresh Logic**: Automatic token refresh with 30-second expiry buffer
- **Collection Schema**: Uses `oauth_tokens` collection with provider, access_token, refresh_token, expiry, scopes

**Current Test State Analysis:**
```go
// CURRENT: Limited integration testing
func TestCallbackHandler_Success(t *testing.T) {
    testApp, err := tests.NewTestApp()
    // Only tests callback success path
    // Missing comprehensive OAuth flow validation
    // Limited HTTP mocking for Google APIs
}
```

**Required Refactoring Tasks:**
1. **Comprehensive OAuth Flow Testing**: Create full end-to-end tests for login → callback → token storage
2. **YouTube API Integration**: Use `testhelpers.SetupAPIHttpMocks()` for YouTube Data API v3 calls
3. **Token Refresh Validation**: Test `withGoogleClient()` function with real token refresh scenarios
4. **Playlist Fetching**: Test `playlistsHandler()` with real YouTube API responses
5. **Collection Integration**: Use `testhelpers.CreateOAuthTokensCollection()` for proper schema
6. **Error Handling**: Test real error scenarios (expired tokens, API failures, missing tokens)

**Key Implementation Functions to Test:**
- `loginHandler()` - PKCE flow initiation
- `callbackHandler()` - Token exchange and storage
- `playlistsHandler()` - YouTube playlist fetching
- `withGoogleClient()` - Authenticated client creation with refresh
- `saveGoogleTokens()` - Token persistence

**HTTP Mocking Requirements:**
```go
// Required API endpoints to mock
httpmock.RegisterResponder("POST", "https://oauth2.googleapis.com/token", ...)
httpmock.RegisterResponder("GET", `=~^https://.*youtube.*playlists`, ...)
```

**PocketBase Integration Points:**
- OAuth tokens collection operations
- Real token storage and retrieval
- Token refresh and persistence
- Error handling for missing/invalid tokens

### 4.2 Task T2: SpotifyAuth Test Integration (RFC-004/004b Context)

**File:** `backend/internal/pbext/spotifyauth/spotifyauth_test.go`  
**Implementation:** `backend/internal/pbext/spotifyauth/spotifyauth.go`  
**Original RFCs:** RFC-004 (Spotify OAuth), RFC-004b (PocketBase SDK Migration)

**Business Logic Context from RFC-004/004b:**
- **OAuth Flow**: Authorization Code Flow with PKCE for Spotify OAuth
- **Token Management**: Stores tokens in `oauth_tokens` collection with `provider = 'spotify'`
- **API Integration**: Spotify Web API for playlist access
- **Refresh Logic**: Automatic token refresh with 30-second expiry buffer
- **PKCE Implementation**: 64-byte verifier with S256 challenge method
- **Collection Schema**: Uses `oauth_tokens` collection with provider, access_token, refresh_token, expiry, scopes

**Current Test State Analysis:**
```go
// CURRENT: Partial mocking, incomplete integration
func TestCallbackHandler_Success(t *testing.T) {
    // Uses httpmock for token exchange
    // Comment: "full test requires database mocking"
    // Missing actual PocketBase integration
}

func TestLoginHandler(t *testing.T) {
    // Only tests redirect URL generation
    // Missing PKCE validation
    // No database operations tested
}
```

**Required Refactoring Tasks:**
1. **Complete OAuth Flow Testing**: Test login → redirect → callback → token storage with real database
2. **PKCE Validation**: Test PKCE verifier/challenge generation and validation
3. **Spotify API Integration**: Use `testhelpers.SetupAPIHttpMocks()` for Spotify Web API
4. **Token Refresh Logic**: Test `withSpotifyClient()` with real refresh scenarios
5. **Playlist Endpoint**: Test `playlistsHandler()` with real Spotify API responses
6. **Collection Operations**: Use `testhelpers.CreateOAuthTokensCollection()` and test real token CRUD
7. **Error Scenarios**: Test missing tokens, expired tokens, API failures

**Key Implementation Functions to Test:**
- `loginHandler()` - PKCE flow initiation with cookie handling
- `callbackHandler()` - Token exchange, PKCE validation, storage
- `playlistsHandler()` - Spotify playlist fetching with pagination
- `withSpotifyClient()` - Authenticated client with refresh
- `saveSpotifyTokens()` - Token persistence
- `parseAuthCookie()` - Cookie parsing logic
- `generateCodeChallenge()` - PKCE challenge generation

**HTTP Mocking Requirements:**
```go
// Required API endpoints to mock
httpmock.RegisterResponder("POST", "https://accounts.spotify.com/api/token", ...)
httpmock.RegisterResponder("GET", `=~^https://api\.spotify\.com/v1/me/playlists`, ...)
```

**PocketBase Integration Points:**
- OAuth tokens collection with Spotify provider
- Real PKCE state management
- Token refresh and persistence
- Error handling for authentication failures

### 4.3 Task T3: Mappings Test Integration (RFC-006 Context)

**File:** `backend/internal/pbext/mappings/mappings_test.go`  
**Implementation:** `backend/internal/pbext/mappings/hooks.go`  
**Original RFC:** RFC-006 (Playlist Mapping Collections & UI)

**Business Logic Context from RFC-006:**
- **Collection Schema**: Mappings collection with Spotify/YouTube playlist pairs
- **Validation Logic**: `interval_minutes >= 5` validation in BeforeCreate/Update hooks
- **Default Values**: `sync_name=true`, `sync_tracks=true`, `interval_minutes=60`
- **Duplicate Prevention**: Unique index on (spotify_playlist_id, youtube_playlist_id)
- **Name Caching**: Background fetching of playlist names via `fetchAndCachePlaylistNames`
- **Hook Implementation**: BeforeCreate, BeforeUpdate, AfterCreate, AfterUpdate hooks

**Current Test State Analysis:**
```go
// CURRENT: Documents expected behavior, doesn't test implementation
func TestMappingsValidation(t *testing.T) {
    t.Run("interval_minutes validation", func(t *testing.T) {
        // Tests isolated logic: isValid := tc.intervalMinutes >= 5
        // Comment: "In actual implementation, this validation happens in the hooks"
        // PROBLEM: Doesn't test actual hook functions
    })
    
    t.Run("duplicate mapping prevention", func(t *testing.T) {
        // Documents expected database behavior
        // Comment: "The database unique index... will return an error"
        // PROBLEM: Doesn't test actual database constraint
    })
}
```

**Required Refactoring Tasks:**
1. **Hook Function Testing**: Test actual `RegisterHooks()` implementation with real PocketBase
2. **Collection Creation**: Use `testhelpers.CreateMappingsCollection()` with proper schema
3. **Validation Testing**: Test BeforeCreate/Update hooks with real record operations
4. **Default Value Testing**: Test actual default value setting in BeforeCreate hook
5. **Duplicate Constraint**: Test real database unique index constraint
6. **Name Caching**: Test AfterCreate/Update hooks trigger name fetching
7. **Error Scenarios**: Test validation failures return proper errors

**Key Implementation Functions to Test:**
- `RegisterHooks()` - Hook registration with PocketBase
- BeforeCreate hook - Default values and interval validation
- BeforeUpdate hook - Interval validation on updates
- AfterCreate hook - Name caching trigger
- AfterUpdate hook - Conditional name refresh
- `fetchAndCachePlaylistNames()` - Background name fetching (placeholder validation)

**PocketBase Integration Points:**
- Mappings collection creation with proper schema
- Real record creation and validation
- Hook execution with actual database operations
- Unique constraint validation
- Field default value setting

**Test Data Requirements:**
```go
// Test mapping creation with various scenarios
mapping := testhelpers.CreateTestMapping(testApp, map[string]interface{}{
    "spotify_playlist_id": "spotify123",
    "youtube_playlist_id": "youtube456",
    "interval_minutes": 30, // Test validation
})
```

### 4.4 Task T4: Setup Wizard Test Integration (RFC-003 Context)

**File:** `backend/internal/pbext/setupwizard/routes_test.go`  
**Implementation:** `backend/internal/pbext/setupwizard/routes.go`  
**Original RFC:** RFC-003 (Environment Setup Wizard)

**Business Logic Context from RFC-003:**
- **Settings Collection**: Singleton record (id="settings") for credential storage
- **Credential Management**: Stores Spotify/Google OAuth credentials
- **Setup Logic**: Checks environment variables and database for existing credentials
- **Update Control**: `UPDATE_ALLOWED=true` environment variable for credential rotation
- **API Endpoints**: `/api/setup/status` (GET) and `/api/setup` (POST)
- **Security**: Write-only endpoints, credentials never returned

**Current Test State Analysis:**
```go
// CURRENT: Unit tests only, no PocketBase integration
func TestSetupRequestValidation(t *testing.T) {
    // Tests isolated validation logic
    // PROBLEM: Doesn't test actual request handling
}

func TestEnvironmentVariableChecking(t *testing.T) {
    // Tests environment variable logic in isolation
    // PROBLEM: Doesn't test isSetupRequired() function
}
```

**Required Refactoring Tasks:**
1. **Settings Collection Integration**: Use `testhelpers.CreateSettingsCollection()` with singleton record
2. **API Endpoint Testing**: Test actual `/api/setup/status` and `/api/setup` handlers
3. **Setup Logic Testing**: Test `isSetupRequired()` function with real database operations
4. **Credential Storage**: Test `saveCredentials()` function with real record operations
5. **Environment Variable Integration**: Test environment vs database priority logic
6. **Update Control**: Test `UPDATE_ALLOWED` flag behavior with real constraints
7. **Error Scenarios**: Test validation failures, duplicate setup attempts

**Key Implementation Functions to Test:**
- `statusHandler()` - Setup status endpoint
- `postHandler()` - Credential submission endpoint
- `isSetupRequired()` - Setup requirement logic
- `saveCredentials()` - Credential persistence
- Request validation and error handling

**PocketBase Integration Points:**
- Settings collection singleton pattern
- Real credential storage and retrieval
- Environment variable vs database priority
- Error handling for setup conflicts

**Test Scenarios to Implement:**
```go
// Test setup status with various configurations
t.Run("setup required when no credentials exist", func(t *testing.T) {
    testApp := testhelpers.SetupTestApp(t)
    // Test actual isSetupRequired() function
})

t.Run("setup not required when env vars present", func(t *testing.T) {
    // Test environment variable priority
})

t.Run("credential submission creates settings record", func(t *testing.T) {
    // Test actual POST handler with real database
})
```

## 5. Dependencies

### Required Shared Helpers (from RFC-007b):
- `testhelpers.SetupTestApp(t)` - Standard test app creation
- `testhelpers.CreateOAuthTokensCollection(t, testApp)` - OAuth collection setup
- `testhelpers.CreateMappingsCollection(t, testApp)` - Mappings collection setup
- `testhelpers.CreateSettingsCollection(t, testApp)` - Settings collection setup
- `testhelpers.SetupAPIHttpMocks(t)` - HTTP mocking configuration
- `testhelpers.CreateTestMapping(testApp, properties)` - Test data creation

### Testing Dependencies (Already Available):
- `github.com/stretchr/testify` - Test assertions
- `github.com/jarcoal/httpmock` - HTTP mocking
- `github.com/pocketbase/pocketbase/tests` - PocketBase test framework

## 6. Checklist

### Task T1: GoogleAuth Integration
- [ ] **T1.1**: Refactor `TestLoginHandler` to use real PocketBase app
- [ ] **T1.2**: Refactor `TestCallbackHandler_Success` with comprehensive OAuth flow
- [ ] **T1.3**: Add `TestPlaylistsHandler_Integration` with YouTube API mocking
- [ ] **T1.4**: Add `TestWithGoogleClient_TokenRefresh` with real token operations
- [ ] **T1.5**: Add error scenario tests with real PocketBase operations
- [ ] **T1.6**: Run regression testing - validate all existing tests pass

### Task T2: SpotifyAuth Integration  
- [ ] **T2.1**: Refactor `TestLoginHandler` to test real PKCE implementation
- [ ] **T2.2**: Refactor `TestCallbackHandler_Success` with complete database integration
- [ ] **T2.3**: Add `TestPlaylistsHandler_Integration` with Spotify API mocking
- [ ] **T2.4**: Add `TestWithSpotifyClient_TokenRefresh` with real refresh logic
- [ ] **T2.5**: Add PKCE validation tests with real cookie handling
- [ ] **T2.6**: Run regression testing - validate all existing tests pass

### Task T3: Mappings Integration
- [ ] **T3.1**: Refactor validation tests to use actual hook functions
- [ ] **T3.2**: Add `TestRegisterHooks_Integration` with real collection operations
- [ ] **T3.3**: Add duplicate constraint testing with real database operations
- [ ] **T3.4**: Add default value testing with real record creation
- [ ] **T3.5**: Add name caching tests with hook execution validation
- [ ] **T3.6**: Run regression testing - validate all existing tests pass

### Task T4: Setup Wizard Integration
- [ ] **T4.1**: Refactor to test actual `isSetupRequired()` function
- [ ] **T4.2**: Add API endpoint integration tests with real handlers
- [ ] **T4.3**: Add credential storage tests with real settings collection
- [ ] **T4.4**: Add environment variable priority tests with database operations
- [ ] **T4.5**: Add `UPDATE_ALLOWED` flag testing with real constraints
- [ ] **T4.6**: Run regression testing - validate all existing tests pass

### Final Validation
- [ ] **T5.1**: Run complete backend test suite (`make test-backend`)
- [ ] **T5.2**: Validate zero regressions in existing functionality
- [ ] **T5.3**: Update test documentation for future reference

## 7. Definition of Done

* All 4 test suites refactored to use proper PocketBase integration testing
* All tests call actual implementation functions with real database operations
* Comprehensive HTTP mocking for OAuth and API endpoints
* Zero regressions in existing test suite
* All business logic from original RFCs properly validated
* Test patterns consistent with RFC-007 established standards

## 8. Key References and Context

### From RFC-007 Implementation:
- **PocketBase Date Format**: `2006-01-02 15:04:05.000Z` (not RFC3339)
- **Relation Field Access**: Use `record.Get("field_name")` → cast to `[]string` → access `[0]`
- **Filter Syntax**: Use `id != ''` instead of empty string for "all records"
- **HTTP Mocking**: Requires `http.DefaultTransport` for background job compatibility
- **Collection Setup**: Proper field types, indexes, and relationships required

### From RFC-007b (Shared Helpers):
- Standardized test app setup patterns
- Reusable collection creation helpers
- Common HTTP mocking configurations
- Consistent test data creation utilities

### Business Logic Context Links:
- **OAuth Token Management**: See RFC-004 (Spotify) and RFC-005 (YouTube) for token refresh logic
- **Playlist Mapping Logic**: See RFC-006 for validation rules and hook behavior
- **Setup Wizard Logic**: See RFC-003 for credential management and priority logic

### Critical Testing Insights:
- Real PocketBase integration catches bugs that unit tests miss
- HTTP mocking must be configured correctly for OAuth flows
- Database constraints and hooks must be tested with actual operations
- Error scenarios require real PocketBase error handling validation

---

*End of RFC-007c* 