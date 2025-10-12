# RFC-007b: Test Refactoring and Shared Helpers

**Status:** Completed  
**Branch:** `rfc/007b-test-refactoring`  
**Depends On:**
* RFC-007 (sync analysis job implementation)

---

## 1. Goal

Refactor the analysis test suite to ensure all tests use the actual implementation functions with proper PocketBase integration, and extract reusable test helpers for future tests across the project.

## 2. Background & Context

During RFC-007 implementation, we discovered that some unit tests (lines 26-283 in `analysis_test.go`) are testing **isolated logic simulations** rather than the actual implementation functions:

- `TestShouldAnalyzeMapping` simulates logic with `nextAnalysisStr == ""` instead of calling `shouldAnalyzeMapping()`
- `TestAnalyzeTracks` tests the `without()` helper instead of calling `analyzeTracks()`
- `TestAnalyzePlaylistNames` reimplements canonical name logic instead of calling `analyzePlaylistNames()`

While we have integration tests that work correctly, **all tests should test the real implementation** to catch actual bugs and ensure proper PocketBase integration.

Additionally, we've developed valuable test helpers (`setupTestApp`, `setupOAuthTokens`, etc.) that should be extracted into shared utilities for future test suites across the project.

### Key Insights from RFC-007 Implementation:

- **PocketBase Relation Fields**: Stored as `[]string` arrays, need special handling to access
- **Date Format Handling**: PocketBase uses `2006-01-02 15:04:05.000Z` format, not RFC3339
- **Filter Requirements**: Empty string filters don't work, need `id != ''` pattern
- **HTTP Mocking**: Requires careful setup for background jobs that don't use Echo context
- **Collection Schema**: Need proper relation options, required fields, and select values

## 3. Technical Design

### 3.1 Test Refactoring Strategy

**Current Issues:**
```go
// WRONG: Testing isolated logic simulation
t.Run("should analyze when next_analysis_at is empty", func(t *testing.T) {
    nextAnalysisStr := ""
    result := nextAnalysisStr == ""  // Simulated logic
    // ...
})
```

**Correct Approach:**
```go
// RIGHT: Testing actual function with PocketBase
t.Run("should analyze when next_analysis_at is empty", func(t *testing.T) {
    testApp := setupTestApp(t)
    defer testApp.Cleanup()
    
    // Create real mapping record with empty next_analysis_at
    mapping := createTestMapping(testApp, map[string]interface{}{
        "spotify_playlist_id": "test_playlist",
        // next_analysis_at left empty
    })
    
    // Test actual function
    result := shouldAnalyzeMapping(mapping, time.Now())
    assert.True(t, result)
})
```

### 3.2 Shared Test Helpers Location

Create `backend/internal/testhelpers/` package with:

**File: `backend/internal/testhelpers/pocketbase.go`**
```go
package testhelpers

import (
    "testing"
    "time"
    "github.com/pocketbase/pocketbase/tests"
    "github.com/pocketbase/pocketbase/models"
    "github.com/pocketbase/pocketbase/models/schema"
    "github.com/stretchr/testify/require"
)

// SetupTestApp creates a test PocketBase app with standard collections
func SetupTestApp(t *testing.T) *tests.TestApp {
    testApp, err := tests.NewTestApp()
    require.NoError(t, err)
    
    CreateStandardCollections(t, testApp)
    return testApp
}

// CreateStandardCollections creates collections used across multiple tests
func CreateStandardCollections(t *testing.T, testApp *tests.TestApp) {
    CreateOAuthTokensCollection(t, testApp)
    CreateMappingsCollection(t, testApp)
    CreateSyncItemsCollection(t, testApp)
}

// SetupOAuthTokens creates test OAuth tokens for both services
func SetupOAuthTokens(t *testing.T, testApp *tests.TestApp) {
    // Implementation from analysis_test.go
}

// CreateTestMapping creates a mapping record with given properties
func CreateTestMapping(testApp *tests.TestApp, properties map[string]interface{}) *models.Record {
    // Helper to create mapping records easily
}
```

**File: `backend/internal/testhelpers/http_mocking.go`**
```go
package testhelpers

import (
    "testing"
    "github.com/jarcoal/httpmock"
)

// SetupAPIHttpMocks configures HTTP mocks for Spotify and YouTube APIs
func SetupAPIHttpMocks(t *testing.T) {
    // Implementation from analysis_test.go setupHTTPMocks
}

// SetupSpotifyMocks configures only Spotify API mocks
func SetupSpotifyMocks(t *testing.T, tracks map[string]interface{}) {
    // Spotify-specific mocking
}

// SetupYouTubeMocks configures only YouTube API mocks  
func SetupYouTubeMocks(t *testing.T, items map[string]interface{}) {
    // YouTube-specific mocking
}
```

