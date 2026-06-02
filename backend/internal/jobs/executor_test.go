package jobs

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/activitylogger"
	"github.com/manlikeabro/spotube/internal/migrate"
	"github.com/manlikeabro/spotube/internal/sqliteconn"
)

type mockSpotifyPlatform struct {
	searchTrackID string
	searchErr     error
	addErr        error
	renameErr     error
	addCalls      []string
}

func (m *mockSpotifyPlatform) SearchTrack(_ context.Context, _, _ string) (string, error) {
	return m.searchTrackID, m.searchErr
}

func (m *mockSpotifyPlatform) AddTrackToPlaylist(_ context.Context, _, trackID string) error {
	m.addCalls = append(m.addCalls, trackID)
	return m.addErr
}

func (m *mockSpotifyPlatform) RenamePlaylist(context.Context, string, string) error {
	return m.renameErr
}

type mockYouTubePlatform struct {
	searchVideoID string
	searchErr     error
	addErr        error
	renameErr     error
	addCalls      []string
}

func (m *mockYouTubePlatform) SearchVideo(_ context.Context, _, _ string) (string, error) {
	return m.searchVideoID, m.searchErr
}

func (m *mockYouTubePlatform) AddVideoToPlaylist(_ context.Context, _, videoID string) error {
	m.addCalls = append(m.addCalls, videoID)
	return m.addErr
}

func (m *mockYouTubePlatform) RenamePlaylist(context.Context, string, string) error {
	return m.renameErr
}

func setupExecutorTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "executor_test.db")

	db, err := sqliteconn.OpenWithPragmas(dbPath)
	require.NoError(t, err)
	require.NoError(t, migrate.Up(db))

	return db, func() {
		migrate.Down(db)
		db.Close()
	}
}

func insertExecutorFixtures(t *testing.T, db *sql.DB, mappingID, itemID, operation, service, status string) {
	t.Helper()
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, spotify_playlist_name, youtube_playlist_name, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		mappingID, "sp-pl", "yt-pl", "Spotify List", "YouTube List", 1, 1, 60, 0, now, now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO sync_items (id, mapping_id, operation, service, track_id, track_title, track_artist, status, error_message, attempt_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', 0, ?, ?)`,
		itemID, mappingID, operation, service, "src-track-1", "Song A", "Artist A", status, now, now)
	require.NoError(t, err)
}

func newTestExecutor(t *testing.T, db *sql.DB, sp spotifyPlatform, yt youtubePlatform) *Executor {
	t.Helper()
	e := &Executor{
		db:             db,
		logger:         zerolog.Nop(),
		activityLogger: activitylogger.New(db),
		spotify:        sp,
		youtube:        yt,
	}
	return e
}

func TestExecuteOne_YouTubeAddSuccess(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()

	insertExecutorFixtures(t, db, "map-1", "item-1", "add", "youtube", "pending")

	yt := &mockYouTubePlatform{searchVideoID: "yt-video-99"}
	e := newTestExecutor(t, db, &mockSpotifyPlatform{}, yt)

	item, err := e.ExecuteOne(context.Background(), "item-1")
	require.NoError(t, err)
	assert.Equal(t, "done", item.Status)
	assert.Equal(t, 1, int(item.AttemptCount))
	assert.Equal(t, []string{"yt-video-99"}, yt.addCalls)
}

func TestExecuteOne_SpotifyAddSuccess(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()

	insertExecutorFixtures(t, db, "map-2", "item-2", "add", "spotify", "pending")

	sp := &mockSpotifyPlatform{searchTrackID: "sp-track-42"}
	e := newTestExecutor(t, db, sp, &mockYouTubePlatform{})

	item, err := e.ExecuteOne(context.Background(), "item-2")
	require.NoError(t, err)
	assert.Equal(t, "done", item.Status)
	assert.Equal(t, []string{"sp-track-42"}, sp.addCalls)
}

