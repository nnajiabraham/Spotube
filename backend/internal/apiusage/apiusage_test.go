package apiusage

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/migrate"
	"github.com/manlikeabro/spotube/internal/sqliteconn"
)

func TestYouTubeCost(t *testing.T) {
	assert.Equal(t, 100, YouTubeCost(http.MethodGet, "/youtube/v3/search"))
	assert.Equal(t, 1, YouTubeCost(http.MethodGet, "/youtube/v3/videos"))
	assert.Equal(t, 1, YouTubeCost(http.MethodGet, "/youtube/v3/playlistItems"))
	assert.Equal(t, 50, YouTubeCost(http.MethodPost, "/youtube/v3/playlistItems"))
	assert.Equal(t, 50, YouTubeCost(http.MethodPut, "/youtube/v3/playlists"))
}

func TestDayUsesPacificTime(t *testing.T) {
	// 07:30 UTC is still the previous day in Los Angeles.
	assert.Equal(t, "2026-10-22", Day(time.Date(2026, 10, 23, 6, 30, 0, 0, time.UTC)))
	assert.Equal(t, "2026-10-23", Day(time.Date(2026, 10, 23, 8, 30, 0, 0, time.UTC)))
}

func TestTransportRecordsYouTubeCalls(t *testing.T) {
	db, err := sqliteconn.OpenWithPragmas(filepath.Join(t.TempDir(), "usage.db"))
	require.NoError(t, err)
	require.NoError(t, migrate.Up(db))
	SetDB(db)
	defer SetDB(nil)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	client := &http.Client{Transport: &Transport{Base: http.DefaultTransport}}

	for _, call := range []struct{ method, path string }{
		{http.MethodGet, "/youtube/v3/search"},
		{http.MethodGet, "/youtube/v3/videos"},
		{http.MethodPost, "/youtube/v3/playlistItems"},
		{http.MethodGet, "/other/api"},
	} {
		req, _ := http.NewRequest(call.method, srv.URL+call.path, nil)
		resp, err := client.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
	}

	used, err := Today(db, "youtube")
	require.NoError(t, err)
	assert.Equal(t, 151, used)
	assert.Equal(t, YouTubeDailyQuota-151, Remaining(db))
}
