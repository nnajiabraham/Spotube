package jobs

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/manlikeabro/spotube/internal/testhelpers"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Interface bridging for TestApp -> PocketBase compatibility
type testAppWrapper struct {
	*tests.TestApp
}

func (w *testAppWrapper) Dao() *daos.Dao {
	return w.TestApp.Dao()
}

func TestProcessQueue_NoItems(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	ctx := context.Background()
	wrapper := &testAppWrapper{testApp}

	// Should handle empty queue gracefully
	err := ProcessQueue(wrapper, ctx)
	assert.NoError(t, err)
}

func TestProcessSyncItem_Success(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	wrapper := &testAppWrapper{testApp}

	// Create a test sync item
	syncItem := createTestSyncItem(t, testApp, map[string]interface{}{
		"service": "spotify",
		"action":  "add_track",
		"payload": `{"track_id":"test123"}`,
		"status":  "pending",
	})

	// Process the item
	err := processSyncItem(wrapper, syncItem)
	assert.NoError(t, err) // Processing itself should succeed

	// Reload to check updated status
	updatedItem, err := testApp.Dao().FindRecordById("sync_items", syncItem.Id)
	require.NoError(t, err)

	assert.Equal(t, "pending", updatedItem.GetString("status"), "Item should remain pending after temporary error")
	assert.Equal(t, 1, updatedItem.GetInt("attempts"), "Item should have 1 attempt")

	// Check that the error message indicates the actual implementation was called
	lastError := updatedItem.GetString("last_error")
	assert.Contains(t, lastError, "no Spotify token found", "Error should indicate real implementation was called")
}

func TestProcessSyncItem_StatusTransition(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	// Create a test sync item
	syncItem := createTestSyncItem(t, testApp, map[string]interface{}{
		"service": "spotify",
		"action":  "add_track",
		"payload": `{"track_id":"test123"}`,
		"status":  "pending",
	})

	wrapper := &testAppWrapper{testApp}

	// Process the item
	err := processSyncItem(wrapper, syncItem)
	assert.NoError(t, err)

	// Item should be marked as running initially, then pending due to "not implemented"
	assert.Equal(t, "pending", syncItem.GetString("status"))
	assert.Equal(t, 1, syncItem.GetInt("attempts"))
}

func TestProcessSyncItem_RateLimitRetry(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	// Create a test sync item with initial backoff
	syncItem := createTestSyncItem(t, testApp, map[string]interface{}{
		"service":              "spotify",
		"action":               "add_track",
		"payload":              `{"track_id":"test123"}`,
		"status":               "pending",
		"attempts":             0,
		"attempt_backoff_secs": 30,
	})

	wrapper := &testAppWrapper{testApp}

	// Test handleRetry function directly with rate limit error
	timeBeforeRetry := time.Now().UTC() // Use UTC for consistency
	err := handleRetry(wrapper, syncItem, "rate_limit", fmt.Errorf("429 too many requests"))
	assert.NoError(t, err)

	assert.Equal(t, "pending", syncItem.GetString("status"))
	assert.Equal(t, 0, syncItem.GetInt("attempts")) // attempts was not incremented in handleRetry
	assert.Contains(t, syncItem.GetString("last_error"), "rate_limit")

	// Check exponential backoff calculation: min(2^0 * 30, 3600) = 30
	expectedBackoff := int(math.Min(math.Pow(2, 0)*30, 3600))
	assert.Equal(t, expectedBackoff, syncItem.GetInt("attempt_backoff_secs"))

	// Check next_attempt_at is set to future time
	nextAttemptStr := syncItem.GetString("next_attempt_at")
	assert.NotEmpty(t, nextAttemptStr)

	nextAttempt, err := time.Parse("2006-01-02 15:04:05.000Z", nextAttemptStr)
	assert.NoError(t, err)

	// Add some debug info and use a more lenient comparison
	timeDiff := nextAttempt.Sub(timeBeforeRetry)
	t.Logf("Time before retry (UTC): %v", timeBeforeRetry)
	t.Logf("Next attempt (UTC): %v", nextAttempt)
	t.Logf("Difference: %v", timeDiff)

	// Should be at least 25 seconds in the future (allowing for some timing variance)
	assert.True(t, timeDiff >= 25*time.Second, "next_attempt_at should be at least 25 seconds in the future, got %v", timeDiff)
}

