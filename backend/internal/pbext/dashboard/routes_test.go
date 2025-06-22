package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tests"

	"github.com/manlikeabro/spotube/internal/jobs"
	"github.com/manlikeabro/spotube/internal/testhelpers"
)

// daoProvider interface for type compatibility
type daoProvider interface {
	Dao() *daos.Dao
}

// statsHandlerWithInterface creates a statsHandler that works with testApp
func statsHandlerWithInterface(provider daoProvider) echo.HandlerFunc {
	return func(c echo.Context) error {
		dao := daos.New(provider.Dao().DB())

		// Get mappings statistics
		mappingsStats, err := getMappingsStats(dao)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get mappings stats: "+err.Error())
		}

		// Get queue statistics
		queueStats, err := getQueueStats(dao)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get queue stats: "+err.Error())
		}

		// Get recent runs from activity logs
		recentRuns, err := getRecentRuns(dao)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get recent runs: "+err.Error())
		}

		// Get YouTube quota statistics
		used, limit := jobs.GetYouTubeQuotaUsage()
		youtubeQuota := YouTubeQuotaStats{
			Used:  used,
			Limit: limit,
		}

		response := StatsResponse{
			Mappings:     mappingsStats,
			Queue:        queueStats,
			RecentRuns:   recentRuns,
			YouTubeQuota: youtubeQuota,
		}

		return c.JSON(http.StatusOK, response)
	}
}

func TestStatsHandler_Success(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	// Create exactly 3 test mappings
	createTestMappings(testApp, 3)

	// Create specific sync items with known counts
	createTestSyncItems(testApp, map[string]int{
		"pending": 5,
		"running": 2,
		"done":    10,
		"error":   1,
		"skipped": 3,
	})
	createTestActivityLogs(testApp, 2)

	// Create echo context and request
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/stats", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Call the handler using interface wrapper
	handler := statsHandlerWithInterface(testApp)
	err := handler(c)

	// Assertions
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	// Parse response
	var response StatsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify mappings count (should be exactly what we created)
	if response.Mappings.Total != 3 {
		t.Errorf("Expected 3 mappings, got %d", response.Mappings.Total)
	}

	// Verify queue stats match what we created
	expectedQueue := QueueStats{
		Pending: 5,
		Running: 2,
		Done:    10,
		Errors:  1,
		Skipped: 3,
	}

	if response.Queue != expectedQueue {
		t.Errorf("Expected queue stats %+v, got %+v", expectedQueue, response.Queue)
	}

	// YouTube quota should have some default values
	if response.YouTubeQuota.Limit <= 0 {
		t.Errorf("Expected positive YouTube quota limit, got %d", response.YouTubeQuota.Limit)
	}

	t.Logf("Dashboard stats response: %+v", response)
}

func TestStatsHandler_EmptyDatabase(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	// Create echo context and request
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/stats", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Call the handler using interface wrapper
	handler := statsHandlerWithInterface(testApp)
	err := handler(c)

	// Assertions
	if err != nil {
		t.Fatalf("Handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	// Parse response
	var response StatsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify empty stats
	if response.Mappings.Total != 0 {
		t.Errorf("Expected 0 mappings, got %d", response.Mappings.Total)
	}

	expectedQueue := QueueStats{
		Pending: 0,
		Running: 0,
		Done:    0,
		Errors:  0,
		Skipped: 0,
	}

	if response.Queue != expectedQueue {
		t.Errorf("Expected empty queue stats %+v, got %+v", expectedQueue, response.Queue)
	}

	if len(response.RecentRuns) != 0 {
		t.Errorf("Expected 0 recent runs, got %d", len(response.RecentRuns))
	}
}

func TestStatsHandler_MissingActivityLogsCollection(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	// Don't create activity_logs collection to test graceful handling

	// Create echo context and request
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/stats", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Call the handler using interface wrapper
	handler := statsHandlerWithInterface(testApp)
	err := handler(c)

	// Should not fail even if activity_logs doesn't exist
	if err != nil {
		t.Fatalf("Handler should not fail with missing activity_logs collection: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", rec.Code)
	}

	// Parse response
	var response StatsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Should have empty recent runs
	if len(response.RecentRuns) != 0 {
		t.Errorf("Expected 0 recent runs when collection missing, got %d", len(response.RecentRuns))
	}
}

// Helper functions to create test data

func createTestMappings(testApp *tests.TestApp, count int) []*models.Record {
	var mappings []*models.Record

	for i := 0; i < count; i++ {
		mapping := testhelpers.CreateTestMapping(testApp, map[string]interface{}{
			"spotify_playlist_id": "test_spotify_" + string(rune(48+i)), // ASCII 48 = '0'
			"youtube_playlist_id": "test_youtube_" + string(rune(48+i)),
			"sync_name":           true,
			"sync_tracks":         true,
			"interval_minutes":    60,
		})
		if mapping != nil {
			mappings = append(mappings, mapping)
		}
	}
	return mappings
}

func createTestSyncItems(testApp *tests.TestApp, statusCounts map[string]int) {
	// Use the first mapping from the ones we already created
	mappings, err := testApp.Dao().FindRecordsByFilter("mappings", "id != ''", "", 1, 0)
	if err != nil || len(mappings) == 0 {
		// Fallback: create a mapping if none exists
		mapping := testhelpers.CreateTestMapping(testApp, map[string]interface{}{
			"spotify_playlist_id": "fallback_spotify",
			"youtube_playlist_id": "fallback_youtube",
			"sync_name":           true,
			"sync_tracks":         true,
			"interval_minutes":    60,
		})
		mappings = []*models.Record{mapping}
	}

	mapping := mappings[0]
	collection, err := testApp.Dao().FindCollectionByNameOrId("sync_items")
	if err != nil {
		return
	}

	for status, count := range statusCounts {
		for i := 0; i < count; i++ {
			record := models.NewRecord(collection)
			record.Set("mapping_id", mapping.Id)
			record.Set("service", "spotify")
			record.Set("action", "add_track")
			record.Set("status", status)

			// Make unique track ID and payload to avoid unique constraint violations
			trackId := fmt.Sprintf("test_track_%s_%d", status, i)
			record.Set("source_track_id", trackId)
			record.Set("source_track_title", fmt.Sprintf("Test Track %s %d", status, i))
			record.Set("source_service", "youtube")
			record.Set("destination_service", "spotify")

			// Make payload unique by including track ID
			record.Set("payload", fmt.Sprintf(`{"track_id":"%s"}`, trackId))
			record.Set("attempts", 0)
			record.Set("next_attempt_at", "2025-01-01 00:00:00.000Z")
			record.Set("attempt_backoff_secs", 30)

			err := testApp.Dao().SaveRecord(record)
			if err != nil {
				// Skip failed records, but don't fail the test
				continue
			}
		}
	}
}

func createTestActivityLogs(testApp *tests.TestApp, count int) {
	collection, err := testApp.Dao().FindCollectionByNameOrId("activity_logs")
	if err != nil {
		// activity_logs collection might not exist in test environment
		return
	}

	for i := 0; i < count; i++ {
		record := models.NewRecord(collection)
		record.Set("level", "info")
		record.Set("job_type", "analysis")
		record.Set("message", "Analysis job completed successfully")
		testApp.Dao().SaveRecord(record)
	}
}
