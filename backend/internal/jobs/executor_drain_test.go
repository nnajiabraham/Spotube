package jobs

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/googleapi"

	"github.com/manlikeabro/spotube/internal/quota"
	"github.com/manlikeabro/spotube/internal/tracklinks"
)

// insertDrainMapping adds a mapping whose playlists are sp-<id> and yt-<id>.
func insertDrainMapping(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, youtube_item_count, created, updated) VALUES (?, ?, ?, 1, 1, 60, 0, 10, ?, ?)`,
		id, "sp-"+id, "yt-"+id, now, now)
	require.NoError(t, err)
}

func itemState(t *testing.T, db *sql.DB, id string) (status string, attempts int, msg string) {
	t.Helper()
	var m sql.NullString
	require.NoError(t, db.QueryRow(`SELECT status, attempt_count, error_message FROM sync_items WHERE id = ?`, id).Scan(&status, &attempts, &m))
	return status, attempts, m.String
}

func mappingStatus(t *testing.T, db *sql.DB, id string) string {
	t.Helper()
	var s sql.NullString
	require.NoError(t, db.QueryRow(`SELECT last_sync_status FROM mappings WHERE id = ?`, id).Scan(&s))
	return s.String
}

func goErr(code int, reason string) error {
	return &googleapi.Error{Code: code, Errors: []googleapi.ErrorItem{{Reason: reason}}}
}

func TestDrain_QuotaBlocksYouTubeWhileSpotifyCarriesOn(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()
	insertDrainMapping(t, db, "m1")
	insertPendingItem(t, db, "m1", "yt1", "youtube", "Song A", "Artist A")
	insertPendingItem(t, db, "m1", "yt2", "youtube", "Song B", "Artist B")
	insertPendingItem(t, db, "m1", "sp1", "spotify", "Song C", "Artist C")
	insertPendingItem(t, db, "m1", "sp2", "spotify", "Song D", "Artist D")

	sp := &mockSpotifyPlatform{searchHits: []platformSearchHit{{ID: "sp-hit", Title: "Song C", Artist: "Artist C"}}}
	yt := &mockYouTubePlatform{searchErr: goErr(403, "quotaExceeded")}
	e := newTestExecutor(t, db, sp, yt)

	result, err := e.Drain(context.Background(), 10, nil)
	require.Error(t, err, "the quota stop is reported")
	assert.Equal(t, 1, result.Deferred, "only the first YouTube item is tried")
	assert.Equal(t, 2, result.Attempted, "both Spotify items run")
	assert.Len(t, sp.addCalls, 1, "the second Spotify item is the same song: skipped as a duplicate")

	status, attempts, msg := itemState(t, db, "yt1")
	assert.Equal(t, "pending", status)
	assert.Zero(t, attempts)
	assert.Contains(t, msg, "Waiting for YouTube quota")
	status, attempts, msg = itemState(t, db, "yt2")
	assert.Equal(t, "pending", status)
	assert.Zero(t, attempts)
	assert.Empty(t, msg, "never tried")
}

func TestDrain_BudgetRefusesBeforeSpending(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()
	insertDrainMapping(t, db, "m1")
	insertPendingItem(t, db, "m1", "yt1", "youtube", "Song A", "Artist A")
	insertPendingItem(t, db, "m1", "sp1", "spotify", "Song C", "Artist C")

	limit := 100 // a search plus insert needs 151
	yt := &mockYouTubePlatform{searchHits: []platformSearchHit{{ID: "v1", Title: "Song A", Artist: "Artist A"}}}
	sp := &mockSpotifyPlatform{searchHits: []platformSearchHit{{ID: "s1", Title: "Song C", Artist: "Artist C"}}}
	e := newTestExecutor(t, db, sp, yt)
	e.budget = quota.New(nil, func() int { return limit }, nil, nil)

	result, err := e.Drain(quota.WithClass(context.Background(), quota.Drain), 10, nil)
	assert.ErrorIs(t, err, quota.ErrBudget)
	assert.Equal(t, 1, result.Deferred)
	assert.Equal(t, 1, result.Done)
	assert.Empty(t, yt.addCalls)
	status, attempts, _ := itemState(t, db, "yt1")
	assert.Equal(t, "pending", status)
	assert.Zero(t, attempts)

	// With room for an insert but not a search, a song with a remembered video still goes.
	limit = 60
	require.NoError(t, (&tracklinks.Store{DB: db}).LinkAuto(context.Background(), "src-yt1", "v-linked", "Song A", "Artist A", 1))
	result, err = e.Drain(quota.WithClass(context.Background(), quota.Drain), 10, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Done)
	assert.Equal(t, []string{"v-linked"}, yt.addCalls)
}

func TestDrain_FullPlaylistPausesYouTubeAddsOnly(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()
	insertDrainMapping(t, db, "m1")
	insertPendingItem(t, db, "m1", "yt1", "youtube", "Song A", "Artist A")
	insertPendingItem(t, db, "m1", "yt2", "youtube", "Song B", "Artist B")
	insertPendingItem(t, db, "m1", "sp1", "spotify", "Song C", "Artist C")

	yt := &mockYouTubePlatform{
		searchHits: []platformSearchHit{{ID: "v1", Title: "Song A", Artist: "Artist A"}},
		addErr:     goErr(403, "playlistContainsMaximumNumberOfVideos"),
	}
	sp := &mockSpotifyPlatform{searchHits: []platformSearchHit{{ID: "s1", Title: "Song C", Artist: "Artist C"}}}
	e := newTestExecutor(t, db, sp, yt)

	result, err := e.Drain(context.Background(), 10, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Skipped)
	assert.Equal(t, 1, result.Done, "the Spotify side still syncs")
	assert.Len(t, yt.addCalls, 1, "no more inserts into a full playlist")
	assert.Equal(t, "playlist_full", mappingStatus(t, db, "m1"))
	status, _, msg := itemState(t, db, "yt1")
	assert.Equal(t, "skipped", status)
	assert.Contains(t, msg, "full")

	items, err := e.applicableItems(context.Background(), "m1")
	require.NoError(t, err)
	assert.Empty(t, items, "YouTube adds wait until the playlist has room")
}

func TestDrain_MissingPlaylistPausesTheMapping(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()
	insertDrainMapping(t, db, "m1")
	insertDrainMapping(t, db, "m2")
	insertPendingItem(t, db, "m1", "a1", "youtube", "Song A", "Artist A")
	insertPendingItem(t, db, "m1", "a2", "spotify", "Song B", "Artist B")
	insertPendingItem(t, db, "m2", "b1", "spotify", "Song C", "Artist C")

	yt := &mockYouTubePlatform{playlistErr: goErr(404, "playlistNotFound")}
	sp := &mockSpotifyPlatform{searchHits: []platformSearchHit{{ID: "s1", Title: "Song C", Artist: "Artist C"}}}
	e := newTestExecutor(t, db, sp, yt)

	result, err := e.Drain(context.Background(), 10, nil)
	require.NoError(t, err, "a missing playlist doesn't stop other mappings")
	assert.Equal(t, 1, result.Deferred)
	assert.Equal(t, 1, result.PerMapping["m2"].Done)
	assert.Nil(t, result.PerMapping["m1"].PerMapping)
	assert.Zero(t, result.PerMapping["m1"].Attempted, "the rest of m1 waits")
	assert.Equal(t, "playlist_missing", mappingStatus(t, db, "m1"))
	status, attempts, _ := itemState(t, db, "a1")
	assert.Equal(t, "pending", status)
	assert.Zero(t, attempts)

	items, err := e.applicableItems(context.Background(), "")
	require.NoError(t, err)
	assert.Empty(t, items, "nothing left but the paused mapping's items")
}

func TestExecuteOne_RateLimitAndInterruptionAreDeferred(t *testing.T) {
	for name, cause := range map[string]error{
		"youtube rate limit": goErr(403, "rateLimitExceeded"),
		"http 429":           &googleapi.Error{Code: 429},
		"cancelled":          context.Canceled,
	} {
		t.Run(name, func(t *testing.T) {
			db, cleanup := setupExecutorTestDB(t)
			defer cleanup()
			insertDrainMapping(t, db, "m1")
			insertPendingItem(t, db, "m1", "i1", "youtube", "Song A", "Artist A")
			e := newTestExecutor(t, db, &mockSpotifyPlatform{}, &mockYouTubePlatform{searchErr: cause})

			_, err := e.ExecuteOne(context.Background(), "i1")
			var d *DeferredError
			require.ErrorAs(t, err, &d)
			assert.True(t, d.BlocksService)
			status, attempts, _ := itemState(t, db, "i1")
			assert.Equal(t, "pending", status)
			assert.Zero(t, attempts)
		})
	}
}

func TestExecuteOne_RemembersMatchBeforeInsert(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()
	insertDrainMapping(t, db, "m1")
	insertPendingItem(t, db, "m1", "i1", "youtube", "Song A", "Artist A")
	yt := &mockYouTubePlatform{
		searchHits: []platformSearchHit{{ID: "v1", Title: "Song A", Artist: "Artist A"}},
		addErr:     goErr(403, "quotaExceeded"),
	}
	e := newTestExecutor(t, db, &mockSpotifyPlatform{}, yt)
	_, err := e.ExecuteOne(context.Background(), "i1")
	require.Error(t, err)

	link, err := (&tracklinks.Store{DB: db}).Find(context.Background(), "spotify", "src-i1")
	require.NoError(t, err)
	require.NotNil(t, link, "the search result is kept for the retry")
	target, _ := link.Counterpart("youtube")
	assert.Equal(t, "v1", target)

	// The retry inserts straight away: no second search.
	yt.addErr, yt.searchErr = nil, goErr(500, "shouldNotSearch")
	item, err := e.ExecuteOne(context.Background(), "i1")
	require.NoError(t, err)
	assert.Equal(t, "done", item.Status)
}

func TestExecutor_YouTubeInsertsBumpExpectedItemCount(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()
	insertDrainMapping(t, db, "m1")
	insertPendingItem(t, db, "m1", "i1", "youtube", "Song A", "Artist A")
	yt := &mockYouTubePlatform{searchHits: []platformSearchHit{{ID: "v1", Title: "Song A", Artist: "Artist A"}}}
	e := newTestExecutor(t, db, &mockSpotifyPlatform{}, yt)

	_, err := e.ExecutePending(context.Background(), "m1", 10)
	require.NoError(t, err)
	var n int
	require.NoError(t, db.QueryRow(`SELECT youtube_item_count FROM mappings WHERE id = 'm1'`).Scan(&n))
	assert.Equal(t, 11, n, "Spotube's own insert isn't mistaken for a change by the next probe")
}

func TestApplicableItems_RetryBackoff(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()
	insertDrainMapping(t, db, "m1")
	now := time.Now().Unix()
	for _, c := range []struct {
		id       string
		attempts int
		ago      int64
	}{
		{"fresh-1", 1, 60},    // 15 min backoff: not yet
		{"due-1", 1, 901},     // past 15 min
		{"fresh-2", 2, 3000},  // 1 h backoff: not yet
		{"due-2", 2, 3601},    // past 1 h
		{"capped", 3, 100000}, // out of attempts
		{"due-0", 0, 0},       // failed without counting (shouldn't happen, but harmless)
	} {
		_, err := db.Exec(`INSERT INTO sync_items (id, mapping_id, operation, service, track_id, track_title, status, attempt_count, last_attempt_at, created, updated) VALUES (?, 'm1', 'add', 'youtube', ?, 'S', 'error', ?, ?, ?, ?)`,
			c.id, "t-"+c.id, c.attempts, now-c.ago, now, now)
		require.NoError(t, err)
	}
	e := newTestExecutor(t, db, &mockSpotifyPlatform{}, &mockYouTubePlatform{})
	items, err := e.applicableItems(context.Background(), "")
	require.NoError(t, err)
	var ids []string
	for _, q := range items {
		ids = append(ids, q.id)
	}
	assert.ElementsMatch(t, []string{"due-1", "due-2", "due-0"}, ids)
}

func TestOrderForDrain(t *testing.T) {
	q := func(id, mapping, op string, linked bool) queuedItem {
		return queuedItem{id: id, mappingID: mapping, operation: op, linked: linked}
	}
	items := []queuedItem{
		q("a1", "A", "add", false), q("a2", "A", "add", false), q("a3", "A", "add", false),
		q("b1", "B", "add", false),
		q("a-linked", "A", "add", true),
		q("b-rename", "B", "rename", false),
		q("c1", "C", "add", false),
	}
	var got []string
	for _, it := range orderForDrain(items) {
		got = append(got, it.id)
	}
	assert.Equal(t, []string{"b-rename", "a-linked", "a1", "b1", "c1", "a2", "a3"}, got)
}

func TestDrain_SkipsBusyMappingsAndRespectsLimit(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()
	insertDrainMapping(t, db, "busy")
	insertDrainMapping(t, db, "free")
	insertPendingItem(t, db, "busy", "x1", "spotify", "Song A", "Artist A")
	for _, id := range []string{"y1", "y2", "y3"} {
		insertPendingItem(t, db, "free", id, "spotify", "Song "+id, "Artist")
	}
	sp := &mockSpotifyPlatform{searchHits: []platformSearchHit{{ID: "s", Title: "Song", Artist: "Artist"}}}
	e := newTestExecutor(t, db, sp, &mockYouTubePlatform{})
	lock := func(id string) (func(), bool) { return func() {}, id != "busy" }

	result, err := e.Drain(context.Background(), 2, lock)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Attempted)
	status, _, _ := itemState(t, db, "x1")
	assert.Equal(t, "pending", status, "left for the next pass")
}

func TestResetInterrupted(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()
	insertDrainMapping(t, db, "m1")
	insertPendingItem(t, db, "m1", "r1", "youtube", "S", "A")
	insertPendingItem(t, db, "m1", "r2", "youtube", "S2", "A")
	_, err := db.Exec(`UPDATE sync_items SET status = 'running' WHERE id = 'r1'`)
	require.NoError(t, err)

	e := newTestExecutor(t, db, &mockSpotifyPlatform{}, &mockYouTubePlatform{})
	n, err := e.ResetInterrupted(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	status, attempts, msg := itemState(t, db, "r1")
	assert.Equal(t, "pending", status)
	assert.Zero(t, attempts)
	assert.Contains(t, msg, "Interrupted")
}
