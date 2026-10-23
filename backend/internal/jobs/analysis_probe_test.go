package jobs

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/db/table"
	"github.com/manlikeabro/spotube/internal/matching"
	"github.com/manlikeabro/spotube/internal/platform"
	"github.com/manlikeabro/spotube/internal/quota"
	"github.com/manlikeabro/spotube/internal/tracklinks"
)

// fakeReader serves playlists from memory and counts YouTube calls.
type fakeReader struct {
	spotify     []matching.Track
	youtube     []matching.Track
	extraVideos int // itemCount beyond len(youtube) (e.g. deleted videos)
	probeErr    error
	probes      int
	fullReads   int
	durationFor []string
}

func (f *fakeReader) SpotifyTracks(context.Context, string) ([]matching.Track, error) {
	return f.spotify, nil
}
func (f *fakeReader) SpotifyName(context.Context, string) (string, error) { return "Road Trip", nil }
func (f *fakeReader) YouTubeInfo(context.Context, string) (string, int, error) {
	f.probes++
	return "Road Trip", len(f.youtube) + f.extraVideos, f.probeErr
}
func (f *fakeReader) YouTubeTracks(_ context.Context, _ string, needDuration func(string) bool) ([]matching.Track, error) {
	f.fullReads++
	f.durationFor = nil
	for _, v := range f.youtube {
		if needDuration(v.ID) {
			f.durationFor = append(f.durationFor, v.ID)
		}
	}
	return f.youtube, nil
}

func sp(id, title, artist string) matching.Track {
	return matching.Track{Platform: matching.PlatformSpotify, ID: id, Title: title, Artist: artist}
}

// yt builds a video the way the YouTube reader does: raw title plus a "Topic" channel.
func yt(id, title, artist string) matching.Track {
	return matching.YouTubeTrack(id, title, artist+" - Topic")
}

func loadFullMapping(t *testing.T, db *sql.DB, id string) model.Mappings {
	t.Helper()
	var ms []model.Mappings
	require.NoError(t, table.Mappings.SELECT(table.Mappings.AllColumns).WHERE(table.Mappings.ID.EQ(sqlite.String(id))).Query(db, &ms))
	require.Len(t, ms, 1)
	return ms[0]
}

func probeFixture(t *testing.T) (*sql.DB, *AnalysisJob, *fakeReader) {
	t.Helper()
	db, cleanup := setupAnalysisTestDB(t)
	t.Cleanup(cleanup)
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES ('m1','sp1','yt1',0,1,60,0,?,?)`, now, now)
	require.NoError(t, err)
	r := &fakeReader{
		spotify: []matching.Track{sp("s1", "Shared Song", "Band"), sp("s2", "Only On Spotify", "Singer")},
		youtube: []matching.Track{yt("v1", "Shared Song", "Band"), yt("v2", "Only On YouTube", "Crew")},
	}
	job := newTestAnalysisJob(db)
	job.reader = r
	return db, job, r
}

func queued(t *testing.T, db *sql.DB, status string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sync_items WHERE mapping_id = 'm1' AND status = ?`, status).Scan(&n))
	return n
}

func TestAnalysis_ProbeSkipsUnchangedYouTubePlaylist(t *testing.T) {
	db, job, r := probeFixture(t)
	ctx := context.Background()

	// First run: no cache, a full read.
	_, err := job.analyzeMapping(ctx, loadFullMapping(t, db, "m1"), 0)
	require.NoError(t, err)
	assert.Equal(t, 1, r.fullReads)
	assert.Equal(t, 2, queued(t, db, "pending"), "one song each way")
	m := loadFullMapping(t, db, "m1")
	require.NotNil(t, m.YoutubeItemCount)
	assert.Equal(t, int32(2), *m.YoutubeItemCount)
	assert.Equal(t, int32(2), m.SpotifyTracksCount)
	assert.Equal(t, int32(2), m.YoutubeTracksCount)
	assert.NotNil(t, m.LastFullReadAt)

	// Unchanged: the 1-unit probe is enough.
	_, err = job.analyzeMapping(ctx, m, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, r.probes)
	assert.Equal(t, 1, r.fullReads, "served from the cache")
	assert.Equal(t, 2, queued(t, db, "pending"), "nothing queued twice")

	// The cache survives a restart (new job, empty memory).
	job2 := newTestAnalysisJob(db)
	job2.reader = r
	_, err = job2.analyzeMapping(ctx, loadFullMapping(t, db, "m1"), 0)
	require.NoError(t, err)
	assert.Equal(t, 1, r.fullReads, "read from playlist_cache")

	// The playlist changed: a full read.
	r.youtube = append(r.youtube, yt("v3", "Brand New", "Crew"))
	_, err = job.analyzeMapping(ctx, loadFullMapping(t, db, "m1"), 0)
	require.NoError(t, err)
	assert.Equal(t, 2, r.fullReads)
	assert.Equal(t, 3, queued(t, db, "pending"))

	// A stale read is refreshed even when the count matches (Sync now uses 10 minutes).
	time.Sleep(10 * time.Millisecond)
	_, err = job.analyzeMapping(ctx, loadFullMapping(t, db, "m1"), time.Millisecond)
	require.NoError(t, err)
	assert.Equal(t, 3, r.fullReads)
}

