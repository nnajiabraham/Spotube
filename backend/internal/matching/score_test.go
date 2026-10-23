package matching

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withDuration(t Track, ms int) Track {
	t.DurationMs = ms
	return t
}

func TestScoreCandidate_LiveVersionIsPenalized(t *testing.T) {
	source := withDuration(SpotifyTrack("sp", "Joy Is Coming", "Fearless BND"), 245_000)
	studio := withDuration(YouTubeTrack("a", "Joy Is Coming", "Fearless BND - Topic"), 246_000)
	live := withDuration(YouTubeTrack("b", "Fearless BND - Joy Is Coming (Live)", "Fearless BND"), 251_000)

	assert.True(t, ScoreCandidate(source, studio).Total >= MatchThreshold)
	liveScore := ScoreCandidate(source, live)
	assert.Less(t, liveScore.Total, MatchThreshold)
	assert.Contains(t, liveScore.Adjustments, Adjustment{Reason: "only one is live", Delta: -0.2})
}

func TestScoreCandidate_LongVideoIsRejected(t *testing.T) {
	source := withDuration(SpotifyTrack("sp", "Fearless", "Fearless Worship"), 290_000)
	sermon := withDuration(YouTubeTrack("v", "Fearless Worship - Fearless", "Fearless Worship Center"), 95*60_000)

	score := ScoreCandidate(source, sermon)
	assert.Less(t, score.Total, MatchThreshold)
	require.NotNil(t, score.DurationDiffSec)
	assert.Greater(t, *score.DurationDiffSec, 3600)
}

func TestScoreCandidate_SameTitleOtherArtistStaysBelowThreshold(t *testing.T) {
	// Real case: the only "Fearless" results on YouTube were by other artists.
	source := withDuration(SpotifyTrack("sp", "Fearless", "Fearless Worship"), 290_000)
	other := withDuration(YouTubeTrack("v", "Fearless", "JasmineMurrayVEVO"), 288_000)
	assert.Less(t, ScoreCandidate(source, other).Total, MatchThreshold)
}

func TestScoreCandidate_DurationBreaksTieBetweenUploads(t *testing.T) {
	source := withDuration(SpotifyTrack("sp", "Me Enredas Con Tu Pelo", "Fernando Milagros"), 201_000)
	official := withDuration(YouTubeTrack("a", "Fernando Milagros - Me Enredas Con Tu Pelo", "Fernando Milagros"), 202_000)
	extended := withDuration(YouTubeTrack("b", "Fernando Milagros - Me Enredas Con Tu Pelo", "Fernando Milagros"), 420_000)

	assert.Greater(t, ScoreCandidate(source, official).Total, ScoreCandidate(source, extended).Total)
}

func TestScoreCandidate_SpellingVariantStillMatches(t *testing.T) {
	source := SpotifyTrack("sp", "Moving Train (Isọkụsọ)", "Johnny Drille")
	yt := YouTubeTrack("v", "Johnny Drille - Moving Train (Isokuso)", "Johnny Drille")
	assert.GreaterOrEqual(t, ScoreCandidate(source, yt).Total, MatchThreshold)
}

func TestScoreCandidate_TopicChannelBonusAndArtistFromChannel(t *testing.T) {
	source := SpotifyTrack("sp", "Awuke", "Lojay")
	yt := YouTubeTrack("v", "Awuke", "Lojay - Topic")
	score := ScoreCandidate(source, yt)
	assert.True(t, score.ArtistKnown)
	assert.Equal(t, 1.0, score.Artist)
	assert.Contains(t, score.Adjustments, Adjustment{Reason: "YouTube Music upload", Delta: 0.03})
	assert.Equal(t, 1.0, score.Total)
}

func TestFindMatch_RecordsBreakdown(t *testing.T) {
	d := FindMatch(SpotifyTrack("sp", "Pelo Negro", "Fernando Milagros"),
		[]Track{YouTubeTrack("v", "Pelo Negro - Fernando Milagros", "Fernando Milagros")})
	require.NotNil(t, d.Breakdown)
	assert.Equal(t, d.Score, d.Breakdown.Total)
	assert.True(t, d.Matched)
}

func TestTrigramSimilarity(t *testing.T) {
	assert.Equal(t, 1.0, trigramSimilarity("abc", "abc"))
	assert.Greater(t, trigramSimilarity("color caramelo", "colour caramelo"), 0.8)
	assert.Less(t, trigramSimilarity("fearless", "umbrella"), 0.2)
}