func TestProcessSyncItem_ExponentialBackoff(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	wrapper := &testAppWrapper{testApp}

	// Test different attempt levels
	testCases := []struct {
		attempts        int
		expectedBackoff int
	}{
		{0, 30},    // 2^0 * 30 = 30
		{1, 60},    // 2^1 * 30 = 60
		{2, 120},   // 2^2 * 30 = 120
		{3, 240},   // 2^3 * 30 = 240
		{10, 3600}, // 2^10 * 30 = 30720, capped at 3600
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("attempts_%d", tc.attempts), func(t *testing.T) {
			// Create unique mapping for each test to avoid conflicts
			uniquePlaylistID := fmt.Sprintf("test_playlist_%d_%d", tc.attempts, time.Now().UnixNano())
			syncItem := createTestSyncItem(t, testApp, map[string]interface{}{
				"attempts":            tc.attempts,
				"service":             "spotify",
				"action":              "add_track",
				"spotify_playlist_id": uniquePlaylistID,
				"youtube_playlist_id": uniquePlaylistID + "_yt",
			})
			require.NotNil(t, syncItem, "syncItem should not be nil")

			err := handleRetry(wrapper, syncItem, "temporary", fmt.Errorf("test error"))
			assert.NoError(t, err)

			assert.Equal(t, tc.expectedBackoff, syncItem.GetInt("attempt_backoff_secs"))
		})
	}
}

func TestYouTubeQuotaTracker_Basic(t *testing.T) {
	// Reset quota tracker for test
	tracker := &YouTubeQuotaTracker{}

	// Should allow consumption within limit
	success := tracker.checkAndConsumeQuota(50)
	assert.True(t, success)

	used, limit, _ := tracker.getCurrentUsage()
	assert.Equal(t, 50, used)
	assert.Equal(t, YOUTUBE_DAILY_QUOTA, limit)
}

func TestYouTubeQuotaTracker_Exhaustion(t *testing.T) {
	// Reset quota tracker for test
	tracker := &YouTubeQuotaTracker{}

	// Consume most of the quota
	success := tracker.checkAndConsumeQuota(YOUTUBE_DAILY_QUOTA - 10)
	assert.True(t, success)

	// Should reject consumption that would exceed limit
	success = tracker.checkAndConsumeQuota(20)
	assert.False(t, success)

	// Should still allow consumption within remaining quota
	success = tracker.checkAndConsumeQuota(5)
	assert.True(t, success)
}

func TestYouTubeQuotaTracker_DailyReset(t *testing.T) {
	// Reset quota tracker for test with yesterday's date
	tracker := &YouTubeQuotaTracker{
		used:      5000,
		resetDate: time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02"), // Yesterday
	}

	// Should reset when checking quota on new day
	success := tracker.checkAndConsumeQuota(50)
	assert.True(t, success)

	used, _, resetDate := tracker.getCurrentUsage()
	assert.Equal(t, 50, used) // Should be reset + new consumption
	assert.Equal(t, time.Now().UTC().Format("2006-01-02"), resetDate)
}

func TestExecuteYouTubeAddTrack_QuotaExhausted(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	// Create sync item
	syncItem := createTestSyncItem(t, testApp, map[string]interface{}{
		"service": "youtube",
		"action":  "add_track",
		"payload": `{"track_id":"test123"}`,
		"status":  "pending",
	})

	// Exhaust quota by setting it very high
	youtubeQuota.mu.Lock()
	youtubeQuota.used = YOUTUBE_DAILY_QUOTA - 10 // Almost exhausted
	youtubeQuota.resetDate = time.Now().UTC().Format("2006-01-02")
	youtubeQuota.mu.Unlock()

	wrapper := &testAppWrapper{testApp}

	// Execute action (should skip due to quota)
	err := executeYouTubeAddTrack(wrapper, syncItem, `{"track_id":"test123"}`)
	assert.NoError(t, err) // Should not error, just skip

	// Check item was marked as skipped
	assert.Equal(t, "skipped", syncItem.GetString("status"))
	assert.Equal(t, "quota", syncItem.GetString("last_error"))
}

func TestErrorClassification(t *testing.T) {
	tests := []struct {
		err         error
		isRateLimit bool
		isFatal     bool
	}{
		{fmt.Errorf("429 too many requests"), true, false},
		{fmt.Errorf("rate limit exceeded"), true, false},
		{fmt.Errorf("Too Many Requests"), true, false},
		{fmt.Errorf("404 not found"), false, true},
		{fmt.Errorf("403 forbidden"), false, true},
		{fmt.Errorf("401 unauthorized"), false, true},
		{fmt.Errorf("invalid request"), false, true},
		{fmt.Errorf("500 internal server error"), false, false},
		{fmt.Errorf("network timeout"), false, false},
	}

	for _, test := range tests {
		t.Run(test.err.Error(), func(t *testing.T) {
			assert.Equal(t, test.isRateLimit, isRateLimitError(test.err))
			assert.Equal(t, test.isFatal, isFatalError(test.err))
		})
	}
}

