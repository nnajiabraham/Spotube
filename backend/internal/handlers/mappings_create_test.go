package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCreator struct {
	calls []string // "service:name"
}

func (f *fakeCreator) CreatePlaylist(_ context.Context, service, name, _ string) (string, error) {
	f.calls = append(f.calls, service+":"+name)
	return "new-" + service, nil
}

func postMapping(t *testing.T, e *echo.Echo, payload map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/collections/mappings/records", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestMappingsCreate_CreatesMissingPlaylist(t *testing.T) {
	db, cleanup := setupMappingsTestDB(t)
	defer cleanup()

	creator := &fakeCreator{}
	quota := 10000
	handler := NewMappingsHandler(db)
	handler.Creator = creator
	handler.QuotaLeft = func() int { return quota }
	e := echo.New()
	RegisterMappingsRoutes(e.Group("/api/collections/mappings/records"), handler)

	// New YouTube playlist named after the Spotify one.
	rec := postMapping(t, e, map[string]any{
		"spotify_playlist_id":   "sp1",
		"spotify_playlist_name": "Road Trip",
		"create_playlist":       "youtube",
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var got MappingResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "new-youtube", got.YoutubePlaylistID)
	assert.Equal(t, "Road Trip", got.YoutubePlaylistName)

	// New Spotify playlist with an explicit name.
	rec = postMapping(t, e, map[string]any{
		"youtube_playlist_id":   "yt2",
		"youtube_playlist_name": "Worship",
		"spotify_playlist_name": "Worship (from YouTube)",
		"create_playlist":       "spotify",
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "new-spotify", got.SpotifyPlaylistID)
	assert.Equal(t, []string{"youtube:Road Trip", "spotify:Worship (from YouTube)"}, creator.calls)

	// Not enough YouTube quota: refused before anything is created.
	quota = 10
	rec = postMapping(t, e, map[string]any{"spotify_playlist_id": "sp3", "spotify_playlist_name": "X", "create_playlist": "youtube"})
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
	assert.Len(t, creator.calls, 2)
}

func TestMappingsCreate_Validation(t *testing.T) {
	db, cleanup := setupMappingsTestDB(t)
	defer cleanup()

	handler := NewMappingsHandler(db)
	handler.Creator = &fakeCreator{}
	e := echo.New()
	RegisterMappingsRoutes(e.Group("/api/collections/mappings/records"), handler)

	cases := map[string]map[string]any{
		"unknown service":       {"spotify_playlist_id": "sp", "create_playlist": "tidal"},
		"id given for new side": {"spotify_playlist_id": "sp", "youtube_playlist_id": "yt", "create_playlist": "youtube"},
		"other side missing":    {"create_playlist": "youtube", "youtube_playlist_name": "x"},
		"no name for new side":  {"spotify_playlist_id": "sp", "create_playlist": "youtube"},
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, http.StatusUnprocessableEntity, postMapping(t, e, payload).Code)
		})
	}
}

func TestMappingsCreate_RejectsAlreadyPairedPlaylist(t *testing.T) {
	db, cleanup := setupMappingsTestDB(t)
	defer cleanup()

	handler := NewMappingsHandler(db)
	e := echo.New()
	RegisterMappingsRoutes(e.Group("/api/collections/mappings/records"), handler)

	require.Equal(t, http.StatusCreated, postMapping(t, e, map[string]any{"spotify_playlist_id": "sp", "youtube_playlist_id": "yt"}).Code)
	assert.Equal(t, http.StatusConflict, postMapping(t, e, map[string]any{"spotify_playlist_id": "sp", "youtube_playlist_id": "yt-other"}).Code)
	assert.Equal(t, http.StatusConflict, postMapping(t, e, map[string]any{"spotify_playlist_id": "sp-other", "youtube_playlist_id": "yt"}).Code)
}

func TestMappingsCreate_AllowsDailyInterval(t *testing.T) {
	db, cleanup := setupMappingsTestDB(t)
	defer cleanup()

	e := echo.New()
	RegisterMappingsRoutes(e.Group("/api/collections/mappings/records"), NewMappingsHandler(db))

	assert.Equal(t, http.StatusCreated, postMapping(t, e, map[string]any{"spotify_playlist_id": "sp", "youtube_playlist_id": "yt", "interval_minutes": 1440}).Code)
	assert.Equal(t, http.StatusUnprocessableEntity, postMapping(t, e, map[string]any{"spotify_playlist_id": "sp2", "youtube_playlist_id": "yt2", "interval_minutes": 2000}).Code)
}
