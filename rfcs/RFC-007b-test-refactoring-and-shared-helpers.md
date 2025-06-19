# RFC-007b: Test Refactoring and Shared Helpers

**Status:** Draft  
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

- [ ] **T1**: Create `backend/internal/testhelpers/` package structure
- [ ] **T2**: Extract `SetupTestApp()` function into `testhelpers/pocketbase.go`
- [ ] **T3**: Extract collection creation helpers (`CreateOAuthTokensCollection`, etc.)
- [ ] **T4**: Extract `SetupOAuthTokens()` into shared helper
- [ ] **T5**: Extract HTTP mocking functions into `testhelpers/http_mocking.go`
- [ ] **T6**: Create `CreateTestMapping()` helper for easy mapping creation
- [ ] **T7**: Refactor `TestShouldAnalyzeMapping` to use actual `shouldAnalyzeMapping()` function
- [ ] **T8**: Refactor `TestAnalyzeTracks` to use actual `analyzeTracks()` function with PocketBase
- [ ] **T9**: Refactor `TestAnalyzePlaylistNames` to use actual `analyzePlaylistNames()` function
- [ ] **T10**: Refactor `TestUpdateMappingAnalysisTime` to use actual function with PocketBase
- [ ] **T11**: Update existing integration tests to use new shared helpers
- [ ] **T12**: **CRITICAL**: Run full test suite (`make test-backend`) to ensure no regressions
- [ ] **T13**: Update test documentation in README about shared helpers availability

## 6. Definition of Done

* All unit tests call actual implementation functions with real PocketBase records
* Shared test helpers extracted and documented for future use
* All existing tests still pass (no regressions)
* Test helpers are reusable across different test suites
* Clear documentation on how to use shared helpers for future tests

## 7. Implementation Notes / Summary

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