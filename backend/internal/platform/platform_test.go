package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseISODuration(t *testing.T) {
	assert.Equal(t, 225_000, ParseISODuration("PT3M45S"))
	assert.Equal(t, 3_723_000, ParseISODuration("PT1H2M3S"))
	assert.Equal(t, 30_000, ParseISODuration("PT30S"))
	assert.Equal(t, 0, ParseISODuration(""))
	assert.Equal(t, 86_400_000+60_000, ParseISODuration("P1DT1M"))
}

func TestParseSpotifyTrackRef(t *testing.T) {
	for _, in := range []string{
		"https://open.spotify.com/track/2Ohl8hFSvgNm9wxO7m4TlL?si=abc",
		"https://open.spotify.com/intl-de/track/2Ohl8hFSvgNm9wxO7m4TlL",
		"spotify:track:2Ohl8hFSvgNm9wxO7m4TlL",
		" 2Ohl8hFSvgNm9wxO7m4TlL ",
	} {
		id, ok := ParseSpotifyTrackRef(in)
		assert.True(t, ok, in)
		assert.Equal(t, "2Ohl8hFSvgNm9wxO7m4TlL", id, in)
	}
	_, ok := ParseSpotifyTrackRef("https://open.spotify.com/playlist/37i9dQZF1DXcBWIGoYBM5M")
	assert.False(t, ok)
}

func TestParseYouTubeVideoRef(t *testing.T) {
	for _, in := range []string{
		"https://www.youtube.com/watch?v=vtQ0PWOYWMo&list=PL123",
		"https://music.youtube.com/watch?v=vtQ0PWOYWMo",
		"https://youtu.be/vtQ0PWOYWMo?t=10",
		"https://www.youtube.com/shorts/vtQ0PWOYWMo",
		"vtQ0PWOYWMo",
	} {
		id, ok := ParseYouTubeVideoRef(in)
		assert.True(t, ok, in)
		assert.Equal(t, "vtQ0PWOYWMo", id, in)
	}
	_, ok := ParseYouTubeVideoRef("https://www.youtube.com/playlist?list=PL123")
	assert.False(t, ok)
}
