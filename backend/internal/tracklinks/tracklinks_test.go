package tracklinks

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/migrate"
	"github.com/manlikeabro/spotube/internal/sqliteconn"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	db, err := sqliteconn.OpenWithPragmas(filepath.Join(t.TempDir(), "links.db"))
	require.NoError(t, err)
	require.NoError(t, migrate.Up(db))
	t.Cleanup(func() { db.Close() })
	return &Store{DB: db}
}

func TestLinkAutoNeverOverridesExistingLinks(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)

	require.NoError(t, s.LinkAuto(ctx, "sp1", "yt1", "Song", "Artist", 0.93))
	// Same Spotify track, different video: ignored.
	require.NoError(t, s.LinkAuto(ctx, "sp1", "yt2", "Song", "Artist", 0.9))
	// Different Spotify track, same video: ignored.
	require.NoError(t, s.LinkAuto(ctx, "sp2", "yt1", "Song", "Artist", 0.9))

	l, err := s.Find(ctx, "spotify", "sp1")
	require.NoError(t, err)
	require.NotNil(t, l)
	assert.Equal(t, "yt1", l.YouTubeID)
	assert.Equal(t, OriginAuto, l.Origin)
	missing, err := s.Find(ctx, "spotify", "sp2")
	require.NoError(t, err)
	assert.Nil(t, missing)
}

func TestSetReplacesConflictingLinksAndMarksMissing(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	require.NoError(t, s.LinkAuto(ctx, "sp1", "yt1", "Song", "Artist", 0.9))
	require.NoError(t, s.LinkAuto(ctx, "sp2", "yt2", "Other", "Artist", 0.9))

	// User says sp1 is really yt2: both old links involving sp1 or yt2 go away.
	l, err := s.Set(ctx, "sp1", "yt2", "Song", "Artist")
	require.NoError(t, err)
	assert.Equal(t, OriginManual, l.Origin)

	links, total, err := s.List(ctx, "", "", 1, 50)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, "yt2", links[0].YouTubeID)

	// Mark a song as not on YouTube.
	_, err = s.Set(ctx, "sp9", "", "Fearless", "Fearless Worship")
	require.NoError(t, err)
	got, err := s.Find(ctx, "spotify", "sp9")
	require.NoError(t, err)
	id, isMissing := got.Counterpart("youtube")
	assert.Empty(t, id)
	assert.True(t, isMissing)

	_, total, err = s.List(ctx, FilterMissingYouTube, "", 1, 50)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	_, total, err = s.List(ctx, "", "fearless", 1, 50)
	require.NoError(t, err)
	assert.Equal(t, 1, total)

	ix, err := s.LoadIndex(ctx)
	require.NoError(t, err)
	got2, ok := ix.For("youtube", "yt2")
	assert.True(t, ok)
	assert.Equal(t, "sp1", got2.SpotifyID)
	var nilIndex *Index
	_, ok = nilIndex.For("spotify", "sp1")
	assert.False(t, ok)

	deleted, err := s.Delete(ctx, got.ID)
	require.NoError(t, err)
	assert.True(t, deleted)
}
