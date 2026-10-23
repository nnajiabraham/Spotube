package backups

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/matching"
	"github.com/manlikeabro/spotube/internal/migrate"
	"github.com/manlikeabro/spotube/internal/sqliteconn"
	"github.com/manlikeabro/spotube/internal/tracklinks"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }
func (c *fakeClock) advance(d time.Duration) {
	c.t = c.t.Add(d)
}

func newStore(t *testing.T) (*Store, *fakeClock) {
	t.Helper()
	db, err := sqliteconn.OpenWithPragmas(filepath.Join(t.TempDir(), "backups.db"))
	require.NoError(t, err)
	require.NoError(t, migrate.Up(db))
	t.Cleanup(func() { db.Close() })
	clock := &fakeClock{t: time.Unix(1_800_000_000, 0)}
	return &Store{DB: db, now: clock.now}, clock
}

func sp(ids ...string) []matching.Track {
	out := make([]matching.Track, 0, len(ids))
	for _, id := range ids {
		out = append(out, matching.SpotifyTrack(id, "Song "+id, "Artist"))
	}
	return out
}

// fakeService records reads and writes in memory.
type fakeService struct {
	playlists map[string][]matching.Track
	names     map[string]string
	created   []string
	added     map[string][]string
	readErr   error
	failAfter int // AddTracks fails after this many adds (0: never)
}

func newFake() *fakeService {
	return &fakeService{playlists: map[string][]matching.Track{}, names: map[string]string{}, added: map[string][]string{}}
}

func (f *fakeService) PlaylistTracks(_ context.Context, service, id string) (string, []matching.Track, error) {
	if f.readErr != nil {
		return "", nil, f.readErr
	}
	return f.names[service+"|"+id], f.playlists[service+"|"+id], nil
}

func (f *fakeService) CreatePlaylist(_ context.Context, service, name, _ string) (string, error) {
	id := "new-" + name
	f.created = append(f.created, service+"|"+name)
	return id, nil
}

func (f *fakeService) AddTracks(_ context.Context, service, playlistID string, ids []string) (int, error) {
	for i, id := range ids {
		if f.failAfter > 0 && len(f.added[service+"|"+playlistID]) >= f.failAfter {
			return i, errors.New("quotaExceeded")
		}
		f.added[service+"|"+playlistID] = append(f.added[service+"|"+playlistID], id)
	}
	return len(ids), nil
}

func TestRecord_StoresOnlyChanges(t *testing.T) {
	ctx := context.Background()
	s, clock := newStore(t)
	b, err := s.Create(ctx, "spotify", "pl1", "Mix", 0, 0)
	require.NoError(t, err)
	assert.Equal(t, DefaultIntervalHours, b.IntervalHours)
	assert.Equal(t, DefaultKeepSnapshots, b.KeepSnapshots)

	first, created, err := s.Record(ctx, *b, "Mix", sp("a", "b"), SourceInitial)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Nil(t, first.Added, "the first snapshot has nothing to compare with")

	clock.advance(time.Hour)
	same, created, err := s.Record(ctx, *b, "Mix", sp("a", "b"), SourceScheduled)
	require.NoError(t, err)
	assert.False(t, created, "an unchanged playlist is not stored again")
	assert.Equal(t, first.ID, same.ID)
	assert.Equal(t, clock.t.Unix(), same.ConfirmedAt)
	assert.Equal(t, first.Created, same.Created)

	clock.advance(time.Hour)
	next, created, err := s.Record(ctx, *b, "Mix", sp("a", "c", "d"), SourceScheduled)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, 2, *next.Added)
	assert.Equal(t, 1, *next.Removed)

	// Reordering is a change too.
	clock.advance(time.Hour)
	_, created, err = s.Record(ctx, *b, "Mix", sp("d", "c", "a"), SourceScheduled)
	require.NoError(t, err)
	assert.True(t, created)

	got, err := s.Get(ctx, b.ID)
	require.NoError(t, err)
	assert.Equal(t, 3, got.SnapshotCount)
	require.NotNil(t, got.LastCheckedAt)
	assert.Equal(t, clock.t.Unix(), *got.LastCheckedAt)
}

func TestRecord_PrunesToKeepCount(t *testing.T) {
	ctx := context.Background()
	s, clock := newStore(t)
	b, err := s.Create(ctx, "spotify", "pl1", "Mix", 24, 2)
	require.NoError(t, err)
	for _, ids := range [][]string{{"a"}, {"a", "b"}, {"a", "b", "c"}} {
		clock.advance(time.Minute)
		_, _, err := s.Record(ctx, *b, "Mix", sp(ids...), SourceScheduled)
		require.NoError(t, err)
	}
	snaps, err := s.Snapshots(ctx, b.ID)
	require.NoError(t, err)
	require.Len(t, snaps, 2)
	assert.Equal(t, 3, snaps[0].TrackCount, "newest first")
	assert.Equal(t, 2, snaps[1].TrackCount)
}

