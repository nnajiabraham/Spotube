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

func setupBlacklistTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "blacklist_test.db")

	db, err := sqliteconn.OpenWithPragmas(dbPath)
	require.NoError(t, err)

	require.NoError(t, migrate.Up(db))

	cleanup := func() {
		migrate.Down(db)
		db.Close()
	}
	return db, cleanup
}

func TestBlacklistCreate(t *testing.T) {
	db, cleanup := setupBlacklistTestDB(t)
	defer cleanup()

	// Create a mapping first (foreign key requirement)
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"mapping1", "spotify123", "youtube456", 1, 1, 60, 0, now, now)
	require.NoError(t, err)

	handler := NewBlacklistHandler(db)
	e := echo.New()
	RegisterBlacklistRoutes(e.Group("/api/collections/blacklist/records"), handler)

	payload := map[string]any{
		"mapping_id": "mapping1",
		"service":    "spotify",
		"track_id":   "track123",
		"reason":     "not found",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/collections/blacklist/records", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)

	var response BlacklistResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "mapping1", response.MappingID)
	assert.Equal(t, "spotify", response.Service)
	assert.Equal(t, "track123", response.TrackID)
	assert.Equal(t, "not found", response.Reason)
	assert.Equal(t, 0, response.SkipCounter)
	assert.NotEmpty(t, response.ID)
}

func TestBlacklistCreateValidationErrors(t *testing.T) {
	db, cleanup := setupBlacklistTestDB(t)
	defer cleanup()

	handler := NewBlacklistHandler(db)
	e := echo.New()
	RegisterBlacklistRoutes(e.Group("/api/collections/blacklist/records"), handler)

	testCases := []struct {
		name     string
		payload  map[string]any
		expected int
	}{
		{
			name: "missing mapping_id",
			payload: map[string]any{
				"service":  "spotify",
				"track_id": "track123",
			},
			expected: http.StatusUnprocessableEntity,
		},
		{
			name: "invalid service",
			payload: map[string]any{
				"mapping_id": "mapping1",
				"service":    "invalid",
				"track_id":   "track123",
			},
			expected: http.StatusUnprocessableEntity,
		},
		{
			name: "missing track_id",
			payload: map[string]any{
				"mapping_id": "mapping1",
				"service":    "spotify",
			},
			expected: http.StatusUnprocessableEntity,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.payload)
			req := httptest.NewRequest(http.MethodPost, "/api/collections/blacklist/records", bytes.NewReader(body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			assert.Equal(t, tc.expected, rec.Code)
		})
	}
}

func TestBlacklistList(t *testing.T) {
	db, cleanup := setupBlacklistTestDB(t)
	defer cleanup()

	// Create mappings first
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"mapping1", "spotify123", "youtube456", 1, 1, 60, 0, now, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"mapping2", "spotify789", "youtube101", 1, 1, 60, 0, now, now)
	require.NoError(t, err)

	// Seed blacklist entries
	_, err = db.Exec(`INSERT INTO blacklist (id, mapping_id, service, track_id, reason, skip_counter, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"blacklist1", "mapping1", "spotify", "track123", "not found", 0, now, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO blacklist (id, mapping_id, service, track_id, reason, skip_counter, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"blacklist2", "mapping2", "youtube", "track456", "regional restriction", 1, now+1, now+1)
	require.NoError(t, err)

	handler := NewBlacklistHandler(db)
	e := echo.New()
	RegisterBlacklistRoutes(e.Group("/api/collections/blacklist/records"), handler)

	// Test list all
	req := httptest.NewRequest(http.MethodGet, "/api/collections/blacklist/records?page=1&per_page=10", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response BlacklistListResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 1, response.Page)
	assert.Equal(t, 10, response.PerPage)
	assert.Equal(t, int64(2), response.TotalItems)
	assert.Len(t, response.Items, 2)

	// Test filtered by mapping_id
	req = httptest.NewRequest(http.MethodGet, "/api/collections/blacklist/records?mapping_id=mapping1", nil)
	rec = httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, int64(1), response.TotalItems)
	assert.Len(t, response.Items, 1)
	assert.Equal(t, "blacklist1", response.Items[0].ID)
	assert.Equal(t, "mapping1", response.Items[0].MappingID)
}

func TestBlacklistDelete(t *testing.T) {
	db, cleanup := setupBlacklistTestDB(t)
	defer cleanup()

	// Create mapping first
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"mapping1", "spotify123", "youtube456", 1, 1, 60, 0, now, now)
	require.NoError(t, err)

	// Seed blacklist entry
	_, err = db.Exec(`INSERT INTO blacklist (id, mapping_id, service, track_id, skip_counter, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"blacklist1", "mapping1", "spotify", "track123", 0, now, now)
	require.NoError(t, err)

	handler := NewBlacklistHandler(db)
	e := echo.New()
	RegisterBlacklistRoutes(e.Group("/api/collections/blacklist/records"), handler)

	req := httptest.NewRequest(http.MethodDelete, "/api/collections/blacklist/records/blacklist1", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)

	// Verify deletion
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM blacklist WHERE id = ?", "blacklist1").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestBlacklistDeleteNotFound(t *testing.T) {
	db, cleanup := setupBlacklistTestDB(t)
	defer cleanup()

	handler := NewBlacklistHandler(db)
	e := echo.New()
	RegisterBlacklistRoutes(e.Group("/api/collections/blacklist/records"), handler)

	req := httptest.NewRequest(http.MethodDelete, "/api/collections/blacklist/records/nonexistent", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestBlacklistCreateDuplicate(t *testing.T) {
	db, cleanup := setupBlacklistTestDB(t)
	defer cleanup()

	// Create mapping first
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"mapping1", "spotify123", "youtube456", 1, 1, 60, 0, now, now)
	require.NoError(t, err)

	// Seed existing blacklist entry
	_, err = db.Exec(`INSERT INTO blacklist (id, mapping_id, service, track_id, skip_counter, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"blacklist1", "mapping1", "spotify", "track123", 0, now, now)
	require.NoError(t, err)

	handler := NewBlacklistHandler(db)
	e := echo.New()
	RegisterBlacklistRoutes(e.Group("/api/collections/blacklist/records"), handler)

	// Try to create duplicate
	payload := map[string]any{
		"mapping_id": "mapping1",
		"service":    "spotify",
		"track_id":   "track123",
		"reason":     "duplicate test",
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/collections/blacklist/records", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}
