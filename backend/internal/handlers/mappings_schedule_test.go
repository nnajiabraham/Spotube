package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/matching"
	"github.com/manlikeabro/spotube/internal/quota"
	"github.com/manlikeabro/spotube/internal/schedule"
	"github.com/manlikeabro/spotube/internal/settings"
	"github.com/manlikeabro/spotube/internal/tracklinks"
)

type fakeSyncer struct{ started []string }

func (f *fakeSyncer) SyncMapping(context.Context, model.Mappings) error { return nil }
func (f *fakeSyncer) StartSync(m model.Mappings) bool {
	f.started = append(f.started, *m.ID)
	return true
}
func (f *fakeSyncer) IsSyncing(id string) bool {
	for _, s := range f.started {
		if s == id {
			return true
		}
	}
	return false
}

func send(t *testing.T, e *echo.Echo, method, url string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	if payload != nil {
		require.NoError(t, json.NewEncoder(&body).Encode(payload))
	}
	req := httptest.NewRequest(method, url, &body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

const mappingsURL = "/api/collections/mappings/records"

func TestMappingsCreate_ScheduleAndSyncNow(t *testing.T) {
	db, cleanup := setupMappingsTestDB(t)
	defer cleanup()
	syncer := &fakeSyncer{}
	now := time.Date(2026, 10, 23, 10, 0, 0, 0, time.UTC)
	h := NewMappingsHandler(db)
	h.Syncer = syncer
	h.Now = func() time.Time { return now }
	syncDefault := true
	h.SyncNowDefault = func() bool { return syncDefault }
	e := echo.New()
	RegisterMappingsRoutes(e.Group(mappingsURL), h)

	rec := send(t, e, http.MethodPost, mappingsURL, map[string]any{
		"spotify_playlist_id": "sp1", "youtube_playlist_id": "yt1",
		"interval_minutes": 1440, "schedule_start": "03:00", "schedule_days": schedule.Weekdays, "schedule_tz": "UTC",
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var got MappingResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "03:00", got.ScheduleStart)
	assert.Equal(t, schedule.Weekdays, got.ScheduleDays)
	require.NotNil(t, got.NextRunAt)
	assert.Equal(t, time.Date(2026, 10, 26, 3, 0, 0, 0, time.UTC).Unix(), *got.NextRunAt, "Friday 10:00 -> Monday 03:00")
	assert.Equal(t, []string{got.ID}, syncer.started, "the default starts the first sync")
	assert.True(t, got.Syncing)

	// sync_now: false overrides the default.
	rec = send(t, e, http.MethodPost, mappingsURL, map[string]any{"spotify_playlist_id": "sp2", "youtube_playlist_id": "yt2", "sync_now": false})
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Len(t, syncer.started, 1)
	// ... and the setting can turn it off.
	syncDefault = false
	rec = send(t, e, http.MethodPost, mappingsURL, map[string]any{"spotify_playlist_id": "sp3", "youtube_playlist_id": "yt3"})
	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Len(t, syncer.started, 1)

	for name, payload := range map[string]map[string]any{
		"bad start": {"schedule_start": "25:00"},
		"no days":   {"schedule_days": 0},
		"bad zone":  {"schedule_tz": "Nowhere/Land"},
		"too often": {"interval_minutes": 1},
	} {
		payload["spotify_playlist_id"], payload["youtube_playlist_id"] = "sp-"+name, "yt-"+name
		assert.Equal(t, http.StatusUnprocessableEntity, send(t, e, http.MethodPost, mappingsURL, payload).Code, name)
	}

	// Editing the schedule moves the next run.
	rec = send(t, e, http.MethodPatch, mappingsURL+"/"+got.ID, map[string]any{"schedule_days": schedule.EveryDay})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, time.Date(2026, 10, 24, 3, 0, 0, 0, time.UTC).Unix(), *got.NextRunAt)
	assert.Equal(t, "03:00", got.ScheduleStart, "untouched fields keep their value")
	assert.Equal(t, http.StatusUnprocessableEntity, send(t, e, http.MethodPatch, mappingsURL+"/"+got.ID, map[string]any{"schedule_start": "x"}).Code)
}

// conflictingCreator creates a playlist whose ID clashes with an existing mapping.
type conflictingCreator struct{ fakeCreator }

func (c *conflictingCreator) CreatePlaylist(ctx context.Context, service, name, desc string) (string, error) {
	c.calls = append(c.calls, service+":"+name)
	return "taken", nil
}

func TestMappingsCreate_CleansUpCreatedPlaylistWhenTheMappingFails(t *testing.T) {
	db, cleanup := setupMappingsTestDB(t)
	defer cleanup()
	creator := &conflictingCreator{}
	h := NewMappingsHandler(db)
	h.Playlists = creator
	e := echo.New()
	RegisterMappingsRoutes(e.Group(mappingsURL), h)
	require.Equal(t, http.StatusCreated, send(t, e, http.MethodPost, mappingsURL, map[string]any{"spotify_playlist_id": "sp0", "youtube_playlist_id": "taken"}).Code)

	rec := send(t, e, http.MethodPost, mappingsURL, map[string]any{"spotify_playlist_id": "sp1", "spotify_playlist_name": "Mix", "create_playlist": "youtube"})
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, []string{"youtube:taken"}, creator.deleted, "the orphan is removed")
}

func TestMappingsDelete_WithPlaylists(t *testing.T) {
	db, cleanup := setupMappingsTestDB(t)
	defer cleanup()
	creator := &fakeCreator{fail: map[string]error{"spotify": errors.New("spotify said no")}}
	limit := 10000
	h := NewMappingsHandler(db)
	h.Playlists = creator
	h.Budget = testBudget(&limit)
	e := echo.New()
	RegisterMappingsRoutes(e.Group(mappingsURL), h)

	create := func(sp, yt string) string {
		rec := send(t, e, http.MethodPost, mappingsURL, map[string]any{"spotify_playlist_id": sp, "youtube_playlist_id": yt, "sync_now": false})
		require.Equal(t, http.StatusCreated, rec.Code)
		var m MappingResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &m))
		return m.ID
	}
	_, err := db.Exec(`INSERT INTO playlist_cache (service, playlist_id, tracks_json, item_count, read_at) VALUES ('youtube', 'yt1', '[]', 0, 0)`)
	require.NoError(t, err)

	// Default: only the mapping goes.
	id := create("sp1", "yt1")
	rec := send(t, e, http.MethodDelete, mappingsURL+"/"+id, nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, creator.deleted)
	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM playlist_cache`).Scan(&n))
	assert.Zero(t, n, "its cached reads go too")

	// Both: YouTube deleted, Spotify's failure reported; the mapping is gone either way.
	id = create("sp1", "yt1")
	rec = send(t, e, http.MethodDelete, mappingsURL+"/"+id+"?delete_spotify_playlist=true&delete_youtube_playlist=true", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var resp DeleteResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.True(t, resp.Deleted)
	require.Len(t, resp.Playlists, 2)
	assert.Equal(t, PlaylistDeletion{Service: "spotify", Error: "spotify said no"}, resp.Playlists[0])
	assert.Equal(t, PlaylistDeletion{Service: "youtube", Deleted: true}, resp.Playlists[1])
	assert.Equal(t, http.StatusNotFound, send(t, e, http.MethodGet, mappingsURL+"/"+id, nil).Code)

	// No quota for the YouTube delete: reported, not attempted.
	limit = 60
	id = create("sp2", "yt2")
	rec = send(t, e, http.MethodDelete, mappingsURL+"/"+id+"?delete_youtube_playlist=true", nil)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Playlists, 1)
	assert.False(t, resp.Playlists[0].Deleted)
	assert.Contains(t, resp.Playlists[0].Error, "quota")
	assert.Equal(t, []string{"youtube:yt1"}, creator.deleted)
}

func TestMappingsList_PerSideCountsAndQueue(t *testing.T) {
	db, cleanup := setupMappingsTestDB(t)
	defer cleanup()
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, spotify_tracks_count, youtube_tracks_count, created, updated) VALUES ('m1','sp','yt',1,1,60,3,0,3,?,?)`, now, now)
	require.NoError(t, err)
	for i, status := range []string{"pending", "pending", "running", "error", "review", "done"} {
		_, err := db.Exec(`INSERT INTO sync_items (id, mapping_id, operation, service, track_id, status, attempt_count, created, updated) VALUES (?, 'm1', 'add', 'youtube', ?, ?, 0, ?, ?)`,
			"i"+string(rune('a'+i)), "t"+string(rune('a'+i)), status, now, now)
		require.NoError(t, err)
	}
	e := echo.New()
	RegisterMappingsRoutes(e.Group(mappingsURL), NewMappingsHandler(db))
	rec := send(t, e, http.MethodGet, mappingsURL, nil)
	var list MappingsListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Len(t, list.Items, 1)
	m := list.Items[0]
	assert.Equal(t, 0, m.SpotifyTracksCount)
	assert.Equal(t, 3, m.YoutubeTracksCount)
	assert.Equal(t, &QueueCounts{Pending: 3, Error: 1, Review: 1, Done: 1}, m.Queue)
}