func TestSnapshot_IncludesCounterpartsAndDiff(t *testing.T) {
	ctx := context.Background()
	s, clock := newStore(t)
	links := &tracklinks.Store{DB: s.DB}
	require.NoError(t, links.LinkAuto(ctx, "a", "yt-a", "Song a", "Artist", 0.95))
	_, err := links.Set(ctx, "b", "", "Song b", "Artist") // not on YouTube
	require.NoError(t, err)

	b, err := s.Create(ctx, "spotify", "pl1", "Mix", 0, 0)
	require.NoError(t, err)
	_, _, err = s.Record(ctx, *b, "Mix", sp("a", "b"), SourceInitial)
	require.NoError(t, err)
	clock.advance(time.Minute)
	sum, _, err := s.Record(ctx, *b, "Mix", sp("a", "c"), SourceManual)
	require.NoError(t, err)

	snap, err := s.Snapshot(ctx, sum.ID)
	require.NoError(t, err)
	require.Len(t, snap.Tracks, 2)
	assert.Equal(t, "a", snap.Tracks[0].SpotifyID)
	assert.Equal(t, "yt-a", snap.Tracks[0].YouTubeID)
	assert.Equal(t, "", snap.Tracks[1].YouTubeID)
	assert.True(t, snap.HasPrevious)
	require.Len(t, snap.AddedTracks, 1)
	assert.Equal(t, "c", snap.AddedTracks[0].ID)
	require.Len(t, snap.RemovedTracks, 1)
	assert.Equal(t, "b", snap.RemovedTracks[0].ID)
}

func TestCreate_RejectsDuplicates(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	_, err := s.Create(ctx, "youtube", "PL1", "", 0, 0)
	require.NoError(t, err)
	_, err = s.Create(ctx, "youtube", "PL1", "", 0, 0)
	assert.ErrorIs(t, err, ErrExists)
	_, err = s.Create(ctx, "deezer", "x", "", 0, 0)
	assert.Error(t, err)
}

func TestDue_RespectsIntervalAndEnabled(t *testing.T) {
	ctx := context.Background()
	s, clock := newStore(t)
	a, err := s.Create(ctx, "spotify", "a", "A", 1, 0)
	require.NoError(t, err)
	off, err := s.Create(ctx, "spotify", "off", "Off", 1, 0)
	require.NoError(t, err)
	disabled := false
	_, err = s.Update(ctx, off.ID, &disabled, nil, nil)
	require.NoError(t, err)

	due, err := s.Due(ctx)
	require.NoError(t, err)
	require.Len(t, due, 1, "never checked: due straight away")

	_, _, err = s.Record(ctx, *a, "A", sp("x"), SourceInitial)
	require.NoError(t, err)
	due, err = s.Due(ctx)
	require.NoError(t, err)
	assert.Empty(t, due)

	clock.advance(61 * time.Minute)
	due, err = s.Due(ctx)
	require.NoError(t, err)
	assert.Len(t, due, 1)
}

func TestObserve_OnlyBackedUpPlaylists(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	created, err := s.Observe(ctx, "spotify", "not-backed-up", "X", sp("a"))
	require.NoError(t, err)
	assert.False(t, created)

	b, err := s.Create(ctx, "spotify", "pl1", "Mix", 0, 0)
	require.NoError(t, err)
	created, err = s.Observe(ctx, "spotify", "pl1", "Mix", sp("a"))
	require.NoError(t, err)
	assert.True(t, created)
	snaps, err := s.Snapshots(ctx, b.ID)
	require.NoError(t, err)
	require.Len(t, snaps, 1)
	assert.Equal(t, SourceSync, snaps[0].Source)
}

func TestBackUp_RecordsReadErrors(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	fake := newFake()
	fake.readErr = errors.New("playlist not found")
	svc := &Service{Store: s, Reader: fake, Writer: fake}
	b, err := s.Create(ctx, "youtube", "PL1", "Mix", 0, 0)
	require.NoError(t, err)

	_, _, err = svc.BackUp(ctx, *b, SourceManual)
	require.Error(t, err)
	got, err := s.Get(ctx, b.ID)
	require.NoError(t, err)
	assert.Equal(t, "playlist not found", got.LastError)
	assert.Nil(t, got.LastCheckedAt, "a failed check stays due")

	fake.readErr = nil
	fake.playlists["youtube|PL1"] = []matching.Track{matching.YouTubeTrack("v1", "Artist - Song", "Artist - Topic")}
	fake.names["youtube|PL1"] = "Renamed"
	_, _, err = svc.BackUp(ctx, *b, SourceManual)
	require.NoError(t, err)
	got, err = s.Get(ctx, b.ID)
	require.NoError(t, err)
	assert.Empty(t, got.LastError)
	assert.Equal(t, "Renamed", got.PlaylistName)
}