### 3.3 Refactored Test Structure

**Target Test Pattern:**
```go
func TestShouldAnalyzeMapping_ActualImplementation(t *testing.T) {
    testApp := testhelpers.SetupTestApp(t)
    defer testApp.Cleanup()
    
    t.Run("should analyze when next_analysis_at is empty", func(t *testing.T) {
        mapping := testhelpers.CreateTestMapping(testApp, map[string]interface{}{
            "spotify_playlist_id": "test_playlist",
            // next_analysis_at left empty
        })
        
        result := shouldAnalyzeMapping(mapping, time.Now())
        assert.True(t, result)
    })
    
    t.Run("should analyze when next_analysis_at is in the past", func(t *testing.T) {
        pastTime := time.Now().Add(-1 * time.Hour)
        mapping := testhelpers.CreateTestMapping(testApp, map[string]interface{}{
            "spotify_playlist_id": "test_playlist",
            "next_analysis_at": pastTime.Format("2006-01-02 15:04:05.000Z"),
        })
        
        result := shouldAnalyzeMapping(mapping, time.Now())
        assert.True(t, result)
    })
    
    // ... more test cases using actual function
}
```

## 4. Dependencies

* **Testing Libraries:** Already have `testify`, `httpmock`
* **PocketBase Test Framework:** Already using `tests.NewTestApp()`
* **No new external dependencies required**

## 5. Checklist

- [X] **T1**: Create `backend/internal/testhelpers/` package structure
- [X] **T2**: Extract `SetupTestApp()` function into `testhelpers/pocketbase.go`
- [X] **T3**: Extract collection creation helpers (`CreateOAuthTokensCollection`, etc.)
- [X] **T4**: Extract `SetupOAuthTokens()` into shared helper
- [X] **T5**: Extract HTTP mocking functions into `testhelpers/http_mocking.go`
- [X] **T6**: Create `CreateTestMapping()` helper for easy mapping creation
- [X] **T7**: Refactor `TestShouldAnalyzeMapping` to use actual `shouldAnalyzeMapping()` function
- [X] **T8**: Refactor `TestAnalyzeTracks` to use actual `analyzeTracks()` function with PocketBase
- [X] **T9**: Refactor `TestAnalyzePlaylistNames` to use actual `analyzePlaylistNames()` function
- [X] **T10**: Refactor `TestUpdateMappingAnalysisTime` to use actual function with PocketBase
- [X] **T11**: Update existing integration tests to use new shared helpers
- [X] **T12**: **CRITICAL**: Run full test suite (`make test-backend`) to ensure no regressions
- [X] **T13**: Update test documentation in README about shared helpers availability

## 6. Definition of Done

* All unit tests call actual implementation functions with real PocketBase records
* Shared test helpers extracted and documented for future use
* All existing tests still pass (no regressions)
* Test helpers are reusable across different test suites
* Clear documentation on how to use shared helpers for future tests

## 7. Implementation Notes / Summary

**T1 COMPLETED** - Created testhelpers package structure:
* Created directory `backend/internal/testhelpers/` for shared test utilities
* Package ready to receive extracted helper functions from analysis_test.go
* Location follows Go project conventions for internal packages

**T2, T3, T4 COMPLETED** - Extracted core PocketBase test helpers:
* Created `backend/internal/testhelpers/pocketbase.go` with complete test setup infrastructure
* **SetupTestApp()**: Main entry point that creates test app with all standard collections
* **CreateStandardCollections()**: Orchestrates creation of all test collections in proper order
* **CreateOAuthTokensCollection()**: Creates oauth_tokens collection with proper schema (provider, access_token, refresh_token, expiry, scopes)
* **CreateMappingsCollection()**: Creates mappings collection with all RFC-007 fields including last_analysis_at and next_analysis_at
* **CreateSyncItemsCollection()**: Creates sync_items collection with proper relation to mappings and cascade delete
* **SetupOAuthTokens()**: Creates fake Spotify and Google OAuth tokens for testing
* **CreateTestMapping()**: Helper to easily create mapping records with default values and custom properties
* All functions include proper error handling with require.NoError() assertions
* Maintains exact schema compatibility with existing tests and production migrations

