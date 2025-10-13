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

	"github.com/manlikeabro/spotube/internal/migrate"
	"github.com/manlikeabro/spotube/internal/sqliteconn"
)

func setupMappingsTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "mappings_test.db")

	db, err := sqliteconn.OpenWithPragmas(dbPath)
	require.NoError(t, err)

	require.NoError(t, migrate.Up(db))

	cleanup := func() {
		migrate.Down(db)
		db.Close()
	}
	return db, cleanup
}

func TestMappingsCreate(t *testing.T) {
	db, cleanup := setupMappingsTestDB(t)
	defer cleanup()

	handler := NewMappingsHandler(db)
	e := echo.New()
	RegisterMappingsRoutes(e.Group("/api/collections/mappings/records"), handler)

	payload := map[string]any{
		"spotify_playlist_id":   "spotify123",
		"youtube_playlist_id":   "youtube456",
		"spotify_playlist_name": "My Spotify Playlist",
		"youtube_playlist_name": "My YouTube Playlist",
		"sync_name":             true,
		"sync_tracks":           true,
		"interval_minutes":      120,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/collections/mappings/records", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var response MappingResponse
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "spotify123", response.SpotifyPlaylistID)
	assert.Equal(t, "youtube456", response.YoutubePlaylistID)
	assert.Equal(t, "My Spotify Playlist", response.SpotifyPlaylistName)
	assert.Equal(t, true, response.SyncName)
	assert.Equal(t, 120, response.IntervalMinutes)
	assert.NotEmpty(t, response.ID)
}

func TestMappingsCreateValidationErrors(t *testing.T) {
	db, cleanup := setupMappingsTestDB(t)
	defer cleanup()

	handler := NewMappingsHandler(db)
	e := echo.New()
	RegisterMappingsRoutes(e.Group("/api/collections/mappings/records"), handler)

	// Missing required fields
	payload := map[string]any{
		"spotify_playlist_id": "spotify123",
		// missing youtube_playlist_id
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/collections/mappings/records", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestMappingsList(t *testing.T) {
	db, cleanup := setupMappingsTestDB(t)
	defer cleanup()

	// Seed test data
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"mapping1", "spotify1", "youtube1", 1, 1, 60, 0, now, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"mapping2", "spotify2", "youtube2", 1, 1, 60, 0, now+1, now+1)
	require.NoError(t, err)

	handler := NewMappingsHandler(db)
	e := echo.New()
	RegisterMappingsRoutes(e.Group("/api/collections/mappings/records"), handler)

	req := httptest.NewRequest(http.MethodGet, "/api/collections/mappings/records?page=1&per_page=10&sort=created&order=desc", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response MappingsListResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 1, response.Page)
	assert.Equal(t, 10, response.PerPage)
	assert.Equal(t, int64(2), response.TotalItems)
	assert.Equal(t, 1, response.TotalPages)
	assert.Len(t, response.Items, 2)

	// Should be ordered by created desc
	assert.Equal(t, "mapping2", response.Items[0].ID)
	assert.Equal(t, "mapping1", response.Items[1].ID)
}

func TestMappingsGet(t *testing.T) {
	db, cleanup := setupMappingsTestDB(t)
	defer cleanup()

	// Seed test data
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, spotify_playlist_name, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"test-mapping", "spotify123", "youtube456", "Test Playlist", 1, 1, 60, 0, now, now)
	require.NoError(t, err)

	handler := NewMappingsHandler(db)
	e := echo.New()
	RegisterMappingsRoutes(e.Group("/api/collections/mappings/records"), handler)

	req := httptest.NewRequest(http.MethodGet, "/api/collections/mappings/records/test-mapping", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response MappingResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "test-mapping", response.ID)
	assert.Equal(t, "spotify123", response.SpotifyPlaylistID)
	assert.Equal(t, "Test Playlist", response.SpotifyPlaylistName)
}

func TestMappingsGetNotFound(t *testing.T) {
	db, cleanup := setupMappingsTestDB(t)
	defer cleanup()

	handler := NewMappingsHandler(db)
	e := echo.New()
	RegisterMappingsRoutes(e.Group("/api/collections/mappings/records"), handler)

	req := httptest.NewRequest(http.MethodGet, "/api/collections/mappings/records/nonexistent", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestMappingsUpdate(t *testing.T) {
	db, cleanup := setupMappingsTestDB(t)
	defer cleanup()

	// Seed test data
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, spotify_playlist_name, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"test-mapping", "spotify123", "youtube456", "Original Name", 1, 1, 60, 0, now, now)
	require.NoError(t, err)

	handler := NewMappingsHandler(db)
	e := echo.New()
	RegisterMappingsRoutes(e.Group("/api/collections/mappings/records"), handler)

	payload := map[string]any{
		"spotify_playlist_name": "Updated Name",
		"sync_name":             false,
		"interval_minutes":      30,
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPatch, "/api/collections/mappings/records/test-mapping", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response MappingResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "test-mapping", response.ID)
	assert.Equal(t, "Updated Name", response.SpotifyPlaylistName)
	assert.Equal(t, false, response.SyncName)
	assert.Equal(t, 30, response.IntervalMinutes)
}

func TestMappingsDelete(t *testing.T) {
	db, cleanup := setupMappingsTestDB(t)
	defer cleanup()

	// Seed test data
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"test-mapping", "spotify123", "youtube456", 1, 1, 60, 0, now, now)
	require.NoError(t, err)

	handler := NewMappingsHandler(db)
	e := echo.New()
	RegisterMappingsRoutes(e.Group("/api/collections/mappings/records"), handler)

	req := httptest.NewRequest(http.MethodDelete, "/api/collections/mappings/records/test-mapping", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Verify deletion
	req = httptest.NewRequest(http.MethodGet, "/api/collections/mappings/records/test-mapping", nil)
	rec = httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