func seedSnapshot(t *testing.T, s *Store, service string, tracks []matching.Track) (*Backup, *SnapshotSummary) {
	t.Helper()
	ctx := context.Background()
	b, err := s.Create(ctx, service, "orig", "Mix", 0, 0)
	require.NoError(t, err)
	sum, _, err := s.Record(ctx, *b, "Mix", tracks, SourceInitial)
	require.NoError(t, err)
	return b, sum
}

func TestRestore_NewPlaylistKeepsOrder(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	fake := newFake()
	svc := &Service{Store: s, Reader: fake, Writer: fake}
	_, sum := seedSnapshot(t, s, "spotify", sp("c", "a", "b"))

	res, err := svc.Restore(ctx, sum.ID, RestoreRequest{Mode: RestoreNew})
	require.NoError(t, err)
	assert.Equal(t, 3, res.Added)
	assert.Contains(t, res.PlaylistName, "Mix (restored ")
	assert.Equal(t, []string{"c", "a", "b"}, fake.added["spotify|"+res.PlaylistID])
	assert.Equal(t, "https://open.spotify.com/playlist/"+res.PlaylistID, res.PlaylistURL)
}

func TestRestore_MissingAddsOnlyWhatIsGone(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	fake := newFake()
	svc := &Service{Store: s, Reader: fake, Writer: fake}
	_, sum := seedSnapshot(t, s, "spotify", sp("a", "b", "c"))
	fake.playlists["spotify|orig"] = sp("b", "z") // a and c were removed, z added

	res, err := svc.Restore(ctx, sum.ID, RestoreRequest{Mode: RestoreMissing})
	require.NoError(t, err)
	assert.Equal(t, 2, res.Added)
	assert.Equal(t, 1, res.AlreadyPresent)
	assert.Equal(t, []string{"a", "c"}, fake.added["spotify|orig"])
	assert.Empty(t, fake.created, "missing mode never creates a playlist")
}

func TestRestore_OtherServiceUsesLinkedIDs(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	links := &tracklinks.Store{DB: s.DB}
	require.NoError(t, links.LinkAuto(ctx, "a", "yt-a", "", "", 0.9))
	fake := newFake()
	svc := &Service{Store: s, Reader: fake, Writer: fake}
	_, sum := seedSnapshot(t, s, "spotify", sp("a", "b"))

	res, err := svc.Restore(ctx, sum.ID, RestoreRequest{Mode: RestoreNew, Service: "youtube"})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Added)
	require.Len(t, res.NoCounterpart, 1)
	assert.Equal(t, "b", res.NoCounterpart[0].ID)
	assert.Equal(t, []string{"yt-a"}, fake.added["youtube|"+res.PlaylistID])

	_, err = svc.Restore(ctx, sum.ID, RestoreRequest{Mode: RestoreMissing, Service: "youtube"})
	assert.ErrorIs(t, err, ErrInvalid)
}

func TestRestore_RefusesWhenYouTubeQuotaIsShort(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	fake := newFake()
	svc := &Service{Store: s, Reader: fake, Writer: fake, QuotaLeft: func() int { return 120 }}
	tracks := []matching.Track{
		matching.YouTubeTrack("v1", "A - One", "A"),
		matching.YouTubeTrack("v2", "A - Two", "A"),
	}
	_, sum := seedSnapshot(t, s, "youtube", tracks)

	_, err := svc.Restore(ctx, sum.ID, RestoreRequest{Mode: RestoreNew})
	var quota *ErrQuota
	require.ErrorAs(t, err, &quota)
	assert.Equal(t, 150, quota.Needed)
	assert.Empty(t, fake.created, "nothing is created when the quota check fails")
	assert.Equal(t, 150, EstimateRestoreCost("youtube", RestoreNew, 2))
	assert.Equal(t, 0, EstimateRestoreCost("spotify", RestoreNew, 2))
}

func TestRestore_ReportsPartialFailure(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	fake := newFake()
	fake.failAfter = 1
	svc := &Service{Store: s, Reader: fake, Writer: fake}
	_, sum := seedSnapshot(t, s, "spotify", sp("a", "b"))

	res, err := svc.Restore(ctx, sum.ID, RestoreRequest{Mode: RestoreNew})
	require.NoError(t, err)
	assert.Equal(t, 1, res.Added)
	assert.Equal(t, "quotaExceeded", res.Error)
}

func TestExportCSV(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	b, sum := seedSnapshot(t, s, "spotify", sp("a"))
	snap, err := s.Snapshot(ctx, sum.ID)
	require.NoError(t, err)
	body, err := NewExport(*b, *snap).CSV()
	require.NoError(t, err)
	assert.Equal(t, "position,title,artist,duration_ms,spotify_id,youtube_id,youtube_title,youtube_channel\n1,Song a,Artist,0,a,,,\n", string(body))
}
