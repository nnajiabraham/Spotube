package testhelpers

import (
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
)

// SetupAPIHttpMocks configures HTTP mocks for Spotify and YouTube APIs
func SetupAPIHttpMocks(t *testing.T) {
	httpmock.Activate()

	// Clear any existing responders
	httpmock.Reset()

	// Setup default mocks for both services
	SetupSpotifyMocks(t, map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"track": map[string]interface{}{
					"id":   "spotify_track_1",
					"name": "Song 1",
				},
			},
			{
				"track": map[string]interface{}{
					"id":   "spotify_track_2",
					"name": "Song 2",
				},
			},
		},
	})

	SetupYouTubeMocks(t, map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"id": "youtube_item_1",
				"snippet": map[string]interface{}{
					"title": "Song 2", // Overlaps with Spotify
				},
			},
			{
				"id": "youtube_item_2",
				"snippet": map[string]interface{}{
					"title": "Song 3", // Only on YouTube
				},
			},
		},
	})

	// Setup OAuth token refresh endpoints
	SetupOAuthRefreshMocks(t)
}

// SetupSpotifyMocks configures only Spotify API mocks
func SetupSpotifyMocks(t *testing.T, tracks map[string]interface{}) {
	httpmock.RegisterResponder("GET", `=~^https://api\.spotify\.com/v1/playlists/.*/tracks`,
		func(req *http.Request) (*http.Response, error) {
			t.Logf("Spotify API called: %s", req.URL.String())
			return httpmock.NewJsonResponse(200, tracks)
		})
}

// SetupYouTubeMocks configures only YouTube API mocks
func SetupYouTubeMocks(t *testing.T, items map[string]interface{}) {
	httpmock.RegisterResponder("GET", `=~^https://.*youtube.*playlistItems`,
		func(req *http.Request) (*http.Response, error) {
			t.Logf("YouTube API called: %s", req.URL.String())
			return httpmock.NewJsonResponse(200, items)
		})
}

// SetupOAuthRefreshMocks configures OAuth token refresh endpoints for both services
func SetupOAuthRefreshMocks(t *testing.T) {
	// Mock Spotify OAuth token refresh
	httpmock.RegisterResponder("POST", "https://accounts.spotify.com/api/token",
		httpmock.NewJsonResponderOrPanic(200, map[string]interface{}{
			"access_token":  "refreshed_spotify_token",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"refresh_token": "fake_spotify_refresh",
		}))

	// Mock Google OAuth token refresh
	httpmock.RegisterResponder("POST", "https://oauth2.googleapis.com/token",
		httpmock.NewJsonResponderOrPanic(200, map[string]interface{}{
			"access_token":  "refreshed_google_token",
			"token_type":    "Bearer",
			"expires_in":    3600,
			"refresh_token": "fake_google_refresh",
		}))
}

// SetupIdenticalPlaylistMocks sets up mocks for identical Spotify and YouTube playlists (for testing no-change scenarios)
func SetupIdenticalPlaylistMocks(t *testing.T) {
	httpmock.Activate()
	httpmock.Reset()

	identicalTracks := map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"track": map[string]interface{}{
					"id":   "same_track_1",
					"name": "Same Song",
				},
			},
		},
	}

	SetupSpotifyMocks(t, identicalTracks)

	youtubeIdentical := map[string]interface{}{
		"items": []map[string]interface{}{
			{
				"id": "same_track_1", // Same ID as Spotify
				"snippet": map[string]interface{}{
					"title": "Same Song",
				},
			},
		},
	}

	SetupYouTubeMocks(t, youtubeIdentical)
	SetupOAuthRefreshMocks(t)
}
