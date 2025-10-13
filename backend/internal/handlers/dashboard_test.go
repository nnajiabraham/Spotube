package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/activitylogger"
	"github.com/manlikeabro/spotube/internal/migrate"
	"github.com/manlikeabro/spotube/internal/sqliteconn"
)

func setupDashboardTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "dashboard_test.db")

	db, err := sqliteconn.OpenWithPragmas(dbPath)
	require.NoError(t, err)

	require.NoError(t, migrate.Up(db))

	cleanup := func() {
		migrate.Down(db)
		db.Close()
	}
	return db, cleanup
}

func TestDashboardStatsEmptyDatabase(t *testing.T) {
	db, cleanup := setupDashboardTestDB(t)
	defer cleanup()

	handler := NewDashboardHandler(db)
	e := echo.New()
	RegisterDashboardRoutes(e.Group("/api/dashboard"), handler)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/stats", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response DashboardStatsResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	// Empty database should return zeros
	assert.Equal(t, int64(0), response.Mappings.Total)
	assert.Equal(t, int64(0), response.Queue.Pending)
	assert.Equal(t, int64(0), response.Queue.Running)
	assert.Equal(t, int64(0), response.Queue.Done)
	assert.Equal(t, int64(0), response.Queue.Error)
	assert.Equal(t, int64(0), response.Queue.Skipped)
	assert.Empty(t, response.RecentRuns)

	// YouTube quota should have placeholder values
	assert.Equal(t, int64(0), response.YouTubeQuota.Used)
	assert.Equal(t, int64(10000), response.YouTubeQuota.Limit)
}

