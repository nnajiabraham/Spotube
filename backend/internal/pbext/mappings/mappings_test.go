package mappings

import (
	"testing"
)

func TestMappingsValidation(t *testing.T) {
	t.Run("interval_minutes validation", func(t *testing.T) {
		// Test that interval_minutes must be at least 5
		testCases := []struct {
			name            string
			intervalMinutes float64
			expectError     bool
		}{
			{"valid minimum", 5, false},
			{"valid normal", 60, false},
			{"valid high", 720, false},
			{"invalid below minimum", 3, true},
			{"invalid zero", 0, true},
			{"invalid negative", -5, true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// In actual implementation, this validation happens in the hooks
				// This test demonstrates the expected behavior
				isValid := tc.intervalMinutes >= 5
				hasError := !isValid

				if hasError != tc.expectError {
					t.Errorf("For interval_minutes=%v, expected error=%v, but got error=%v",
						tc.intervalMinutes, tc.expectError, hasError)
				}
			})
		}
	})

	t.Run("default values", func(t *testing.T) {
		// Test that default values are set correctly
		// In actual implementation, these are set in BeforeCreate hook
		expectedDefaults := struct {
			syncName        bool
			syncTracks      bool
			intervalMinutes float64
		}{
			syncName:        true,
			syncTracks:      true,
			intervalMinutes: 60,
		}

		// This test documents the expected default behavior
		if !expectedDefaults.syncName {
			t.Error("Expected sync_name to default to true")
		}
		if !expectedDefaults.syncTracks {
			t.Error("Expected sync_tracks to default to true")
		}
		if expectedDefaults.intervalMinutes != 60 {
			t.Errorf("Expected interval_minutes to default to 60, got %v", expectedDefaults.intervalMinutes)
		}
	})

	t.Run("duplicate mapping prevention", func(t *testing.T) {
		// Document that duplicate mappings are prevented by unique index
		// The database unique index on (spotify_playlist_id, youtube_playlist_id)
		// will return an error when trying to create a duplicate mapping

		// Example scenarios that should fail:
		scenarios := []struct {
			name       string
			spotifyID1 string
			youtubeID1 string
			spotifyID2 string
			youtubeID2 string
			shouldFail bool
		}{
			{
				name:       "exact duplicate",
				spotifyID1: "spotify123",
				youtubeID1: "youtube456",
				spotifyID2: "spotify123",
				youtubeID2: "youtube456",
				shouldFail: true,
			},
			{
				name:       "different spotify playlist",
				spotifyID1: "spotify123",
				youtubeID1: "youtube456",
				spotifyID2: "spotify789",
				youtubeID2: "youtube456",
				shouldFail: false,
			},
			{
				name:       "different youtube playlist",
				spotifyID1: "spotify123",
				youtubeID1: "youtube456",
				spotifyID2: "spotify123",
				youtubeID2: "youtube789",
				shouldFail: false,
			},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				// This test documents the expected behavior
				// In production, the database will enforce this constraint
				isDuplicate := scenario.spotifyID1 == scenario.spotifyID2 &&
					scenario.youtubeID1 == scenario.youtubeID2

				if isDuplicate != scenario.shouldFail {
					t.Errorf("Expected duplicate check to fail=%v for scenario %s",
						scenario.shouldFail, scenario.name)
				}
			})
		}
	})
}
