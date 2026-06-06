package matching

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindMatch_PeloNegroCrossPlatform(t *testing.T) {
	source := Track{
		Platform: PlatformSpotify,
		ID:       "spotify-track-1",
		Title:    "Pelo Negro",
		Artist:   "Fernando Milagros",
	}
	dest := []Track{
		YouTubeFromRaw("yt-video-1", "Pelo Negro - Fernando Milagros (Official Video)"),
	}

	decision := FindMatch(source, dest)
	require.True(t, decision.Matched, "expected cross-platform title match without using IDs")
	assert.GreaterOrEqual(t, decision.Score, MatchThreshold)
	assert.Equal(t, "yt-video-1", decision.BestMatch.ID)
}

func TestFindMatch_DoesNotMatchByIDAlone(t *testing.T) {
	source := Track{
		Platform: PlatformSpotify,
		ID:       "shared-id",
		Title:    "Completely Different Song",
		Artist:   "Artist A",
	}
	dest := []Track{
		{
			Platform: PlatformYouTube,
			ID:       "shared-id",
			Title:    "Another Song Entirely",
			Artist:   "Artist B",
		},
	}

	decision := FindMatch(source, dest)
	assert.False(t, decision.Matched, "platform IDs must not imply a cross-platform match")
}

func TestFindMatch_SamePlatformUsesMetadataNotID(t *testing.T) {
	source := Track{
		Platform: PlatformSpotify,
		ID:       "sp-1",
		Title:    "Song A",
		Artist:   "Artist 1",
	}
	dest := []Track{
		{
			Platform: PlatformSpotify,
			ID:       "sp-2",
			Title:    "Song A",
			Artist:   "Artist 1",
		},
	}

	decision := FindMatch(source, dest)
	assert.True(t, decision.Matched)
	assert.Equal(t, "sp-2", decision.BestMatch.ID)
}

func TestFindMatch_NoMatchDifferentSongs(t *testing.T) {
	source := Track{Platform: PlatformSpotify, Title: "Alpha", Artist: "One"}
	dest := []Track{YouTubeFromRaw("v1", "Beta - Two (Official)")}

	decision := FindMatch(source, dest)
	assert.False(t, decision.Matched)
}

func TestYouTubeFromRaw_ParsesArtistFromTitle(t *testing.T) {
	track := YouTubeFromRaw("vid", "Track Name - Singer Name | Album")
	assert.Equal(t, "Track Name", track.Title)
	assert.Equal(t, "Singer Name", track.Artist)
}

func TestScorePair_HigherForCloserTitles(t *testing.T) {
	source := Track{Title: "Pelo Negro", Artist: "Fernando Milagros"}
	close := Track{Title: "Pelo Negro", Artist: "Fernando Milagros"}
	far := Track{Title: "Unrelated", Artist: "Someone Else"}

	assert.Greater(t, ScorePair(source, close), ScorePair(source, far))
}

func TestFindMatch_SongAFromAnalysisFixture(t *testing.T) {
	youtube := []Track{YouTubeFromRaw("youtube1", "Song A")}
	spotify := SpotifyTrack("spotify1", "Song A", "Artist 1")

	assert.True(t, FindMatch(spotify, youtube).Matched)
	assert.True(t, FindMatch(YouTubeFromRaw("youtube1", "Song A"), []Track{spotify}).Matched)
}

func TestBriefAnalysisLine_Queued(t *testing.T) {
	line := BriefAnalysisLine(Decision{
		Source:    Track{Title: "Pelo Negro", Artist: "Fernando Milagros"},
		Matched:   false,
		Score:     0.2,
		BestMatch: &Track{Title: "Other", Artist: "X"},
	}, "youtube", true)
	assert.Contains(t, line, "queued add youtube")
	assert.Contains(t, line, "Pelo Negro")
}