func TestDashboardStatsWithData(t *testing.T) {
	db, cleanup := setupDashboardTestDB(t)
	defer cleanup()

	now := time.Now().Unix()

	// Seed mappings
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"mapping1", "spotify1", "youtube1", 1, 1, 60, 0, now, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"mapping2", "spotify2", "youtube2", 1, 1, 60, 0, now, now)
	require.NoError(t, err)

	// Seed sync_items with various statuses
	_, err = db.Exec(`INSERT INTO sync_items (id, mapping_id, operation, service, status, track_id, attempt_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"sync1", "mapping1", "add", "spotify", "pending", "track1", 0, now, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO sync_items (id, mapping_id, operation, service, status, track_id, attempt_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"sync2", "mapping1", "add", "youtube", "running", "track2", 1, now, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO sync_items (id, mapping_id, operation, service, status, track_id, attempt_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"sync3", "mapping2", "add", "spotify", "done", "track3", 2, now, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO sync_items (id, mapping_id, operation, service, status, track_id, attempt_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"sync4", "mapping2", "add", "youtube", "error", "track4", 3, now, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO sync_items (id, mapping_id, operation, service, status, track_id, attempt_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"sync5", "mapping1", "add", "spotify", "skipped", "track5", 0, now, now)
	require.NoError(t, err)

	// Seed activity logs using the activity logger
	logger := activitylogger.New(db)
	err = logger.RecordInfo("Analysis completed for mapping1", "mapping1", "analysis")
	require.NoError(t, err)

	err = logger.RecordWarn("Rate limit encountered", "mapping2", "executor")
	require.NoError(t, err)

	err = logger.RecordError("Failed to sync track", "mapping1", "executor")
	require.NoError(t, err)

	handler := NewDashboardHandler(db)
	e := echo.New()
	RegisterDashboardRoutes(e.Group("/api/dashboard"), handler)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/stats", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response DashboardStatsResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify counts
	assert.Equal(t, int64(2), response.Mappings.Total)
	assert.Equal(t, int64(1), response.Queue.Pending)
	assert.Equal(t, int64(1), response.Queue.Running)
	assert.Equal(t, int64(1), response.Queue.Done)
	assert.Equal(t, int64(1), response.Queue.Error)
	assert.Equal(t, int64(1), response.Queue.Skipped)

	// Verify recent runs (should be ordered by timestamp desc)
	assert.Equal(t, 3, len(response.RecentRuns))
	assert.Equal(t, "error", response.RecentRuns[0].Level)
	assert.Equal(t, "Failed to sync track", response.RecentRuns[0].Message)
	assert.Equal(t, "executor", response.RecentRuns[0].JobType)
	assert.NotNil(t, response.RecentRuns[0].MappingID)
	assert.Equal(t, "mapping1", *response.RecentRuns[0].MappingID)

	// YouTube quota placeholder
	assert.Equal(t, int64(0), response.YouTubeQuota.Used)
	assert.Equal(t, int64(10000), response.YouTubeQuota.Limit)
}

func TestDashboardStatsResponseShape(t *testing.T) {
	db, cleanup := setupDashboardTestDB(t)
	defer cleanup()

	// Seed minimal data to test JSON structure
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"test-mapping", "spotify-test", "youtube-test", 1, 1, 60, 0, now, now)
	require.NoError(t, err)

	handler := NewDashboardHandler(db)
	e := echo.New()
	RegisterDashboardRoutes(e.Group("/api/dashboard"), handler)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/stats", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Verify JSON structure matches expected shape
	var responseMap map[string]any
	err = json.Unmarshal(rec.Body.Bytes(), &responseMap)
	require.NoError(t, err)

	// Check top-level keys
	assert.Contains(t, responseMap, "mappings")
	assert.Contains(t, responseMap, "queue")
	assert.Contains(t, responseMap, "recent_runs")
	assert.Contains(t, responseMap, "youtube_quota")

	// Check mappings structure
	mappings := responseMap["mappings"].(map[string]any)
	assert.Contains(t, mappings, "total")

	// Check queue structure
	queue := responseMap["queue"].(map[string]any)
	assert.Contains(t, queue, "pending")
	assert.Contains(t, queue, "running")
	assert.Contains(t, queue, "done")
	assert.Contains(t, queue, "error")
	assert.Contains(t, queue, "skipped")

	// Check youtube_quota structure
	youtubeQuota := responseMap["youtube_quota"].(map[string]any)
	assert.Contains(t, youtubeQuota, "used")
	assert.Contains(t, youtubeQuota, "limit")

	// Check recent_runs is array
	recentRuns := responseMap["recent_runs"].([]any)
	assert.IsType(t, []any{}, recentRuns)
}

func TestDashboardStatsRecentRunsLimit(t *testing.T) {
	db, cleanup := setupDashboardTestDB(t)
	defer cleanup()

	logger := activitylogger.New(db)

	// Create more than 10 activity logs to test limit
	for i := 0; i < 15; i++ {
		err := logger.RecordInfo("Test message "+string(rune(i+'A')), "mapping1", "analysis")
		require.NoError(t, err)
	}

	handler := NewDashboardHandler(db)
	e := echo.New()
	RegisterDashboardRoutes(e.Group("/api/dashboard"), handler)

	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/stats", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response DashboardStatsResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	// Should be limited to 10 recent runs
	assert.Len(t, response.RecentRuns, 10)
}

func TestDashboardStatsUnauthenticated(t *testing.T) {
	db, cleanup := setupDashboardTestDB(t)
	defer cleanup()

	handler := NewDashboardHandler(db)
	e := echo.New()
	RegisterDashboardRoutes(e.Group("/api/dashboard"), handler)

	// No authentication headers or cookies needed
	req := httptest.NewRequest(http.MethodGet, "/api/dashboard/stats", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// Should work without authentication
	assert.Equal(t, http.StatusOK, rec.Code)

	var response DashboardStatsResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	// Basic structure verification
	assert.GreaterOrEqual(t, response.Mappings.Total, int64(0))
	assert.NotNil(t, response.Queue)
	assert.NotNil(t, response.RecentRuns)
	assert.Equal(t, int64(10000), response.YouTubeQuota.Limit)
}