func TestMappingsRecommend(t *testing.T) {
	db, cleanup := setupMappingsTestDB(t)
	defer cleanup()
	h := NewMappingsHandler(db)
	h.DailyLimit = func() int { return 10000 }
	e := echo.New()
	RegisterMappingsRoutes(e.Group(mappingsURL), h)

	rec := send(t, e, http.MethodGet, mappingsURL+"/schedule-recommendation?spotify_tracks=2000&youtube_videos=10&tz=Europe/London", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var r schedule.Recommendation
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &r))
	assert.Equal(t, 360, r.Schedule.IntervalMinutes)
	assert.Equal(t, "Europe/London", r.Schedule.TZ)
	assert.Equal(t, http.StatusBadRequest, send(t, e, http.MethodGet, mappingsURL+"/schedule-recommendation?tz=Nowhere", nil).Code)
}

func TestMappingsMirror(t *testing.T) {
	db, cleanup := setupMappingsTestDB(t)
	defer cleanup()
	creator := &fakeCreator{}
	syncer := &fakeSyncer{}
	limit := 10000
	h := NewMappingsHandler(db)
	h.Playlists = creator
	h.Syncer = syncer
	h.Budget = testBudget(&limit)
	e := echo.New()
	RegisterMappingsRoutes(e.Group(mappingsURL), h)
	require.Equal(t, http.StatusCreated, send(t, e, http.MethodPost, mappingsURL, map[string]any{"spotify_playlist_id": "paired", "youtube_playlist_id": "yt0", "sync_now": false}).Code)

	rec := send(t, e, http.MethodPost, mappingsURL+"/mirror", map[string]any{
		"source": "spotify",
		"playlists": []map[string]any{
			{"id": "a", "name": "Alpha", "tracks": 40},
			{"id": "paired", "name": "Already", "tracks": 5},
			{"id": "b", "name": " ", "tracks": 1},
			{"id": "c", "name": "Gamma", "tracks": 9000},
		},
		"schedule_tz": "UTC",
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out struct{ Items []MirrorResult }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Len(t, out.Items, 4)
	statuses := []string{}
	for _, it := range out.Items {
		statuses = append(statuses, it.Status)
	}
	assert.Equal(t, []string{"created", "already_paired", "failed", "created"}, statuses)
	assert.Equal(t, "Alpha", out.Items[0].Mapping.YoutubePlaylistName)
	assert.Equal(t, "a", out.Items[0].Mapping.SpotifyPlaylistID)
	assert.Equal(t, 30, out.Items[0].Mapping.IntervalMinutes, "small playlist: frequent")
	assert.Equal(t, 1440, out.Items[3].Mapping.IntervalMinutes, "huge playlist: daily")
	assert.NotEqual(t, out.Items[0].Mapping.ScheduleStart, out.Items[3].Mapping.ScheduleStart)
	assert.Len(t, syncer.started, 2, "first syncs are queued")

	// Quota runs out part-way: the rest wait, nothing is half-made.
	limit = 160 // room for two 50-unit creates after the margin
	rec = send(t, e, http.MethodPost, mappingsURL+"/mirror", map[string]any{
		"source":    "spotify",
		"playlists": []map[string]any{{"id": "d", "name": "D"}, {"id": "e", "name": "E"}, {"id": "f", "name": "F"}},
	})
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	statuses = statuses[:0]
	for _, it := range out.Items {
		statuses = append(statuses, it.Status)
	}
	// The fake creator doesn't spend quota, so the budget never fills: all three are created.
	// Spend it for real and the next one waits.
	assert.Equal(t, []string{"created", "created", "created"}, statuses)
	require.NoError(t, h.Budget.Charge(context.Background(), 100))
	rec = send(t, e, http.MethodPost, mappingsURL+"/mirror", map[string]any{
		"source":    "youtube",
		"playlists": []map[string]any{{"id": "yt-x", "name": "X"}, {"id": "yt-y", "name": "Y"}},
	})
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	// Creating on Spotify costs no YouTube quota.
	assert.Equal(t, "created", out.Items[0].Status)
	assert.Equal(t, "yt-x", out.Items[0].Mapping.YoutubePlaylistID)

	rec = send(t, e, http.MethodPost, mappingsURL+"/mirror", map[string]any{
		"source":    "spotify",
		"playlists": []map[string]any{{"id": "g", "name": "G"}, {"id": "h", "name": "H"}},
	})
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	assert.Equal(t, "waiting_quota", out.Items[0].Status)
	assert.Equal(t, "waiting_quota", out.Items[1].Status, "the rest aren't tried")

	assert.Equal(t, http.StatusUnprocessableEntity, send(t, e, http.MethodPost, mappingsURL+"/mirror", map[string]any{"source": "tidal", "playlists": []map[string]any{{"id": "x", "name": "x"}}}).Code)
	assert.Equal(t, http.StatusUnprocessableEntity, send(t, e, http.MethodPost, mappingsURL+"/mirror", map[string]any{"source": "spotify"}).Code)
}

func TestAppSettingsAPI(t *testing.T) {
	db, cleanup := setupMappingsTestDB(t)
	defer cleanup()
	store, err := settings.Load(db, func(k string) (string, bool) {
		if k == "SYNC_WORKERS_ENABLED" {
			return "true", true
		}
		return "", false
	})
	require.NoError(t, err)
	e := echo.New()
	RegisterAppSettingsRoutes(e.Group("/api/settings"), &AppSettingsHandler{Store: store})

	rec := send(t, e, http.MethodPatch, "/api/settings", map[string]any{"sync_workers_enabled": false, "drain_items_per_tick": 25})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out struct{ Items []settings.View }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	byKey := map[string]settings.View{}
	for _, v := range out.Items {
		byKey[v.Key] = v
	}
	assert.Equal(t, settings.SourceRuntime, byKey["sync_workers_enabled"].Source)
	assert.Equal(t, "false", byKey["sync_workers_enabled"].Value)
	assert.Equal(t, settings.SourceDatabase, byKey["drain_items_per_tick"].Source)
	assert.Equal(t, "25", byKey["drain_items_per_tick"].Value)

	// One bad value: nothing changes.
	rec = send(t, e, http.MethodPatch, "/api/settings", map[string]any{"drain_items_per_tick": 50, "youtube_daily_quota": 1})
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Equal(t, 25, store.Int(settings.DrainItemsPerTick))
	assert.Equal(t, http.StatusBadRequest, send(t, e, http.MethodPatch, "/api/settings", map[string]any{"nope": 1}).Code)
	assert.Equal(t, http.StatusBadRequest, send(t, e, http.MethodPatch, "/api/settings", map[string]any{}).Code)
}

type fakeSpotifyReader struct{ tracks []matching.Track }

func (f *fakeSpotifyReader) ListPlaylistTracks(context.Context, string) ([]matching.Track, error) {
	return f.tracks, nil
}

func TestQuotaAPI(t *testing.T) {
	db, cleanup := setupMappingsTestDB(t)
	defer cleanup()
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES ('m1','sp','yt',1,1,60,0,?,?)`, now, now)
	require.NoError(t, err)
	for i := 0; i < 4; i++ {
		_, err := db.Exec(`INSERT INTO sync_items (id, mapping_id, operation, service, track_id, status, attempt_count, created, updated) VALUES (?, 'm1', 'add', 'youtube', ?, 'pending', 0, ?, ?)`,
			"q"+string(rune('0'+i)), "s"+string(rune('0'+i)), now, now)
		require.NoError(t, err)
	}
	ctx := context.Background()
	require.NoError(t, (&tracklinks.Store{DB: db}).LinkAuto(ctx, "s0", "v0", "T", "A", 1))

	b := quota.New(db, func() int { return 10000 }, func() int { return 500 }, func() int { return 500 })
	h := &QuotaHandler{DB: db, Budget: b, Spotify: &fakeSpotifyReader{tracks: []matching.Track{{ID: "s0"}, {ID: "new1"}, {ID: "new2"}}}}
	e := echo.New()
	RegisterQuotaRoutes(e.Group("/api/quota"), h)

	rec := send(t, e, http.MethodGet, "/api/quota", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	var st QuotaStatusResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &st))
	assert.Equal(t, 1, st.Backlog.YouTubeLinked)
	assert.Equal(t, 3, st.Backlog.YouTubeUnlinked)
	assert.Equal(t, 50+3*151, st.Backlog.Units)
	assert.Equal(t, 9000, st.DailyCapacity)
	assert.Equal(t, 0, st.DaysToDrain)
	assert.True(t, strings.Contains(rec.Body.String(), `"reset_at"`))

	// Exact: one of the three Spotify songs is already matched.
	rec = send(t, e, http.MethodPost, "/api/quota/estimate", map[string]any{"pairs": []map[string]any{{"spotify_playlist_id": "sp9", "spotify_tracks": 3, "create": "youtube"}}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var est EstimateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &est))
	assert.True(t, est.Exact)
	assert.Equal(t, 2, est.Work.ToYouTube)
	assert.Equal(t, 1, est.Work.ToYouTubeLinked)
	assert.Equal(t, 2*151+50+50, est.Units)
	assert.False(t, est.UpperBound)

	// Two existing playlists: an upper bound; a huge one flags the 5,000-video limit.
	rec = send(t, e, http.MethodPost, "/api/quota/estimate", map[string]any{"pairs": []map[string]any{{"spotify_tracks": 4000, "youtube_videos": 2000}}})
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &est))
	assert.True(t, est.UpperBound)
	assert.True(t, est.OverYouTubeLimit)
	assert.False(t, est.Exact)
	assert.Greater(t, est.Days, 0)

	assert.Equal(t, http.StatusBadRequest, send(t, e, http.MethodPost, "/api/quota/estimate", map[string]any{"pairs": []any{}}).Code)
}