func TestAnalysis_SpotubesOwnInsertsDontForceARead(t *testing.T) {
	db, job, r := probeFixture(t)
	ctx := context.Background()
	_, err := job.analyzeMapping(ctx, loadFullMapping(t, db, "m1"), 0)
	require.NoError(t, err)

	// The executor adds "Only On Spotify" to YouTube and bumps the expected count.
	yte := &cachedYouTube{youtubePlatform: &mockYouTubePlatform{}, cache: job.cache, db: db}
	require.NoError(t, yte.AddVideoToPlaylist(ctx, "yt1", "v-new"))
	r.youtube = append(r.youtube, yt("v-new", "Only On Spotify", "Singer"))

	_, err = job.analyzeMapping(ctx, loadFullMapping(t, db, "m1"), 0)
	require.NoError(t, err)
	assert.Equal(t, 1, r.fullReads, "the count matches what Spotube expects")
}

func TestAnalysis_DurationsOnlyForUnlinkedVideos(t *testing.T) {
	db, job, r := probeFixture(t)
	ctx := context.Background()
	require.NoError(t, (&tracklinks.Store{DB: db}).LinkAuto(ctx, "s1", "v1", "Shared Song", "Band", 1))
	_, err := job.analyzeMapping(ctx, loadFullMapping(t, db, "m1"), 0)
	require.NoError(t, err)
	assert.Equal(t, []string{"v2"}, r.durationFor)
}

func TestAnalysis_BudgetRefusesFullReadButNotProbe(t *testing.T) {
	db, job, r := probeFixture(t)
	job.budget = quota.New(nil, func() int { return 10000 }, func() int { return 10000 - 1 }, nil)

	_, err := job.analyzeMapping(quota.WithClass(context.Background(), quota.Read), loadFullMapping(t, db, "m1"), 0)
	assert.ErrorIs(t, err, quota.ErrBudget)
	assert.Equal(t, 1, r.probes)
	assert.Zero(t, r.fullReads)
	assert.Zero(t, queued(t, db, "pending"))
}

