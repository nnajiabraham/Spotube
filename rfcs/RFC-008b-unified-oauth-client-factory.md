# RFC-008b: Unified OAuth Client Factory System

**Status:** Draft  
**Branch:** `rfc/008b-unified-oauth-client-factory`  
**Depends On:**
* RFC-004 (Spotify OAuth integration)
* RFC-005 (YouTube OAuth integration) 
* RFC-008 (Sync execution job - completed)

---

## 1. Goal

Create a unified OAuth client factory system that eliminates code duplication between background jobs and API handlers for Spotify and YouTube authentication, while implementing the missing settings collection integration and maintaining backward compatibility.

## 2. Background & Context

During RFC-008 implementation, we discovered significant code duplication and inconsistency in OAuth client creation:

### Current Issues:
1. **Code Duplication**: `getSpotifyClientForJob()` and `getYouTubeServiceForJob()` in `backend/internal/jobs/analysis.go` duplicate OAuth token handling logic already present in `backend/internal/pbext/spotifyauth/spotifyauth.go` and `backend/internal/pbext/googleauth/googleauth.go`

2. **Interface Inconsistency**: 
   - Jobs use `daoProvider` interface for database access
   - Auth packages expect `*pocketbase.PocketBase` + Echo context
   - This prevents code sharing and creates maintenance burden

3. **Missing Settings Integration**: 
   - `spotifyauth.go` line 41 has TODO comment: "Implement loading from settings collection"
   - `googleauth.go` doesn't have equivalent functionality
   - Both packages currently only load from environment variables

4. **Maintenance Burden**: OAuth token refresh logic, error handling, and client configuration exist in multiple places

### RFC-008 Implementation Context:
From RFC-008 completion, we learned:
- Background jobs need OAuth clients without Echo context dependency
- Token refresh logic is critical for long-running jobs
- Error handling must be consistent across job and API contexts
- PocketBase relation fields require special handling (arrays vs strings)

### Original RFC Dependencies:
- **RFC-004** (Spotify OAuth): Implemented `backend/internal/pbext/spotifyauth/spotifyauth.go` with Echo-dependent client factory
- **RFC-005** (YouTube OAuth): Implemented `backend/internal/pbext/googleauth/googleauth.go` with similar Echo-dependent approach
- Both RFCs created the foundation but didn't anticipate background job requirements

## 3. Technical Design

### 3.1 Unified OAuth Client Factory Package

Create `backend/internal/auth/` package with unified client factories:

**Files to Create:**
- `backend/internal/auth/spotify.go` - Unified Spotify client factory
- `backend/internal/auth/youtube.go` - Unified YouTube client factory  
- `backend/internal/auth/common.go` - Shared OAuth token management
- `backend/internal/auth/settings.go` - Settings collection integration

**Core Interfaces:**
```go
// Common interface for database access (compatible with both jobs and API handlers)
type DatabaseProvider interface {
    Dao() *daos.Dao
}

// Context interface for different execution environments  
type AuthContext interface {
    GetCredentials(service string) (clientID, clientSecret string, err error)
}

// Unified client factory interface
type ClientFactory interface {
    GetSpotifyClient(ctx context.Context, dbProvider DatabaseProvider, authCtx AuthContext) (*spotify.Client, error)
    GetYouTubeService(ctx context.Context, dbProvider DatabaseProvider, authCtx AuthContext) (*youtube.Service, error)
}
```

### 3.2 Settings Collection Integration

Implement credential loading from PocketBase settings collection with environment variable fallback:

**Priority Order:**
1. Settings collection (`spotify_client_id`, `spotify_client_secret`, `google_client_id`, `google_client_secret`)
2. Environment variables (`SPOTIFY_CLIENT_ID`, `SPOTIFY_CLIENT_SECRET`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`)

**Files to Modify:**
- Update settings collection schema if needed
- Implement `loadCredentialsFromSettings()` function
- Add error handling for missing credentials

### 3.3 Background Job Integration

Refactor job OAuth clients to use unified factory:

**Files to Modify:**
- `backend/internal/jobs/analysis.go` - Remove duplicate `getSpotifyClientForJob()` and `getYouTubeServiceForJob()`
- `backend/internal/jobs/executor.go` - Update references to use unified factory
- Update imports and function calls

**Context Implementation:**
```go
type JobAuthContext struct {
    dbProvider DatabaseProvider
}

func (j *JobAuthContext) GetCredentials(service string) (string, string, error) {
    // Load from settings collection first, fallback to env vars
}
```

### 3.4 API Handler Integration  

Refactor existing auth packages to use unified factory while maintaining backward compatibility:

**Files to Modify:**
- `backend/internal/pbext/spotifyauth/spotifyauth.go` - Refactor `withSpotifyClient()` to use unified factory
- `backend/internal/pbext/googleauth/googleauth.go` - Refactor `withGoogleClient()` to use unified factory
- Maintain existing function signatures for backward compatibility

**Context Implementation:**
```go
type APIAuthContext struct {
    echoContext echo.Context
    dbProvider DatabaseProvider
}

func (a *APIAuthContext) GetCredentials(service string) (string, string, error) {
    // Same settings/env loading logic as jobs
}
```

### 3.5 Token Refresh Logic Unification

Extract and unify OAuth token refresh logic:

**Common Features:**
- Automatic token refresh when expired (30-second buffer)
- Database persistence of refreshed tokens
- Error handling for refresh failures
- Thread-safe token operations

**Implementation Pattern:**
```go
func refreshTokenIfNeeded(ctx context.Context, dbProvider DatabaseProvider, token *oauth2.Token, config *oauth2.Config, provider string) (*oauth2.Token, error) {
    if token.Expiry.Before(time.Now().Add(30 * time.Second)) {
        // Unified refresh logic
    }
    return token, nil
}
```

## 4. Dependencies

**New Dependencies:**
- No new external dependencies required

**Internal Dependencies:**
- `backend/internal/jobs/analysis.go` (RFC-008)
- `backend/internal/jobs/executor.go` (RFC-008) 
- `backend/internal/pbext/spotifyauth/spotifyauth.go` (RFC-004)
- `backend/internal/pbext/googleauth/googleauth.go` (RFC-005)

## 5. Checklist

### Phase 1: Foundation & Settings Integration
- [ ] **F1** Create `backend/internal/auth/` package structure with `common.go`, `settings.go`
- [ ] **F2** Implement settings collection credential loading with environment fallback
- [ ] **F3** Create unified `DatabaseProvider` and `AuthContext` interfaces
- [ ] **F4** Write comprehensive tests for settings loading and credential resolution
- [ ] **F5** Verify all existing tests still pass after foundation changes

### Phase 2: Unified OAuth Token Management
- [ ] **T1** Extract common OAuth token refresh logic into `backend/internal/auth/common.go`
- [ ] **T2** Implement thread-safe token operations with proper error handling
- [ ] **T3** Create unified token persistence methods
- [ ] **T4** Write tests for token refresh scenarios (expired, near-expired, refresh failure)
- [ ] **T5** Verify token refresh works correctly with test OAuth scenarios

### Phase 3: Spotify Client Factory
- [ ] **S1** Create `backend/internal/auth/spotify.go` with unified Spotify client factory
- [ ] **S2** Implement `JobAuthContext` for background job credential access
- [ ] **S3** Implement `APIAuthContext` for Echo-based credential access  
- [ ] **S4** Write comprehensive tests for both job and API contexts
- [ ] **S5** Verify Spotify client creation works in both environments

### Phase 4: YouTube Client Factory
- [ ] **Y1** Create `backend/internal/auth/youtube.go` with unified YouTube service factory
- [ ] **Y2** Implement context-aware YouTube service creation
- [ ] **Y3** Handle YouTube quota integration compatibility (from RFC-008)
- [ ] **Y4** Write comprehensive tests for YouTube service creation
- [ ] **Y5** Verify YouTube service works correctly with quota tracking

### Phase 5: Background Jobs Refactoring
- [ ] **J1** Update `backend/internal/jobs/analysis.go` to use unified auth factory
- [ ] **J2** Remove duplicate `getSpotifyClientForJob()` and `getYouTubeServiceForJob()` functions
- [ ] **J3** Update `backend/internal/jobs/executor.go` imports and client creation calls
- [ ] **J4** Update all job tests to use unified factory
- [ ] **J5** Verify all RFC-008 functionality still works (analysis job, executor job, quota tracking)

### Phase 6: API Handler Refactoring  
- [ ] **A1** Refactor `backend/internal/pbext/spotifyauth/spotifyauth.go` to use unified factory
- [ ] **A2** Refactor `backend/internal/pbext/googleauth/googleauth.go` to use unified factory
- [ ] **A3** Maintain backward compatibility for existing API endpoints
- [ ] **A4** Update auth package tests to verify unified factory integration
- [ ] **A5** Verify all RFC-004 and RFC-005 functionality still works (OAuth flows, API endpoints)

### Phase 7: Integration Testing & Documentation
- [ ] **I1** Run complete test suite to ensure no regressions in any RFC functionality
- [ ] **I2** Test end-to-end scenarios: OAuth login → mapping creation → sync execution
- [ ] **I3** Verify settings collection credential loading works in all contexts
- [ ] **I4** Update relevant documentation and code comments
- [ ] **I5** Performance test: ensure no significant overhead from unified approach

## 6. Definition of Done

- All code duplication between job and API OAuth clients eliminated
- Settings collection credential loading implemented for both Spotify and YouTube
- Unified auth factory works for both background jobs and API handlers  
- All existing functionality from RFC-004, RFC-005, and RFC-008 continues to work
- Comprehensive test coverage for all auth scenarios (job context, API context, settings loading, token refresh)
- Zero regressions: all existing tests pass
- Code is maintainable: OAuth changes only need to be made in one place

## Implementation Notes / Summary

**CRITICAL CONTEXT FOR IMPLEMENTING AGENT:**

### Required Research Before Implementation

**MANDATORY:** Before starting any implementation work, you must research the original RFC implementations to understand the design decisions, requirements, and constraints. Use these commands and guidelines:

#### 1. Research Original RFC Context

**Git Blame Analysis:**
```bash
# Research Spotify auth implementation history
git log --oneline --follow backend/internal/pbext/spotifyauth/spotifyauth.go | head -10
git blame backend/internal/pbext/spotifyauth/spotifyauth.go | grep -E "(TODO|SPOTIFY|CLIENT)"

# Research Google auth implementation history  
git log --oneline --follow backend/internal/pbext/googleauth/googleauth.go | head -10
git blame backend/internal/pbext/googleauth/googleauth.go | grep -E "(GOOGLE|CLIENT|OAUTH)"
```

**Why:** These commands reveal which commits/RFCs implemented the original auth packages, helping you understand the original design decisions and requirements.

#### 2. Locate and Study Original RFCs

**Find RFC Files:**
```bash
# Look for RFC-004 (Spotify) and RFC-005 (YouTube) files
find rfcs/ -name "*004*" -o -name "*spotify*" | head -5
find rfcs/ -name "*005*" -o -name "*youtube*" -o -name "*google*" | head -5
```

**Read Original RFCs:** Once located, read the complete RFC-004 and RFC-005 files, paying special attention to:
- **Technical Design sections** - understand the original architecture decisions
- **Implementation Notes/Summary sections** - critical context about what was implemented and why
- **Dependencies and requirements** - what the original implementations needed to support
- **Testing requirements** - what test patterns were established

#### 3. Analyze Current Implementation Patterns

**Study Current Auth Usage:**
```bash
# Find all current usages of Spotify auth
grep -r "withSpotifyClient\|getSpotifyClientForJob" backend/ --include="*.go"

# Find all current usages of YouTube auth  
grep -r "withGoogleClient\|getYouTubeServiceForJob" backend/ --include="*.go"

# Check current settings collection usage
grep -r "settings" backend/ --include="*.go" | grep -E "(collection|record)"
```

**Why:** Understanding current usage patterns helps ensure backward compatibility and identifies all integration points that need to be maintained.

#### 4. Research Test Patterns

**Analyze Test Approaches:**
```bash
# Study auth-related test patterns
find backend/ -name "*auth*test*.go" -exec basename {} \;
find backend/ -name "*test*.go" -exec grep -l "spotify\|youtube\|oauth" {} \;
```

**Review Test Files:** Examine the test files to understand:
- How OAuth mocking is currently handled
- What test scenarios are covered
- Integration patterns with httpmock
- Expected error handling patterns

#### 5. Settings Collection Investigation

**Current Settings Schema:**
```bash
# Check current settings collection structure
grep -r "spotify_client\|google_client" backend/ --include="*.go"
find backend/migrations/ -name "*settings*" -exec cat {} \;
```

**Why:** The TODO comment in `spotifyauth.go` line 41 indicates planned settings collection integration. You need to understand what settings schema already exists and what needs to be added.

### When to Use Research Commands

**Before Each Phase:**
- **Phase 1 (Foundation):** Research settings collection current state and auth interface patterns
- **Phase 3 (Spotify):** Deep dive into RFC-004 implementation and `spotifyauth.go` design decisions  
- **Phase 4 (YouTube):** Deep dive into RFC-005 implementation and `googleauth.go` design decisions
- **Phase 5 (Jobs):** Review RFC-008 job patterns and `daoProvider` interface usage
- **Phase 6 (API):** Review original OAuth flow requirements from RFC-004/RFC-005

**During Implementation:**
- When encountering unexpected behavior, use git blame to understand why specific code exists
- When tests fail, research the original test patterns and requirements
- When refactoring, verify against original RFC requirements

**Before Completing Each Checklist Item:**
- Verify your changes don't conflict with original RFC requirements
- Check that test patterns match established conventions
- Ensure backward compatibility with original design decisions

### RFC-008 Key Implementation Details:
- **PocketBase Relation Fields**: `mapping_id` fields are stored as `[]string` arrays, not strings. Use this pattern:
  ```go
  var mappingID string
  rawMappingId := item.Get("mapping_id")
  if mappingIds, ok := rawMappingId.([]string); ok && len(mappingIds) > 0 {
      mappingID = mappingIds[0] // Get first element from array
  } else {
      mappingID = item.GetString("mapping_id") // Fallback to string
  }
  ```

- **daoProvider Interface Pattern**: RFC-008 established this pattern for testability:
  ```go
  type daoProvider interface {
      Dao() *daos.Dao
  }
  ```
  The unified factory must be compatible with this interface.

- **Token Refresh Requirements**: Background jobs require automatic token refresh without user interaction. Current implementations in `getSpotifyClientForJob()` and `getYouTubeServiceForJob()` show the correct refresh pattern.

- **YouTube Quota Integration**: The unified factory must maintain compatibility with `YouTubeQuotaTracker` from RFC-008. See `backend/internal/jobs/executor.go` lines 30-61 for quota tracking implementation.

### Original RFC Context:
- **RFC-004 Context**: Check git blame on `backend/internal/pbext/spotifyauth/spotifyauth.go` for implementation details and requirements
- **RFC-005 Context**: Check git blame on `backend/internal/pbext/googleauth/googleauth.go` for implementation details and requirements
- **Settings Collection**: The TODO on line 41 of `spotifyauth.go` was noted during RFC-004 but not implemented

### Testing Requirements:
- **All existing tests must pass** after each checklist item - use `go test ./...` to verify
- **MSW mocking**: Frontend tests should continue to use MSW for API mocking  
- **HTTP mocking**: Backend tests use `httpmock` for external API calls (pattern established in RFC-008)

### Critical Files Modified in RFC-008:
- `backend/internal/jobs/analysis.go` - Contains OAuth client creation for jobs
- `backend/internal/jobs/executor.go` - Uses OAuth clients for actual API calls
- `backend/internal/jobs/executor_test.go` - Test patterns for OAuth client testing

### Environment Variables:
Current environment variables that must be supported:
- `SPOTIFY_CLIENT_ID`, `SPOTIFY_CLIENT_SECRET`
- `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`

Settings collection fields to implement:
- `spotify_client_id`, `spotify_client_secret`  
- `google_client_id`, `google_client_secret`

### Backward Compatibility Requirements:
- All existing API endpoints must continue to work unchanged
- All existing job functionality must continue to work unchanged
- Existing function signatures in auth packages should be maintained where possible
- OAuth flows from RFC-004 and RFC-005 must remain functional

**IMPLEMENTATION SEQUENCE**: Complete checklist items sequentially, marking each `[X]` and updating this Implementation Notes section with detailed changes after each phase. Pay special attention to test compatibility and ensure zero regressions.

**RESEARCH FIRST, IMPLEMENT SECOND**: Do not skip the research phase. Understanding the original design decisions is critical to successful implementation without regressions.

---

**References:**
- RFC-008: Sync Execution Job (completed) - for job OAuth patterns and `daoProvider` interface
- RFC-004: Spotify OAuth Integration - for API OAuth patterns and Echo context usage  
- RFC-005: YouTube OAuth Integration - for YouTube-specific OAuth patterns
- RFC-007: Sync Analysis Job - for background job execution context
- `backend/internal/jobs/analysis.go` lines 349-442 - Current job OAuth implementation
- `backend/internal/pbext/spotifyauth/spotifyauth.go` lines 250-422 - Current API OAuth implementation
``` 