func TestTruncateError(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a very long error message", 10, "this is..."},
		{"exactly10", 10, "exactly10"},
		{"", 5, ""},
	}

	for _, test := range tests {
		result := truncateError(test.input, test.maxLen)
		assert.Equal(t, test.expected, result)
		assert.True(t, len(result) <= test.maxLen)
	}
}

// Helper function to create test sync items
func createTestSyncItem(t *testing.T, testApp *tests.TestApp, properties map[string]interface{}) *models.Record {
	// Create a unique mapping for this test to avoid conflicts
	spotifyPlaylistID := "test_spotify_playlist"
	youtubePlaylistID := "test_youtube_playlist"

	// Use unique playlist IDs if provided
	if id, ok := properties["spotify_playlist_id"]; ok {
		spotifyPlaylistID = id.(string)
		delete(properties, "spotify_playlist_id") // Remove from properties so it's not set on sync item
	}
	if id, ok := properties["youtube_playlist_id"]; ok {
		youtubePlaylistID = id.(string)
		delete(properties, "youtube_playlist_id") // Remove from properties so it's not set on sync item
	}

	// First create a mapping for the relation
	mapping := testhelpers.CreateTestMapping(testApp, map[string]interface{}{
		"spotify_playlist_id": spotifyPlaylistID,
		"youtube_playlist_id": youtubePlaylistID,
	})
	require.NotNil(t, mapping, "CreateTestMapping should not return nil")

	collection, err := testApp.Dao().FindCollectionByNameOrId("sync_items")
	require.NoError(t, err)

	record := models.NewRecord(collection)

	// Set defaults
	record.Set("mapping_id", mapping.Id)
	record.Set("service", "spotify")
	record.Set("action", "add_track")
	record.Set("payload", `{"track_id":"default"}`)
	record.Set("status", "pending")
	record.Set("attempts", 0)
	record.Set("next_attempt_at", time.Now().Format("2006-01-02 15:04:05.000Z"))
	record.Set("attempt_backoff_secs", 30)

	// Override with provided properties
	for key, value := range properties {
		record.Set(key, value)
	}

	err = testApp.Dao().SaveRecord(record)
	require.NoError(t, err)

	return record
}

