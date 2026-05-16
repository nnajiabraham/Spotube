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

func TestSyncItemsGetNotFound(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()

	handler := NewSyncItemsHandler(db)
	e := echo.New()
	RegisterSyncItemsRoutes(e.Group("/api/collections/sync_items/records"), handler)

	req := httptest.NewRequest(http.MethodGet, "/api/collections/sync_items/records/nonexistent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSyncItemsGetExisting(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()

	now := time.Now().Unix()

	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated)
		VALUES ('m1', 'sp1', 'yt1', 1, 1, 60, 0, ?, ?)`, now, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO sync_items (id, mapping_id, operation, service, track_id, track_title, track_artist, status, attempt_count, created, updated)
		VALUES ('si1', 'm1', 'add', 'youtube', 'track123', 'Test Track', 'Test Artist', 'pending', 0, ?, ?)`, now, now)
	require.NoError(t, err)

	handler := NewSyncItemsHandler(db)
	e := echo.New()
	RegisterSyncItemsRoutes(e.Group("/api/collections/sync_items/records"), handler)

	req := httptest.NewRequest(http.MethodGet, "/api/collections/sync_items/records/si1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp SyncItemResponse
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "si1", resp.ID)
	assert.Equal(t, "m1", resp.MappingID)
	assert.Equal(t, "add", resp.Operation)
	assert.Equal(t, "youtube", resp.Service)
	assert.Equal(t, "Test Track", resp.TrackTitle)
	assert.Equal(t, "Test Artist", resp.TrackArtist)
	assert.Equal(t, "pending", resp.Status)
	assert.Equal(t, 0, resp.AttemptCount)
}
