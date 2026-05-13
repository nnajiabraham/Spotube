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

	"github.com/manlikeabro/spotube/internal/migrate"
	"github.com/manlikeabro/spotube/internal/sqliteconn"
)

func setupSyncItemsTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "sync_items_test.db")

	db, err := sqliteconn.OpenWithPragmas(dbPath)
	require.NoError(t, err)
	require.NoError(t, migrate.Up(db))

	cleanup := func() {
		migrate.Down(db)
		db.Close()
	}

	return db, cleanup
}

func TestSyncItemsGet(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()

	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"mapping-1", "sp-1", "yt-1", 1, 1, 60, 0, now, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO sync_items (id, mapping_id, operation, service, track_id, track_title, track_artist, status, error_message, attempt_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"sync-1", "mapping-1", "add", "spotify", "track-1", "Song A", "Artist A", "pending", "", 2, now, now)
	require.NoError(t, err)

	handler := NewSyncItemsHandler(db)
	e := echo.New()
	RegisterSyncItemsRoutes(e.Group("/api/collections/sync_items/records"), handler)

	req := httptest.NewRequest(http.MethodGet, "/api/collections/sync_items/records/sync-1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var response SyncItemResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	assert.Equal(t, "sync-1", response.ID)
	assert.Equal(t, "mapping-1", response.MappingID)
	assert.Equal(t, "add", response.Operation)
	assert.Equal(t, "spotify", response.Service)
	assert.Equal(t, "track-1", response.TrackID)
	assert.Equal(t, "Song A", response.TrackTitle)
	assert.Equal(t, "pending", response.Status)
	assert.Equal(t, 2, response.AttemptCount)
}

func TestSyncItemsGetNotFound(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()

	handler := NewSyncItemsHandler(db)
	e := echo.New()
	RegisterSyncItemsRoutes(e.Group("/api/collections/sync_items/records"), handler)

	req := httptest.NewRequest(http.MethodGet, "/api/collections/sync_items/records/missing", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