func TestExecutorActions_ActualImplementations(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	// Setup OAuth tokens and API mocks
	testhelpers.SetupOAuthTokens(t, testApp)
	testhelpers.SetupAPIHttpMocks(t)
	defer httpmock.DeactivateAndReset()

	// Set environment variables for OAuth
	os.Setenv("SPOTIFY_CLIENT_ID", "test_spotify_id")
	os.Setenv("SPOTIFY_CLIENT_SECRET", "test_spotify_secret")
	os.Setenv("GOOGLE_CLIENT_ID", "test_google_id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test_google_secret")
	defer func() {
		os.Unsetenv("SPOTIFY_CLIENT_ID")
		os.Unsetenv("SPOTIFY_CLIENT_SECRET")
		os.Unsetenv("GOOGLE_CLIENT_ID")
		os.Unsetenv("GOOGLE_CLIENT_SECRET")
	}()

	wrapper := &testAppWrapper{testApp}

	t.Run("executeSpotifyAddTrack success", func(t *testing.T) {
		// Setup Spotify API mock for adding tracks
		httpmock.RegisterResponder("POST", `=~^https://api\.spotify\.com/v1/playlists/.*/tracks`,
			httpmock.NewJsonResponderOrPanic(201, map[string]interface{}{
				"snapshot_id": "test_snapshot_123",
			}))

		// Create test sync item with unique playlist IDs for this test
		uniqueSpotifyId := fmt.Sprintf("test_spotify_playlist_%d", time.Now().UnixNano())
		uniqueYouTubeId := fmt.Sprintf("test_youtube_playlist_%d", time.Now().UnixNano())

		syncItem := createTestSyncItem(t, testApp, map[string]interface{}{
			"spotify_playlist_id": uniqueSpotifyId,
			"youtube_playlist_id": uniqueYouTubeId,
			"service":             "spotify",
			"action":              "add_track",
			"payload":             `{"track_id":"spotify_track_123"}`,
			"status":              "running",
		})

		// Execute action
		err := executeSpotifyAddTrack(wrapper, syncItem, `{"track_id":"spotify_track_123"}`)
		assert.NoError(t, err)

		// Verify API was called
		info := httpmock.GetCallCountInfo()
		spotifyAddCalls := 0
		for url, count := range info {
			if strings.Contains(url, "spotify.com") && strings.Contains(url, "/tracks") {
				spotifyAddCalls += count
			}
		}
		assert.True(t, spotifyAddCalls > 0, "Spotify add track API should have been called")
	})

	t.Run("executeYouTubeAddTrack success", func(t *testing.T) {
		// Reset quota for this test
		youtubeQuota.mu.Lock()
		youtubeQuota.used = 0
		youtubeQuota.resetDate = time.Now().UTC().Format("2006-01-02")
		youtubeQuota.mu.Unlock()

		// Setup YouTube API mock for adding tracks
		httpmock.RegisterResponder("POST", `=~^https://.*youtube.*playlistItems`,
			httpmock.NewJsonResponderOrPanic(200, map[string]interface{}{
				"id": "test_playlist_item_id",
				"snippet": map[string]interface{}{
					"playlistId": "test_youtube_playlist_456",
					"resourceId": map[string]interface{}{
						"kind":    "youtube#video",
						"videoId": "youtube_track_123",
					},
				},
			}))

		// Create test sync item with unique playlist IDs for this test
		uniqueSpotifyId := fmt.Sprintf("test_spotify_playlist_%d", time.Now().UnixNano())
		uniqueYouTubeId := fmt.Sprintf("test_youtube_playlist_%d", time.Now().UnixNano())

		syncItem := createTestSyncItem(t, testApp, map[string]interface{}{
			"spotify_playlist_id": uniqueSpotifyId,
			"youtube_playlist_id": uniqueYouTubeId,
			"service":             "youtube",
			"action":              "add_track",
			"payload":             `{"track_id":"youtube_track_123"}`,
			"status":              "running",
		})

		// Execute action
		err := executeYouTubeAddTrack(wrapper, syncItem, `{"track_id":"youtube_track_123"}`)
		assert.NoError(t, err)

		// Verify quota was consumed
		used, _, _ := youtubeQuota.getCurrentUsage()
		assert.Equal(t, YOUTUBE_ADD_TRACK_COST, used)
	})

	t.Run("executeSpotifyRenamePlaylist success", func(t *testing.T) {
		// Setup Spotify API mock for renaming playlist
		httpmock.RegisterResponder("PUT", `=~^https://api\.spotify\.com/v1/playlists/.*`,
			httpmock.NewStringResponder(200, ""))

		// Create test sync item with unique playlist IDs for this test
		uniqueSpotifyId := fmt.Sprintf("test_spotify_playlist_%d", time.Now().UnixNano())
		uniqueYouTubeId := fmt.Sprintf("test_youtube_playlist_%d", time.Now().UnixNano())

		syncItem := createTestSyncItem(t, testApp, map[string]interface{}{
			"spotify_playlist_id": uniqueSpotifyId,
			"youtube_playlist_id": uniqueYouTubeId,
			"service":             "spotify",
			"action":              "rename_playlist",
			"payload":             `{"new_name":"My New Playlist Name"}`,
			"status":              "running",
		})

		// Execute action
		err := executeSpotifyRenamePlaylist(wrapper, syncItem, `{"new_name":"My New Playlist Name"}`)
		assert.NoError(t, err)

		// Verify API was called
		info := httpmock.GetCallCountInfo()
		spotifyRenameCalls := 0
		for url, count := range info {
			if strings.Contains(url, "spotify.com") && strings.Contains(url, "playlists") {
				spotifyRenameCalls += count
			}
		}
		assert.True(t, spotifyRenameCalls > 0, "Spotify rename playlist API should have been called")
	})

	t.Run("executeYouTubeRenamePlaylist success", func(t *testing.T) {
		// Reset quota for this test
		youtubeQuota.mu.Lock()
		youtubeQuota.used = 0
		youtubeQuota.resetDate = time.Now().UTC().Format("2006-01-02")
		youtubeQuota.mu.Unlock()

		// Setup YouTube API mock for renaming playlist
		httpmock.RegisterResponder("PUT", `=~^https://.*youtube.*playlists`,
			httpmock.NewJsonResponderOrPanic(200, map[string]interface{}{
				"id": "test_youtube_playlist_456",
				"snippet": map[string]interface{}{
					"title": "My New YouTube Playlist Name",
				},
			}))

		// Create test sync item with unique playlist IDs for this test
		uniqueSpotifyId := fmt.Sprintf("test_spotify_playlist_%d", time.Now().UnixNano())
		uniqueYouTubeId := fmt.Sprintf("test_youtube_playlist_%d", time.Now().UnixNano())

		syncItem := createTestSyncItem(t, testApp, map[string]interface{}{
			"spotify_playlist_id": uniqueSpotifyId,
			"youtube_playlist_id": uniqueYouTubeId,
			"service":             "youtube",
			"action":              "rename_playlist",
			"payload":             `{"new_name":"My New YouTube Playlist Name"}`,
			"status":              "running",
		})

		// Execute action
		err := executeYouTubeRenamePlaylist(wrapper, syncItem, `{"new_name":"My New YouTube Playlist Name"}`)
		assert.NoError(t, err)

		// Verify quota was consumed (minimal cost for rename)
		used, _, _ := youtubeQuota.getCurrentUsage()
		assert.Equal(t, 1, used)
	})

	t.Run("executeSpotifyAddTrack invalid payload", func(t *testing.T) {
		// Create test sync item with unique playlist IDs for this test
		uniqueSpotifyId := fmt.Sprintf("test_spotify_playlist_%d", time.Now().UnixNano())
		uniqueYouTubeId := fmt.Sprintf("test_youtube_playlist_%d", time.Now().UnixNano())

		syncItem := createTestSyncItem(t, testApp, map[string]interface{}{
			"spotify_playlist_id": uniqueSpotifyId,
			"youtube_playlist_id": uniqueYouTubeId,
			"service":             "spotify",
			"action":              "add_track",
			"payload":             `invalid json`,
			"status":              "running",
		})

		err := executeSpotifyAddTrack(wrapper, syncItem, `invalid json`)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse payload")
	})

	t.Run("executeYouTubeAddTrack missing track_id", func(t *testing.T) {
		// Create test sync item with unique playlist IDs for this test
		uniqueSpotifyId := fmt.Sprintf("test_spotify_playlist_%d", time.Now().UnixNano())
		uniqueYouTubeId := fmt.Sprintf("test_youtube_playlist_%d", time.Now().UnixNano())

		syncItem := createTestSyncItem(t, testApp, map[string]interface{}{
			"spotify_playlist_id": uniqueSpotifyId,
			"youtube_playlist_id": uniqueYouTubeId,
			"service":             "youtube",
			"action":              "add_track",
			"payload":             `{"wrong_field":"value"}`,
			"status":              "running",
		})

		err := executeYouTubeAddTrack(wrapper, syncItem, `{"wrong_field":"value"}`)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "track_id not found in payload")
	})

	t.Run("executeSpotifyRenamePlaylist missing new_name", func(t *testing.T) {
		// Create test sync item with unique playlist IDs for this test
		uniqueSpotifyId := fmt.Sprintf("test_spotify_playlist_%d", time.Now().UnixNano())
		uniqueYouTubeId := fmt.Sprintf("test_youtube_playlist_%d", time.Now().UnixNano())

		syncItem := createTestSyncItem(t, testApp, map[string]interface{}{
			"spotify_playlist_id": uniqueSpotifyId,
			"youtube_playlist_id": uniqueYouTubeId,
			"service":             "spotify",
			"action":              "rename_playlist",
			"payload":             `{"old_name":"Old Name"}`,
			"status":              "running",
		})

		err := executeSpotifyRenamePlaylist(wrapper, syncItem, `{"old_name":"Old Name"}`)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "new_name not found in payload")
	})

	t.Run("executeYouTubeRenamePlaylist mapping not found", func(t *testing.T) {
		// Create test sync item but with an invalid mapping_id directly
		collection, err := testApp.Dao().FindCollectionByNameOrId("sync_items")
		require.NoError(t, err)

		record := models.NewRecord(collection)
		record.Set("mapping_id", "invalid_mapping_id")
		record.Set("service", "youtube")
		record.Set("action", "rename_playlist")
		record.Set("payload", `{"new_name":"New Name"}`)
		record.Set("status", "running")
		record.Set("attempts", 0)
		record.Set("next_attempt_at", time.Now().Format("2006-01-02 15:04:05.000Z"))
		record.Set("attempt_backoff_secs", 30)

		err = testApp.Dao().SaveRecord(record)
		require.NoError(t, err)

		err = executeYouTubeRenamePlaylist(wrapper, record, `{"new_name":"New Name"}`)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to find mapping")
	})
}
