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

func TestAnalysisJobGetMappingsForAnalysis(t *testing.T) {
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

	// Create test mappings
	now := time.Now().Unix()

	// Mapping that needs analysis (no last_analysis_at)
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"mapping1", "spotify1", "youtube1", 1, 1, 60, 0, now, now)
	require.NoError(t, err)

	// Mapping that was analyzed recently (should be skipped)
	_, err = db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, last_analysis_at, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"mapping2", "spotify2", "youtube2", 1, 1, 60, 0, now-30*60, now, now) // 30 minutes ago
	require.NoError(t, err)

	// Mapping that needs analysis (old timestamp)
	_, err = db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, last_analysis_at, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"mapping3", "spotify3", "youtube3", 1, 1, 60, 0, now-2*60*60, now, now) // 2 hours ago
	require.NoError(t, err)

	mappings, err := job.getMappingsForAnalysis(context.Background())
	require.NoError(t, err)

	// Should return mapping1 (null timestamp) and mapping3 (old timestamp)
	assert.Len(t, mappings, 2)

	ids := []string{}
	for _, m := range mappings {
		ids = append(ids, *m.ID)
	}
	assert.Contains(t, ids, "mapping1")
	assert.Contains(t, ids, "mapping3")
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
	syncItems := job.generateSyncItems(mapping, spotifyTracks, youtubeTracks)

	// Should generate sync items:
	// - spotify2, spotify3 -> add to youtube (2 items)
	// - youtube2, youtube3 -> add to spotify (2 items)
	// Total: 4 items
	assert.Len(t, syncItems, 4)

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

	syncItems := job.generateSyncItems(mapping, nil, nil)
	require.Len(t, syncItems, 1)
	assert.Equal(t, "rename", syncItems[0].Operation)
	assert.Equal(t, "youtube", syncItems[0].Service)
	assert.Equal(t, "Spotify Title", *syncItems[0].TrackTitle)
	assert.Equal(t, renameTrackIDKey, *syncItems[0].TrackID)
}

func TestAnalysisJobInsertSyncItemsRenameReplacesStale(t *testing.T) {
	db, cleanup := setupAnalysisTestDB(t)
	defer cleanup()

	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"map-r", "sp-pl", "yt-pl", 1, 0, 60, 0, now, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO sync_items (id, mapping_id, operation, service, track_id, track_title, status, attempt_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"old-rename", "map-r", "rename", "youtube", "PLold", "Old Title", "done", 1, now, now)
	require.NoError(t, err)

	job := NewAnalysisJob(JobDeps{DB: db, Logger: zerolog.Nop(), ActivityLogger: activitylogger.New(db)}, nil)
	item := model.SyncItems{
		ID:           stringPtr("new-rename"),
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

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sync_items WHERE mapping_id = ? AND operation = 'rename'`, "map-r").Scan(&count))
	assert.Equal(t, 1, count)

	var title string
	require.NoError(t, db.QueryRow(`SELECT track_title FROM sync_items WHERE mapping_id = ? AND operation = 'rename'`, "map-r").Scan(&title))
	assert.Equal(t, "New Title", title)
}

func TestAnalysisJobContainsTrack(t *testing.T) {
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

	tracks := []TrackInfo{
		{ID: "track1", Title: "Song A", Artist: "Artist 1"},
		{ID: "track2", Title: "Song B", Artist: "Artist 2"},
	}

	// Test exact ID match
	assert.True(t, job.containsTrack(tracks, TrackInfo{ID: "track1", Title: "Different", Artist: "Different"}))

	// Test title/artist match (case insensitive)
	assert.True(t, job.containsTrack(tracks, TrackInfo{ID: "different", Title: "song a", Artist: "artist 1"}))

	// Test no match
	assert.False(t, job.containsTrack(tracks, TrackInfo{ID: "different", Title: "Different Song", Artist: "Different Artist"}))
}

func TestAnalysisJobUpdateMappingTimestamp(t *testing.T) {
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

	// Create test mapping
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"test-mapping", "spotify1", "youtube1", 1, 1, 60, 0, now-100, now-100)
	require.NoError(t, err)

	// Update timestamp
	err = job.updateMappingAnalysisTimestamp(context.Background(), "test-mapping", 3, 5)
	require.NoError(t, err)

	// Verify timestamp was updated
	var lastAnalysisAt, updated sql.NullInt64
	err = db.QueryRow("SELECT last_analysis_at, updated FROM mappings WHERE id = ?", "test-mapping").Scan(&lastAnalysisAt, &updated)
	require.NoError(t, err)

	assert.True(t, lastAnalysisAt.Valid)
	assert.True(t, updated.Valid)
	assert.True(t, lastAnalysisAt.Int64 >= now)
	assert.True(t, updated.Int64 >= now)
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
