package jobs

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/activitylogger"
	"github.com/manlikeabro/spotube/internal/auth"
	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/matching"
	"github.com/manlikeabro/spotube/internal/migrate"
	"github.com/manlikeabro/spotube/internal/sqliteconn"
)

func setupAnalysisTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "analysis_test.db")

	db, err := sqliteconn.OpenWithPragmas(dbPath)
	require.NoError(t, err)

	require.NoError(t, migrate.Up(db))

	cleanup := func() {
		migrate.Down(db)
		db.Close()
	}
	return db, cleanup
}

func TestAnalysisJobDueMappings(t *testing.T) {
	db, cleanup := setupAnalysisTestDB(t)
	defer cleanup()

	logger := zerolog.Nop()
	activityLogger := activitylogger.New(db)

	// Mock client factory (won't be used in this test)
	tokenRepo := auth.NewSQLiteTokenRepository(db)
	credsRepo := &testAnalysisCredentialsRepo{}
	clientFactory := auth.NewClientFactory(db, credsRepo, tokenRepo)

	job := NewAnalysisJob(JobDeps{
		DB:             db,
		Logger:         logger,
		ActivityLogger: activityLogger,
	}, clientFactory)

	now := time.Now().Unix()
	insert := func(id string, lastAnalysis, nextRun any) {
		_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, last_analysis_at, next_run_at, created, updated) VALUES (?, ?, ?, 1, 1, 60, 0, ?, ?, ?, ?)`,
			id, "sp-"+id, "yt-"+id, lastAnalysis, nextRun, now, now)
		require.NoError(t, err)
	}
	insert("never-run", nil, nil)      // never analysed: due
	insert("not-yet", now-60, now+600) // next run in 10 minutes
	insert("due", now-2*3600, now-60)  // next run a minute ago
	insert("missed", now-3*3600, nil)  // from before schedules; an hourly run has passed
	_, err := db.Exec(`UPDATE mappings SET sync_tracks = 0, sync_name = 0 WHERE id = 'not-yet'`)
	require.NoError(t, err)
	insert("disabled", nil, nil)
	_, err = db.Exec(`UPDATE mappings SET sync_tracks = 0, sync_name = 0 WHERE id = 'disabled'`)
	require.NoError(t, err)

	mappings, err := job.dueMappings(time.Now())
	require.NoError(t, err)
	ids := []string{}
	for _, m := range mappings {
		ids = append(ids, *m.ID)
	}
	assert.ElementsMatch(t, []string{"never-run", "due", "missed"}, ids)
}

func TestTrackInfosToMatching_SongAMatchesBothWays(t *testing.T) {
	spotifyTracks := []TrackInfo{
		{ID: "spotify1", Title: "Song A", Artist: "Artist 1"},
	}
	youtubeTracks := []TrackInfo{
		{ID: "youtube1", Title: "Song A", Artist: "Artist 1"},
	}

	assert.True(t, matching.FindMatch(
		matching.SpotifyTrack("spotify1", "Song A", "Artist 1"),
		trackInfosToYouTubeMatching(youtubeTracks),
	).Matched)
	assert.True(t, matching.FindMatch(
		matching.YouTubeFromRaw("youtube1", "Song A"),
		trackInfosToSpotifyMatching(spotifyTracks),
	).Matched)
}

func TestAnalysisJobGenerateSyncItems(t *testing.T) {
	db, cleanup := setupAnalysisTestDB(t)
	defer cleanup()

	logger := zerolog.Nop()
	activityLogger := activitylogger.New(db)

	tokenRepo := auth.NewSQLiteTokenRepository(db)
	credsRepo := &testAnalysisCredentialsRepo{}
	clientFactory := auth.NewClientFactory(db, credsRepo, tokenRepo)

	job := NewAnalysisJob(JobDeps{
		DB:             db,
		Logger:         logger,
		ActivityLogger: activityLogger,
	}, clientFactory)

	// Test track comparison logic
	spotifyTracks := []TrackInfo{
		{ID: "spotify1", Title: "Song A", Artist: "Artist 1"},
		{ID: "spotify2", Title: "Song B", Artist: "Artist 2"},
		{ID: "spotify3", Title: "Song C", Artist: "Artist 3"},
	}

	youtubeTracks := []TrackInfo{
		{ID: "youtube1", Title: "Song A", Artist: "Artist 1"}, // Same as spotify1
		{ID: "youtube2", Title: "Song D", Artist: "Artist 4"}, // Different
		{ID: "youtube3", Title: "Song E", Artist: "Artist 5"}, // Different
	}

	mapping := model.Mappings{
		ID:                stringPtr("mapping1"),
		SyncName:          0,
		SyncTracks:        1,
		YoutubePlaylistID: "youtube-playlist",
	}
	result := job.generateSyncItems(mapping, spotifyTracks, youtubeTracks, nil)
	syncItems := result.items

	// Should generate sync items:
	// - spotify2, spotify3 -> add to youtube (2 items)
	// - youtube2, youtube3 -> add to spotify (2 items)
	// Total: 4 items
	assert.Len(t, syncItems, 4)
	assert.Equal(t, 2, result.matchedSkipped)

	// Verify items have proper structure
	for _, item := range syncItems {
		assert.NotNil(t, item.ID)
		assert.Equal(t, "mapping1", item.MappingID)
		assert.Equal(t, "add", item.Operation)
		assert.Contains(t, []string{"spotify", "youtube"}, item.Service)
		assert.Equal(t, "pending", item.Status)
		assert.Equal(t, int32(0), item.AttemptCount)
	}
}

func TestAnalysisJobGenerateSyncItemsRename(t *testing.T) {
	db, cleanup := setupAnalysisTestDB(t)
	defer cleanup()

	logger := zerolog.Nop()
	activityLogger := activitylogger.New(db)
	tokenRepo := auth.NewSQLiteTokenRepository(db)
	credsRepo := &testAnalysisCredentialsRepo{}
	clientFactory := auth.NewClientFactory(db, credsRepo, tokenRepo)

	job := NewAnalysisJob(JobDeps{
		DB:             db,
		Logger:         logger,
		ActivityLogger: activityLogger,
	}, clientFactory)

	spotifyName := "Spotify Title"
	youtubeName := "YouTube Title"
	mapping := model.Mappings{
		ID:                  stringPtr("mapping-rename"),
		SyncName:            1,
		SyncTracks:          0,
		YoutubePlaylistID:   "yt-playlist",
		SpotifyPlaylistName: &spotifyName,
		YoutubePlaylistName: &youtubeName,
	}

	syncItems := job.generateSyncItems(mapping, nil, nil, nil).items
	require.Len(t, syncItems, 1)
	assert.Equal(t, "rename", syncItems[0].Operation)
	assert.Equal(t, "youtube", syncItems[0].Service)
	assert.Equal(t, "Spotify Title", *syncItems[0].TrackTitle)
	assert.Equal(t, renameTrackIDKey, *syncItems[0].TrackID)
}

func TestAnalysisJobGenerateSyncItems_PeloNegroDoesNotQueueDuplicate(t *testing.T) {
	db, cleanup := setupAnalysisTestDB(t)
	defer cleanup()

	job := NewAnalysisJob(JobDeps{DB: db, Logger: zerolog.Nop(), ActivityLogger: activitylogger.New(db)}, nil)

	spotifyTracks := []TrackInfo{
		{ID: "sp-pelo", Title: "Pelo Negro", Artist: "Fernando Milagros"},
	}
	youtubeTracks := []TrackInfo{
		{
			ID:       "yt-pelo",
			Title:    "Pelo Negro - Fernando Milagros (Official Video)",
			RawTitle: "Pelo Negro - Fernando Milagros (Official Video)",
		},
	}

	mapping := model.Mappings{
		ID:                stringPtr("mapping-pelo"),
		SyncName:          0,
		SyncTracks:        1,
		YoutubePlaylistID: "youtube-playlist",
	}
	result := job.generateSyncItems(mapping, spotifyTracks, youtubeTracks, nil)
	assert.Empty(t, result.items)
	assert.Equal(t, 2, result.matchedSkipped)
}

func TestAnalysisJobInsertSyncItemsRenamePreservesID(t *testing.T) {
	db, cleanup := setupAnalysisTestDB(t)
	defer cleanup()

	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"map-r", "sp-pl", "yt-pl", 1, 0, 60, 0, now, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO sync_items (id, mapping_id, operation, service, track_id, track_title, status, attempt_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"rename-stable-id", "map-r", "rename", "youtube", renameTrackIDKey, "Old Title", "done", 1, now, now)
	require.NoError(t, err)

	job := NewAnalysisJob(JobDeps{DB: db, Logger: zerolog.Nop(), ActivityLogger: activitylogger.New(db)}, nil)
	item := model.SyncItems{
		ID:           stringPtr("would-be-new-id"),
		MappingID:    "map-r",
		Operation:    "rename",
		Service:      "youtube",
		TrackID:      stringPtr(renameTrackIDKey),
		TrackTitle:   stringPtr("New Title"),
		Status:       "pending",
		AttemptCount: 0,
		Created:      int32(now),
		Updated:      int32(now),
	}

	inserted, err := job.insertSyncItems(context.Background(), []model.SyncItems{item})
	require.NoError(t, err)
	assert.Equal(t, 1, inserted)

	var id, title, status string
	require.NoError(t, db.QueryRow(
		`SELECT id, track_title, status FROM sync_items WHERE mapping_id = ? AND operation = 'rename'`,
		"map-r",
	).Scan(&id, &title, &status))
	assert.Equal(t, "rename-stable-id", id)
	assert.Equal(t, "New Title", title)
	assert.Equal(t, "pending", status)
}

func TestAnalysisJobContainsTrack(t *testing.T) {
	candidates := trackInfosToSpotifyMatching([]TrackInfo{
		{ID: "track1", Title: "Song A", Artist: "Artist 1"},
		{ID: "track2", Title: "Song B", Artist: "Artist 2"},
	})

	// Metadata match (IDs alone must not match across different titles)
	assert.True(t, matching.FindMatch(
		matching.SpotifyTrack("other-id", "Song A", "Artist 1"),
		candidates,
	).Matched)

	// Shared ID strings with different metadata must not match
	assert.False(t, matching.FindMatch(
		matching.SpotifyTrack("track1", "Different Song", "Different Artist"),
		[]matching.Track{{Platform: matching.PlatformSpotify, ID: "track1", Title: "Other", Artist: "Other"}},
	).Matched)

	// No match
	assert.False(t, matching.FindMatch(
		matching.SpotifyTrack("x", "Different Song", "Different Artist"),
		candidates,
	).Matched)
}

func TestAnalysisJobUpdateMappingCounts(t *testing.T) {
	db, cleanup := setupAnalysisTestDB(t)
	defer cleanup()

	logger := zerolog.Nop()
	activityLogger := activitylogger.New(db)

	tokenRepo := auth.NewSQLiteTokenRepository(db)
	credsRepo := &testAnalysisCredentialsRepo{}
	clientFactory := auth.NewClientFactory(db, credsRepo, tokenRepo)

	job := NewAnalysisJob(JobDeps{
		DB:             db,
		Logger:         logger,
		ActivityLogger: activityLogger,
	}, clientFactory)

	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"test-mapping", "spotify1", "youtube1", 1, 1, 60, 0, now-100, now-100)
	require.NoError(t, err)

	// An empty Spotify side next to 3 videos is reported as 0 and 3, not "3 tracks".
	require.NoError(t, job.updateMappingCounts(context.Background(), "test-mapping", 0, 3, true))

	var spotifyCount, youtubeCount int
	var itemCount, fullRead, updated sql.NullInt64
	require.NoError(t, db.QueryRow("SELECT spotify_tracks_count, youtube_tracks_count, youtube_item_count, last_full_read_at, updated FROM mappings WHERE id = ?", "test-mapping").
		Scan(&spotifyCount, &youtubeCount, &itemCount, &fullRead, &updated))
	assert.Equal(t, 0, spotifyCount)
	assert.Equal(t, 3, youtubeCount)
	assert.Equal(t, int64(3), itemCount.Int64)
	assert.True(t, fullRead.Int64 >= now)
	assert.True(t, updated.Int64 >= now)

	// Without a fresh read the probe baseline stays.
	require.NoError(t, job.updateMappingCounts(context.Background(), "test-mapping", 2, 4, false))
	require.NoError(t, db.QueryRow("SELECT youtube_item_count FROM mappings WHERE id = ?", "test-mapping").Scan(&itemCount))
	assert.Equal(t, int64(3), itemCount.Int64)
}

type testAnalysisCredentialsRepo struct{}

func (r *testAnalysisCredentialsRepo) GetSettings() (*auth.SettingsRecord, error) {
	return &auth.SettingsRecord{
		SpotifyClientID:     sql.NullString{String: "test-spotify-id", Valid: true},
		SpotifyClientSecret: sql.NullString{String: "test-spotify-secret", Valid: true},
		GoogleClientID:      sql.NullString{String: "test-google-id", Valid: true},
		GoogleClientSecret:  sql.NullString{String: "test-google-secret", Valid: true},
	}, nil
}

func TestReconcileQueuedAdds_SkipsItemsNoLongerNeeded(t *testing.T) {
	db, cleanup := setupAnalysisTestDB(t)
	defer cleanup()

	job := NewAnalysisJob(JobDeps{DB: db, Logger: zerolog.Nop(), ActivityLogger: activitylogger.New(db)}, nil)
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES ('m1','sp','yt',0,1,60,0,?,?)`, now, now)
	require.NoError(t, err)

	insert := func(id, service, trackID, status string) {
		_, err := db.Exec(`INSERT INTO sync_items (id, mapping_id, operation, service, track_id, status, created, updated) VALUES (?, 'm1', 'add', ?, ?, ?, ?, ?)`,
			id, service, trackID, status, now, now)
		require.NoError(t, err)
	}
	insert("still-wanted", "youtube", "sp-missing", "pending")
	insert("now-matched", "youtube", "sp-matched", "pending")
	insert("removed", "spotify", "yt-gone", "error")
	insert("finished", "spotify", "yt-done", "done")

	wanted := []model.SyncItems{{Operation: "add", Service: "youtube", TrackID: stringPtr("sp-missing")}}
	spotifyTracks := []TrackInfo{{ID: "sp-missing"}, {ID: "sp-matched"}}
	youtubeTracks := []TrackInfo{{ID: "yt-done"}}

	require.NoError(t, job.reconcileQueuedAdds(context.Background(), "m1", wanted, spotifyTracks, youtubeTracks, nil))

	status := func(id string) (string, string) {
		var s string
		var msg sql.NullString
		require.NoError(t, db.QueryRow(`SELECT status, error_message FROM sync_items WHERE id = ?`, id).Scan(&s, &msg))
		return s, msg.String
	}
	s, _ := status("still-wanted")
	assert.Equal(t, "pending", s)
	s, msg := status("now-matched")
	assert.Equal(t, "skipped", s)
	assert.Equal(t, "already in destination playlist", msg)
	s, msg = status("removed")
	assert.Equal(t, "skipped", s)
	assert.Equal(t, "no longer in source playlist", msg)
	s, _ = status("finished")
	assert.Equal(t, "done", s)
}