func TestSyncOne_QuotaRefusalRetriesAtReset(t *testing.T) {
	db, job, _ := probeFixture(t)
	_, err := db.Exec(`UPDATE mappings SET interval_minutes = 1440, schedule_start = '03:00' WHERE id = 'm1'`)
	require.NoError(t, err)
	job.budget = quota.New(nil, func() int { return 10000 }, func() int { return 10000 }, nil)

	job.syncOne(quota.WithClass(context.Background(), quota.Read), loadFullMapping(t, db, "m1"), true, 0)
	m := loadFullMapping(t, db, "m1")
	assert.Equal(t, "waiting_quota", *m.LastSyncStatus)
	require.NotNil(t, m.NextRunAt)
	next := time.Unix(int64(*m.NextRunAt), 0)
	assert.False(t, next.After(quota.ResetAt(time.Now())), "tries again at the reset, not a day later")

	var errs int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM activity_logs WHERE level = 'error'`).Scan(&errs))
	assert.Zero(t, errs, "waiting for quota isn't an error")
}

func TestSyncOne_ScheduledRunSetsNextRun(t *testing.T) {
	db, job, _ := probeFixture(t)
	before := time.Now()
	job.syncOne(context.Background(), loadFullMapping(t, db, "m1"), true, 0)
	m := loadFullMapping(t, db, "m1")
	assert.Equal(t, "ok", *m.LastSyncStatus)
	require.NotNil(t, m.NextRunAt)
	next := time.Unix(int64(*m.NextRunAt), 0)
	assert.True(t, next.After(before) && next.Before(before.Add(61*time.Minute)), "next hourly slot, got %s", next)

	// Sync now doesn't move the schedule.
	_, err := db.Exec(`UPDATE mappings SET next_run_at = 42 WHERE id = 'm1'`)
	require.NoError(t, err)
	require.NoError(t, job.SyncMapping(context.Background(), loadFullMapping(t, db, "m1")))
	assert.Equal(t, int32(42), *loadFullMapping(t, db, "m1").NextRunAt)
}

func TestSyncOne_MissingPlaylistStatus(t *testing.T) {
	db, job, r := probeFixture(t)
	r.probeErr = platform.ErrPlaylistNotFound
	job.syncOne(context.Background(), loadFullMapping(t, db, "m1"), true, 0)
	assert.Equal(t, "playlist_missing", *loadFullMapping(t, db, "m1").LastSyncStatus)
}

func TestAnalysis_FullPlaylistStatus(t *testing.T) {
	db, job, r := probeFixture(t)
	r.extraVideos = 4998
	job.syncOne(context.Background(), loadFullMapping(t, db, "m1"), true, 0)
	assert.Equal(t, "playlist_full", *loadFullMapping(t, db, "m1").LastSyncStatus)
}

func TestAnalysis_RequeuesSongThatCameBack(t *testing.T) {
	db, job, r := probeFixture(t)
	ctx := context.Background()
	_, err := job.analyzeMapping(ctx, loadFullMapping(t, db, "m1"), 0)
	require.NoError(t, err)

	// "Only On Spotify" leaves the Spotify playlist: its queued copy is skipped.
	saved := r.spotify
	r.spotify = r.spotify[:1]
	_, err = job.analyzeMapping(ctx, loadFullMapping(t, db, "m1"), 0)
	require.NoError(t, err)
	var status, msg string
	require.NoError(t, db.QueryRow(`SELECT status, error_message FROM sync_items WHERE track_id = 's2'`).Scan(&status, &msg))
	assert.Equal(t, "skipped", status)
	assert.Equal(t, "no longer in source playlist", msg)

	// It comes back: wanted again.
	r.spotify = saved
	_, err = job.analyzeMapping(ctx, loadFullMapping(t, db, "m1"), 0)
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(`SELECT status FROM sync_items WHERE track_id = 's2'`).Scan(&status))
	assert.Equal(t, "pending", status)
}

func TestAnalysis_OnlyLogsRunsThatFoundSomething(t *testing.T) {
	db, job, _ := probeFixture(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_, err := job.analyzeMapping(ctx, loadFullMapping(t, db, "m1"), 0)
		require.NoError(t, err)
	}
	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM activity_logs WHERE job_type = 'analysis'`).Scan(&n))
	assert.Equal(t, 1, n)
}

func TestDailyReadUnits(t *testing.T) {
	db, job, _ := probeFixture(t)
	_, err := db.Exec(`UPDATE mappings SET youtube_tracks_count = 1000 WHERE id = 'm1'`)
	require.NoError(t, err)
	assert.Equal(t, 54, job.DailyReadUnits(), "hourly: 24 probes + one 30-unit read")
}

func TestSyncOne_InterruptedRunIsNotAFailure(t *testing.T) {
	db, job, r := probeFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // the server is stopping
	r.probeErr = context.Canceled
	job.syncOne(ctx, loadFullMapping(t, db, "m1"), true, 0)

	m := loadFullMapping(t, db, "m1")
	assert.Nil(t, m.LastSyncStatus, "status left as it was")
	assert.Nil(t, m.NextRunAt, "still due, so it runs after the restart")
	var errs int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM activity_logs WHERE level = 'error'`).Scan(&errs))
	assert.Zero(t, errs)
}
