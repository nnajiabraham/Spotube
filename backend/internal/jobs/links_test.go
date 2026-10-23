package jobs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/matching"
	"github.com/manlikeabro/spotube/internal/tracklinks"
)

func TestExecuteOne_SuccessfulAddRemembersLink(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()
	insertExecutorFixtures(t, db, "m", "item", "add", "youtube", "pending")

	yt := &mockYouTubePlatform{searchHits: []platformSearchHit{{ID: "yt-new", Title: "Song A", Artist: "Artist A"}}}
	e := newTestExecutor(t, db, &mockSpotifyPlatform{}, yt)

	item, err := e.ExecuteOne(context.Background(), "item")
	require.NoError(t, err)
	assert.Equal(t, "done", item.Status)

	link, err := (&tracklinks.Store{DB: db}).Find(context.Background(), "spotify", "src-track-1")
	require.NoError(t, err)
	require.NotNil(t, link)
	assert.Equal(t, "yt-new", link.YouTubeID)
	assert.Equal(t, tracklinks.OriginAuto, link.Origin)
}

func TestExecuteOne_UsesLinkWithoutSearching(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()
	insertExecutorFixtures(t, db, "m", "item", "add", "youtube", "review")
	_, err := db.Exec(`UPDATE sync_items SET status = 'pending' WHERE id = 'item'`)
	require.NoError(t, err)
	_, err = (&tracklinks.Store{DB: db}).Set(context.Background(), "src-track-1", "yt-chosen", "Song A", "Artist A")
	require.NoError(t, err)

	// Search would fail; the link must be used instead.
	yt := &mockYouTubePlatform{searchErr: assert.AnError}
	e := newTestExecutor(t, db, &mockSpotifyPlatform{}, yt)

	item, err := e.ExecuteOne(context.Background(), "item")
	require.NoError(t, err)
	assert.Equal(t, "done", item.Status)
	assert.Equal(t, []string{"yt-chosen"}, yt.addCalls)
}

func TestExecuteOne_NotAvailableSkipsWithoutAPICalls(t *testing.T) {
	db, cleanup := setupExecutorTestDB(t)
	defer cleanup()
	insertExecutorFixtures(t, db, "m", "item", "add", "youtube", "pending")
	_, err := (&tracklinks.Store{DB: db}).Set(context.Background(), "src-track-1", "", "Song A", "Artist A")
	require.NoError(t, err)

	yt := &mockYouTubePlatform{searchErr: assert.AnError}
	e := newTestExecutor(t, db, &mockSpotifyPlatform{}, yt)

	item, err := e.ExecuteOne(context.Background(), "item")
	require.ErrorIs(t, err, ErrNotAvailable)
	assert.Equal(t, "skipped", item.Status)
	assert.Equal(t, "not on YouTube", stringFromPtr(item.ErrorMessage))
	assert.Empty(t, yt.addCalls)
}

// After copying Spotify track S to YouTube video Y, analysis must treat Y as present in
// Spotify even when Y's title would not fuzzy-match S, so Y is never copied back.
func TestGenerateSyncItems_LinkedPairIsNotCopiedBack(t *testing.T) {
	db, cleanup := setupAnalysisTestDB(t)
	defer cleanup()
	ctx := context.Background()
	store := &tracklinks.Store{DB: db}
	require.NoError(t, store.LinkAuto(ctx, "sp-s", "yt-y", "Canción", "Artista", 0.9))
	_, err := store.Set(ctx, "sp-gone", "", "Not On YouTube", "Someone")
	require.NoError(t, err)
	ix, err := store.LoadIndex(ctx)
	require.NoError(t, err)

	mapping := testMapping("m")
	spotify := []TrackInfo{
		{ID: "sp-s", Title: "Canción", Artist: "Artista"},
		{ID: "sp-gone", Title: "Not On YouTube", Artist: "Someone"},
	}
	youtube := []TrackInfo{{ID: "yt-y", Title: "Totally different upload title", RawTitle: "Totally different upload title", Channel: "Some Channel"}}

	job := newTestAnalysisJob(db)
	result := job.generateSyncItems(mapping, spotify, youtube, ix)
	assert.Empty(t, result.items, "linked and unavailable tracks must not be queued")
	assert.Equal(t, 3, result.matchedSkipped, "S→Y, Y→S and the unavailable song")
	assert.True(t, result.unavailable["youtube|sp-gone"])
}

func TestPickSearchCandidate_RecordsBreakdown(t *testing.T) {
	source := matching.SpotifyTrack("sp", "Song A", "Artist A")
	_, candidates, err := pickSearchCandidate(source, []platformSearchHit{{ID: "v", Title: "Song A", Artist: "Artist A", DurationMs: 1000}}, matching.PlatformYouTube)
	require.NoError(t, err)
	require.Len(t, candidates, 1)
	require.NotNil(t, candidates[0].Breakdown)
	assert.Equal(t, candidates[0].Score, candidates[0].Breakdown.Total)
}

func testMapping(id string) model.Mappings {
	return model.Mappings{ID: &id, SyncTracks: 1, SpotifyPlaylistID: "sp-pl", YoutubePlaylistID: "yt-pl"}
}
