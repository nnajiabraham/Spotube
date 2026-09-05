package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/jobs"
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

func insertSyncItemFixture(t *testing.T, db *sql.DB, itemID, mappingID, operation, service, status string) {
	t.Helper()
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, spotify_playlist_name, youtube_playlist_name, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		mappingID, "sp-1", "yt-1", "Spotify Hits", "YouTube Mix", 1, 1, 60, 0, now, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO sync_items (id, mapping_id, operation, service, track_id, track_title, track_artist, status, error_message, attempt_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		itemID, mappingID, operation, service, "track-1", "Song A", "Artist A", status, "", 2, now, now)
	require.NoError(t, err)
}

type stubSyncItemExecutor struct {
	item  model.SyncItems
	err   error
	calls int
}

func (s *stubSyncItemExecutor) ExecuteOne(_ context.Context, _ string) (model.SyncItems, error) {
	s.calls++
	return s.item, s.err
}

func TestSyncItemsGet(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()

	insertSyncItemFixture(t, db, "sync-1", "mapping-1", "add", "spotify", "pending")

	handler := NewSyncItemsHandler(db, nil)
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
	assert.Equal(t, "youtube", response.SourceService)
	assert.Equal(t, "spotify", response.DestinationService)
	assert.Equal(t, "YouTube Mix", response.SourcePlaylistName)
	assert.Equal(t, "Spotify Hits", response.DestinationPlaylistName)
	assert.Equal(t, "pending", response.Status)
	assert.Equal(t, 2, response.AttemptCount)
}

func TestSyncItemsGetNotFound(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()

	handler := NewSyncItemsHandler(db, nil)
	e := echo.New()
	RegisterSyncItemsRoutes(e.Group("/api/collections/sync_items/records"), handler)

	req := httptest.NewRequest(http.MethodGet, "/api/collections/sync_items/records/missing", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestSyncItemsListPaginationAndFilters(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()

	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, spotify_playlist_name, youtube_playlist_name, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"mapping-1", "sp-1", "yt-1", "Spotify Hits", "YouTube Mix", 1, 1, 60, 0, now, now)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"mapping-2", "sp-2", "yt-2", 1, 1, 60, 0, now, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO sync_items (id, mapping_id, operation, service, track_id, track_title, track_artist, status, error_message, attempt_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"sync-a", "mapping-1", "add", "youtube", "track-1", "Song A", "Artist A", "pending", "", 0, now, now)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO sync_items (id, mapping_id, operation, service, track_id, track_title, track_artist, status, error_message, attempt_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"sync-b", "mapping-1", "rename", "youtube", "track-1", "Title", "", "done", "", 0, now, now)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO sync_items (id, mapping_id, operation, service, track_id, status, attempt_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		"sync-c", "mapping-2", "add", "spotify", "t2", "error", now, now)
	require.NoError(t, err)

	handler := NewSyncItemsHandler(db, nil)
	e := echo.New()
	RegisterSyncItemsRoutes(e.Group("/api/collections/sync_items/records"), handler)

	req := httptest.NewRequest(http.MethodGet, "/api/collections/sync_items/records?status=error&service=spotify&mapping_id=mapping-2&per_page=10", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var response SyncItemsListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Len(t, response.Items, 1)
	assert.Equal(t, "sync-c", response.Items[0].ID)
	assert.Equal(t, "error", response.Items[0].Status)
	assert.Equal(t, int64(1), response.TotalItems)
}

func TestSyncItemsExecuteSuccess(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()

	insertSyncItemFixture(t, db, "sync-exec", "mapping-1", "add", "youtube", "done")

	executor := &stubSyncItemExecutor{
		item: model.SyncItems{
			ID:           stringPtr("sync-exec"),
			MappingID:    "mapping-1",
			Operation:    "add",
			Service:      "youtube",
			Status:       "done",
			AttemptCount: 3,
		},
	}

	handler := NewSyncItemsHandler(db, executor)
	e := echo.New()
	RegisterSyncItemsRoutes(e.Group("/api/collections/sync_items/records"), handler)

	req := httptest.NewRequest(http.MethodPost, "/api/collections/sync_items/records/sync-exec/execute", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, executor.calls)

	var response SyncItemExecuteResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	assert.Equal(t, "done", response.Status)
}

func TestSyncItemsExecuteNotExecutable(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()

	insertSyncItemFixture(t, db, "sync-done", "mapping-1", "add", "youtube", "done")

	executor := &stubSyncItemExecutor{err: jobs.ErrSyncItemNotExecutable}
	handler := NewSyncItemsHandler(db, executor)
	e := echo.New()
	RegisterSyncItemsRoutes(e.Group("/api/collections/sync_items/records"), handler)

	req := httptest.NewRequest(http.MethodPost, "/api/collections/sync_items/records/sync-done/execute", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestSyncItemsExecuteBlacklistedConflictBody(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()

	insertSyncItemFixture(t, db, "sync-bl", "mapping-1", "add", "youtube", "skipped")

	executor := &stubSyncItemExecutor{
		err: jobs.ErrSyncItemBlacklisted,
		item: model.SyncItems{
			ID:        stringPtr("sync-bl"),
			MappingID: "mapping-1",
			Operation: "add",
			Service:   "youtube",
			Status:    "skipped",
		},
	}

	handler := NewSyncItemsHandler(db, executor)
	e := echo.New()
	RegisterSyncItemsRoutes(e.Group("/api/collections/sync_items/records"), handler)

	req := httptest.NewRequest(http.MethodPost, "/api/collections/sync_items/records/sync-bl/execute", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code)

	var response SyncItemExecuteResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	assert.Equal(t, "skipped", response.Status)
}

func TestSyncItemsExecuteWithErrorReturnsBody(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()

	insertSyncItemFixture(t, db, "sync-err", "mapping-1", "add", "youtube", "error")

	now := int32(time.Now().Unix())
	errMsg := "no match on youtube"
	execErr := errors.New("no match on youtube")
	executor := &stubSyncItemExecutor{
		err: execErr,
		item: model.SyncItems{
			ID:           stringPtr("sync-err"),
			MappingID:    "mapping-1",
			Operation:    "add",
			Service:      "youtube",
			Status:       "error",
			ErrorMessage: &errMsg,
			AttemptCount: 1,
			Updated:      now,
		},
	}

	handler := NewSyncItemsHandler(db, executor)
	e := echo.New()
	RegisterSyncItemsRoutes(e.Group("/api/collections/sync_items/records"), handler)

	req := httptest.NewRequest(http.MethodPost, "/api/collections/sync_items/records/sync-err/execute", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var response SyncItemExecuteResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	assert.Equal(t, "error", response.Status)
	assert.NotEmpty(t, response.ExecutionLog)
}

func TestDeriveSourceDestination(t *testing.T) {
	source, dest := deriveSourceDestination("youtube")
	assert.Equal(t, "spotify", source)
	assert.Equal(t, "youtube", dest)

	source, dest = deriveSourceDestination("spotify")
	assert.Equal(t, "youtube", source)
	assert.Equal(t, "spotify", dest)
}