func TestExecuteOne_YouTubeRenameSuccess(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()

	insertExecutorFixtures(t, db, "map-3", "item-3", "rename", "youtube", "pending")
	_, err := db.Exec(`UPDATE sync_items SET track_title = ? WHERE id = ?`, "Canonical Name", "item-3")
	require.NoError(t, err)

	yt := &mockYouTubePlatform{}
	e := newTestExecutor(t, db, &mockSpotifyPlatform{}, yt)

	item, err := e.ExecuteOne(context.Background(), "item-3")
	require.NoError(t, err)
	assert.Equal(t, "done", item.Status)
}

func TestExecuteOne_SearchEmptyMarksError(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()

	insertExecutorFixtures(t, db, "map-4", "item-4", "add", "youtube", "pending")

	e := newTestExecutor(t, db, &mockSpotifyPlatform{}, &mockYouTubePlatform{searchVideoID: ""})

	item, err := e.ExecuteOne(context.Background(), "item-4")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no match on youtube")
	assert.Equal(t, "error", item.Status)
	assert.NotEmpty(t, stringFromPtr(item.ErrorMessage))
}

func TestExecuteOne_BlacklistedSkipped(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()

	insertExecutorFixtures(t, db, "map-5", "item-5", "add", "youtube", "pending")
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO blacklist (id, mapping_id, service, track_id, reason, skip_counter, created, updated) VALUES (?, ?, ?, ?, ?, 0, ?, ?)`,
		"bl-1", "map-5", "youtube", "src-track-1", "manual", now, now)
	require.NoError(t, err)

	e := newTestExecutor(t, db, &mockSpotifyPlatform{}, &mockYouTubePlatform{searchVideoID: "vid"})

	item, err := e.ExecuteOne(context.Background(), "item-5")
	require.ErrorIs(t, err, ErrSyncItemBlacklisted)
	assert.Equal(t, "skipped", item.Status)
}

func TestExecuteOne_NotExecutableDone(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()

	insertExecutorFixtures(t, db, "map-6", "item-6", "add", "youtube", "done")

	e := newTestExecutor(t, db, &mockSpotifyPlatform{}, &mockYouTubePlatform{})
	_, err := e.ExecuteOne(context.Background(), "item-6")
	require.ErrorIs(t, err, ErrSyncItemNotExecutable)
}

func TestExecuteOne_NotFound(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()

	e := newTestExecutor(t, db, &mockSpotifyPlatform{}, &mockYouTubePlatform{})
	_, err := e.ExecuteOne(context.Background(), "missing")
	require.ErrorIs(t, err, ErrSyncItemNotFound)
}

func TestExecuteOne_IdempotentAlreadyExists(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()

	insertExecutorFixtures(t, db, "map-7", "item-7", "add", "youtube", "pending")

	yt := &mockYouTubePlatform{
		searchVideoID: "vid-1",
		addErr:        errors.New("video already in playlist"),
	}
	e := newTestExecutor(t, db, &mockSpotifyPlatform{}, yt)

	item, err := e.ExecuteOne(context.Background(), "item-7")
	require.NoError(t, err)
	assert.Equal(t, "done", item.Status)
}

func TestExecuteOne_ReExecuteFromError(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()

	insertExecutorFixtures(t, db, "map-8", "item-8", "add", "spotify", "error")
	_, err := db.Exec(`UPDATE sync_items SET error_message = ?, attempt_count = 1 WHERE id = ?`, "previous failure", "item-8")
	require.NoError(t, err)

	sp := &mockSpotifyPlatform{searchTrackID: "sp-ok"}
	e := newTestExecutor(t, db, sp, &mockYouTubePlatform{})

	item, err := e.ExecuteOne(context.Background(), "item-8")
	require.NoError(t, err)
	assert.Equal(t, "done", item.Status)
	assert.Equal(t, 2, int(item.AttemptCount))
}

func TestIsIdempotentSuccess(t *testing.T) {
	assert.True(t, isIdempotentSuccess(errors.New("Item already exists")))
	assert.False(t, isIdempotentSuccess(errors.New("network timeout")))
}

func TestTruncateExecutorError(t *testing.T) {
	long := string(make([]byte, 600))
	assert.Len(t, truncateExecutorError(long), maxErrorMessageLen)
}
