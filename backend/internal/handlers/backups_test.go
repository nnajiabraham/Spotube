package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/backups"
	"github.com/manlikeabro/spotube/internal/matching"
)

// memoryPlaylists is an in-memory Spotify/YouTube for backup handler tests.
type memoryPlaylists struct {
	tracks map[string][]matching.Track
	added  map[string][]string
}

func (m *memoryPlaylists) PlaylistTracks(_ context.Context, service, id string) (string, []matching.Track, error) {
	return "Road trip", m.tracks[service+"|"+id], nil
}

func (m *memoryPlaylists) CreatePlaylist(_ context.Context, _, _, _ string) (string, error) {
	return "restored-1", nil
}

func (m *memoryPlaylists) AddTracks(_ context.Context, service, id string, ids []string) (int, error) {
	m.added[service+"|"+id] = append(m.added[service+"|"+id], ids...)
	return len(ids), nil
}

func backupsServer(t *testing.T) (*echo.Echo, *memoryPlaylists) {
	t.Helper()
	db, cleanup := setupSyncItemsTestDB(t)
	t.Cleanup(cleanup)
	mem := &memoryPlaylists{tracks: map[string][]matching.Track{
		"spotify|pl1": {matching.SpotifyTrack("t1", "One", "A"), matching.SpotifyTrack("t2", "Two", "A")},
	}, added: map[string][]string{}}
	svc := &backups.Service{Store: &backups.Store{DB: db}, Reader: mem, Writer: mem}
	e := echo.New()
	RegisterBackupsRoutes(e.Group("/api/backups"), NewBackupsHandler(svc))
	return e, mem
}

func call(t *testing.T, e *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestBackupsAPI_CreateSnapshotExportRestore(t *testing.T) {
	e, mem := backupsServer(t)

	rec := call(t, e, http.MethodPost, "/api/backups", `{"service":"spotify","playlist_id":"pl1"}`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var b backups.Backup
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &b))
	assert.Equal(t, "Road trip", b.PlaylistName, "name comes from the first snapshot")
	assert.Equal(t, 1, b.SnapshotCount)
	require.NotNil(t, b.Latest)
	assert.Equal(t, 2, b.Latest.TrackCount)
	assert.Equal(t, backups.SourceInitial, b.Latest.Source)

	rec = call(t, e, http.MethodPost, "/api/backups", `{"service":"spotify","playlist_id":"pl1"}`)
	assert.Equal(t, http.StatusConflict, rec.Code)

	// Unchanged: no new snapshot.
	rec = call(t, e, http.MethodPost, "/api/backups/"+b.ID+"/run", ``)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"changed":false`)

	mem.tracks["spotify|pl1"] = mem.tracks["spotify|pl1"][:1]
	rec = call(t, e, http.MethodPost, "/api/backups/"+b.ID+"/run", ``)
	assert.Contains(t, rec.Body.String(), `"changed":true`)

	rec = call(t, e, http.MethodGet, "/api/backups/"+b.ID+"/snapshots", ``)
	require.Equal(t, http.StatusOK, rec.Code)
	var list struct {
		Items []backups.SnapshotSummary `json:"items"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Len(t, list.Items, 2)
	assert.Equal(t, 1, *list.Items[0].Removed)
	original := list.Items[1].ID

	rec = call(t, e, http.MethodGet, "/api/backups/snapshots/"+original+"/export?format=csv", ``)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Disposition"), `Road-trip-spotify-`)
	assert.Equal(t, 3, strings.Count(rec.Body.String(), "\n"))

	rec = call(t, e, http.MethodGet, "/api/backups/snapshots/"+original+"/export", ``)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"track_count": 2`)

	rec = call(t, e, http.MethodPost, "/api/backups/snapshots/"+original+"/restore", `{"mode":"missing"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []string{"t2"}, mem.added["spotify|pl1"])

	rec = call(t, e, http.MethodPost, "/api/backups/snapshots/"+original+"/restore", `{"mode":"wipe"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = call(t, e, http.MethodPatch, "/api/backups/"+b.ID, `{"enabled":false,"interval_hours":6}`)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"enabled":false`)
	assert.Contains(t, rec.Body.String(), `"interval_hours":6`)

	rec = call(t, e, http.MethodDelete, "/api/backups/"+b.ID, ``)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	rec = call(t, e, http.MethodGet, "/api/backups/snapshots/"+original, ``)
	assert.Equal(t, http.StatusNotFound, rec.Code, "snapshots go with their backup")
}
