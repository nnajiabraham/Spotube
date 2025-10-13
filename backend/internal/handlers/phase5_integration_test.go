package handlers

import (
	"bytes"
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

func TestPhase5EndpointIntegration(t *testing.T) {
	db, cleanup := setupPhase5TestDB(t)
	defer cleanup()

	// Setup Echo server with all handlers (similar to main.go)
	e := echo.New()

	// Register blacklist routes
	blacklistHandler := NewBlacklistHandler(db)
	blacklistGroup := e.Group("/api/collections/blacklist/records")
	RegisterBlacklistRoutes(blacklistGroup, blacklistHandler)

	// Register activity logs routes
	activityLogsHandler := NewActivityLogsHandler(db)
	activityLogsGroup := e.Group("/api/collections/activity_logs/records")
	RegisterActivityLogsRoutes(activityLogsGroup, activityLogsHandler)

	// Create mapping (required for blacklist FK)
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"integration-mapping", "spotify-integration", "youtube-integration", 1, 1, 60, 0, now, now)
	require.NoError(t, err)

	// Test: Create blacklist entry via API
	blacklistPayload := map[string]any{
		"mapping_id": "integration-mapping",
		"service":    "spotify",
		"track_id":   "problem-track-123",
		"reason":     "integration test blacklist",
	}

	body, _ := json.Marshal(blacklistPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/collections/blacklist/records", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	// Test: List blacklist entries
	req = httptest.NewRequest(http.MethodGet, "/api/collections/blacklist/records?mapping_id=integration-mapping", nil)
	rec = httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var blacklistResponse BlacklistListResponse
	err = json.Unmarshal(rec.Body.Bytes(), &blacklistResponse)
	require.NoError(t, err)
	assert.Equal(t, int64(1), blacklistResponse.TotalItems)
	assert.Equal(t, "problem-track-123", blacklistResponse.Items[0].TrackID)

	// Test: Create activity log via helper
	activityLogger := activitylogger.New(db)
	err = activityLogger.RecordInfo("Integration test activity", "integration-mapping", "analysis")
	require.NoError(t, err)

	// Test: List activity logs via API
	req = httptest.NewRequest(http.MethodGet, "/api/collections/activity_logs/records?job_type=analysis", nil)
	rec = httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var activityResponse ActivityLogsListResponse
	err = json.Unmarshal(rec.Body.Bytes(), &activityResponse)
	require.NoError(t, err)
	assert.Equal(t, int64(1), activityResponse.TotalItems)
	assert.Equal(t, "Integration test activity", activityResponse.Items[0].Message)
	assert.Equal(t, "analysis", activityResponse.Items[0].JobType)

	// Test: Delete blacklist entry
	blacklistID := blacklistResponse.Items[0].ID
	req = httptest.NewRequest(http.MethodDelete, "/api/collections/blacklist/records/"+blacklistID, nil)
	rec = httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Verify blacklist is now empty
	req = httptest.NewRequest(http.MethodGet, "/api/collections/blacklist/records", nil)
	rec = httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	err = json.Unmarshal(rec.Body.Bytes(), &blacklistResponse)
	require.NoError(t, err)
	assert.Equal(t, int64(0), blacklistResponse.TotalItems)
}

func setupPhase5TestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "phase5_integration_test.db")

	db, err := sqliteconn.OpenWithPragmas(dbPath)
	require.NoError(t, err)

	require.NoError(t, migrate.Up(db))

	cleanup := func() {
		migrate.Down(db)
		db.Close()
	}
	return db, cleanup
}