func newTestAnalysisJob(db *sql.DB) *AnalysisJob {
	clientFactory := auth.NewClientFactory(db, &testAnalysisCredentialsRepo{}, auth.NewSQLiteTokenRepository(db))
	return NewAnalysisJob(JobDeps{DB: db, Logger: zerolog.Nop(), ActivityLogger: activitylogger.New(db)}, clientFactory)
}

func loadTestMapping(t *testing.T, db *sql.DB, id string) model.Mappings {
	t.Helper()
	var m model.Mappings
	var lastErr sql.NullString
	require.NoError(t, db.QueryRow(`SELECT id, sync_tracks, sync_name, last_sync_error FROM mappings WHERE id = ?`, id).
		Scan(&id, &m.SyncTracks, &m.SyncName, &lastErr))
	m.ID = &id
	if lastErr.Valid {
		m.LastSyncError = &lastErr.String
	}
	return m
}

func TestSyncMapping_RepeatedFailureLoggedOnce(t *testing.T) {
	db, cleanup := setupAnalysisTestDB(t)
	defer cleanup()

	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES ('m1','sp','yt',0,1,5,0,?,?)`, now, now)
	require.NoError(t, err)

	job := newTestAnalysisJob(db)
	for i := 0; i < 3; i++ {
		// No tokens are stored, so every run fails the same way.
		job.SyncMapping(context.Background(), loadTestMapping(t, db, "m1"))
	}

	var errors int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM activity_logs WHERE mapping_id = 'm1' AND level = 'error'`).Scan(&errors))
	assert.Equal(t, 1, errors)

	var status string
	require.NoError(t, db.QueryRow(`SELECT last_sync_status FROM mappings WHERE id = 'm1'`).Scan(&status))
	assert.NotEqual(t, "ok", status)
}

func TestPruneActivityLogs(t *testing.T) {
	db, cleanup := setupAnalysisTestDB(t)
	defer cleanup()

	old := time.Now().Add(-activityLogRetention - time.Hour).Unix()
	recent := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO activity_logs (id, level, message, job_type, created) VALUES ('old','error','x','analysis',?), ('new','info','y','analysis',?)`, old, recent)
	require.NoError(t, err)

	newTestAnalysisJob(db).pruneActivityLogs()

	var ids []string
	rows, err := db.Query(`SELECT id FROM activity_logs`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var id string
		require.NoError(t, rows.Scan(&id))
		ids = append(ids, id)
	}
	assert.Equal(t, []string{"new"}, ids)
}