**T5 COMPLETED** - Extracted HTTP mocking functions:
* Created `backend/internal/testhelpers/http_mocking.go` with comprehensive HTTP mocking infrastructure
* **SetupAPIHttpMocks()**: Main entry point that configures mocks for both Spotify and YouTube APIs with default test data
* **SetupSpotifyMocks()**: Configures Spotify Web API playlist tracks endpoint with custom track data
* **SetupYouTubeMocks()**: Configures YouTube Data API playlist items endpoint with custom video data  
* **SetupOAuthRefreshMocks()**: Configures OAuth token refresh endpoints for both Spotify and Google services
* **SetupIdenticalPlaylistMocks()**: Special helper for testing no-change scenarios with identical playlist content
* All mocks use proper regex patterns compatible with background job HTTP calls (uses http.DefaultTransport)
* Includes logging for API calls to help with test debugging
* Supports flexible mock data configuration for different test scenarios

**T7 COMPLETED** - Refactored TestShouldAnalyzeMapping to use actual implementation:
* Replaced simulated logic testing with real `shouldAnalyzeMapping()` function calls
* **OLD**: Tests simulated `nextAnalysisStr == ""` and manual time parsing logic 
* **NEW**: Tests actual PocketBase records with real database operations
* Uses proper PocketBase date format `2006-01-02 15:04:05.000Z` instead of RFC3339
* Creates real mapping records in test database and calls actual `shouldAnalyzeMapping(mapping, now)`
* Validates all timing scenarios: empty next_analysis_at, past time, future time, invalid format
* Uses UTC time throughout to avoid timezone issues in CI/testing environments
* Proper error handling with `require.NoError()` for database operations

**T8 COMPLETED** - Refactored TestAnalyzeTracks to use actual implementation:
* Replaced `without()` helper simulation with real `analyzeTracks()` function calls
* **OLD**: Tests simulated `lo.Without()` behavior manually with string arrays
* **NEW**: Tests actual track difference analysis with TrackList structs and PocketBase integration
* Creates real mapping records and calls `analyzeTracks(testApp, mapping, spotifyTracks, youtubeTracks)`
* Tests bidirectional sync item creation: tracks missing from each service are queued for addition
* Validates proper sync_items record creation with correct service, action, and payload fields
* Tests edge cases: no overlapping tracks (6 items), identical tracks (0 items), partial overlap (2 items)
* Clears sync items between test runs to ensure clean isolated testing
* Validates JSON payload structure with track IDs for proper work queue creation

**T9 COMPLETED** - Refactored TestAnalyzePlaylistNames to use actual implementation:
* Replaced simulated canonical name logic with real `analyzePlaylistNames()` function calls
* **OLD**: Tests simulated if/else conditional logic manually with string variables
* **NEW**: Tests actual playlist name analysis with PocketBase mapping records and sync item creation
* Creates real mapping records with different playlist name scenarios and calls `analyzePlaylistNames(testApp, mapping, emptyTracks, emptyTracks)`
* Tests canonical naming logic: YouTube name is canonical by default, so Spotify gets renamed to match YouTube
* Validates proper sync_items creation with "rename_playlist" action and new_name payload
* Tests all edge cases: different names (1 rename item), identical names (0 items), empty YouTube name (0 items), empty Spotify name (0 items)  
* Clears sync items between test runs for proper isolation and validates actual RFC-007 implementation behavior
* Confirms that both names must be non-empty for rename analysis to trigger (matches actual function logic)

**T10 COMPLETED** - Refactored TestUpdateMappingAnalysisTime to use actual implementation:
* Replaced simulated duration calculations with real `updateMappingAnalysisTime()` function calls
* **OLD**: Tests manually calculated `time.Duration(intervalMinutes) * time.Minute` without database operations
* **NEW**: Tests actual timestamp update with PocketBase mapping records and database persistence
* Creates real mapping records with different interval_minutes values and calls `updateMappingAnalysisTime(testApp, mapping, now)`
* Tests default interval behavior (0 → 60 minutes), custom intervals (30 minutes), and long intervals (720 minutes)
* Validates proper PocketBase date format `2006-01-02 15:04:05.000Z` for both last_analysis_at and next_analysis_at fields
* Verifies timestamp calculations with tolerance for test timing variations (±1 second)
* Confirms database persistence by reloading records and parsing stored timestamp values
* Validates that last_analysis_at matches the provided time and next_analysis_at correctly reflects the interval

**T11 COMPLETED** - Updated existing integration tests to use new shared helpers:
* Replaced all `setupTestApp(t)` calls with `testhelpers.SetupTestApp(t)` throughout analysis_test.go
* Replaced `setupOAuthTokens(t, testApp)` calls with `testhelpers.SetupOAuthTokens(t, testApp)`
* Replaced `setupHTTPMocks(t)` calls with `testhelpers.SetupAPIHttpMocks(t)`
* Used specialized helper `testhelpers.SetupIdenticalPlaylistMocks(t)` for no-change scenarios
* **Removed duplicate code**: Deleted 150+ lines of duplicated setup functions from analysis_test.go
* **Cleaner imports**: Added proper module import `github.com/manlikeabro/spotube/internal/testhelpers`
* **Centralized setup**: All test setup now uses shared, reusable helpers ensuring consistency
* **Maintained functionality**: All existing integration tests continue to work with new shared helpers
* Tests now use DRY principle with shared infrastructure instead of copy-pasted setup code

**T12 COMPLETED** - Full test suite validation passed:
* ✅ **ALL TESTS PASSING**: Ran `go test ./...` in backend directory with 100% success rate
* ✅ **No regressions introduced**: All existing functionality intact after refactoring
* ✅ **Shared helpers working**: New testhelpers package functions correctly across all test files
* **Test results summary:**
  - `internal/jobs` package: All tests pass (0.599s)
  - `internal/pbext/googleauth`: All tests pass (cached)
  - `internal/pbext/mappings`: All tests pass (cached) 
  - `internal/pbext/setupwizard`: All tests pass (cached)
  - `internal/pbext/spotifyauth`: All tests pass (cached)
* **Validation confirms**: RFC-007b refactoring successful, tests are more maintainable and continue to validate real implementation behavior
* **Zero tolerance for regressions met**: All pre-existing tests still pass, proving refactoring preserved functionality

**T13 COMPLETED** - Updated test documentation in README:
* Added comprehensive "Testing" section to README.md documenting shared helpers infrastructure  
* **Available Test Helpers**: Documented all testhelpers functions with clear descriptions
  - `SetupTestApp(t)` - PocketBase test instance creation
  - `SetupOAuthTokens(t, testApp)` - OAuth token setup
  - `CreateTestMapping(testApp, properties)` - Mapping record creation
  - `SetupAPIHttpMocks(t)` - HTTP API mocking
  - `SetupIdenticalPlaylistMocks(t)` - No-change scenarios
* **Usage Example**: Complete Go code example showing proper test setup pattern
* **Key Testing Principles**: Emphasized real implementation testing vs mocked logic
* **Running Tests**: Clear commands for backend, frontend, and combined test execution
* **Location**: Added to README.md before Contributing section for easy discoverability
* **For Future RFCs**: Documentation enables other developers to leverage shared helpers consistently

### Key Insights from RFC-007 for Test Implementation:

**PocketBase-Specific Testing Patterns:**
* **Relation Field Access**: Use `rawValue := record.Get("mapping_id")` then cast to `[]string` and access `[0]`
* **Date Format**: Always use `2006-01-02 15:04:05.000Z` format for PocketBase dates
* **Filter Syntax**: Use `id != ''` instead of empty string for "all records" queries
* **Collection Schema**: Proper relation options with `CollectionId`, `CascadeDelete`, etc.

**HTTP Mocking for Background Jobs:**
* Use `http.DefaultTransport` to ensure httpmock interception
* Configure both OAuth token refresh endpoints (Spotify + Google)
* Use flexible regex patterns: `=~^https://api\.spotify\.com/v1/playlists/.*/tracks`

**Test Data Patterns:**
```go
// Spotify API Mock Response
spotifyTracks := map[string]interface{}{
    "items": []map[string]interface{}{
        {"track": map[string]interface{}{"id": "track1", "name": "Song 1"}},
    },
}

// YouTube API Mock Response  
youtubeItems := map[string]interface{}{
    "items": []map[string]interface{}{
        {"id": "item1", "snippet": map[string]interface{}{"title": "Song 1"}},
    },
}
```

**Error Handling Patterns:**
* Always use `require.NoError(t, err)` for setup that must succeed
* Use `assert.NoError(t, err)` for test assertions
* Handle timezone differences with `time.Now().UTC()` for consistent testing

### References to RFC-007:
* **Section 3.1**: Collection schema definitions → Use for shared collection helpers
* **Section 3.3**: Analysis algorithm → Basis for refactored function tests
* **A4 Implementation**: OAuth helpers → Pattern for shared auth setup
* **A5 Implementation**: Integration test patterns → Model for all test refactoring

---

*End of RFC-007b* 