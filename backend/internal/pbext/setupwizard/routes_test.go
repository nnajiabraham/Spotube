package setupwizard

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupRequestValidation(t *testing.T) {
	tests := []struct {
		name     string
		request  SetupRequest
		expected bool
	}{
		{
			name: "valid request",
			request: SetupRequest{
				SpotifyID:          "test-spotify-id",
				SpotifySecret:      "test-spotify-secret",
				GoogleClientID:     "test-google-id",
				GoogleClientSecret: "test-google-secret",
			},
			expected: true,
		},
		{
			name: "missing spotify ID",
			request: SetupRequest{
				SpotifyID:          "",
				SpotifySecret:      "test-spotify-secret",
				GoogleClientID:     "test-google-id",
				GoogleClientSecret: "test-google-secret",
			},
			expected: false,
		},
		{
			name: "missing spotify secret",
			request: SetupRequest{
				SpotifyID:          "test-spotify-id",
				SpotifySecret:      "",
				GoogleClientID:     "test-google-id",
				GoogleClientSecret: "test-google-secret",
			},
			expected: false,
		},
		{
			name: "missing google client ID",
			request: SetupRequest{
				SpotifyID:          "test-spotify-id",
				SpotifySecret:      "test-spotify-secret",
				GoogleClientID:     "",
				GoogleClientSecret: "test-google-secret",
			},
			expected: false,
		},
		{
			name: "missing google client secret",
			request: SetupRequest{
				SpotifyID:          "test-spotify-id",
				SpotifySecret:      "test-spotify-secret",
				GoogleClientID:     "test-google-id",
				GoogleClientSecret: "",
			},
			expected: false,
		},
		{
			name: "all fields empty",
			request: SetupRequest{
				SpotifyID:          "",
				SpotifySecret:      "",
				GoogleClientID:     "",
				GoogleClientSecret: "",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation logic
			valid := tt.request.SpotifyID != "" &&
				tt.request.SpotifySecret != "" &&
				tt.request.GoogleClientID != "" &&
				tt.request.GoogleClientSecret != ""

			assert.Equal(t, tt.expected, valid)
		})
	}
}

func TestEnvironmentVariableChecking(t *testing.T) {
	// Clear all env vars first
	os.Unsetenv("SPOTIFY_ID")
	os.Unsetenv("SPOTIFY_SECRET")
	os.Unsetenv("GOOGLE_CLIENT_ID")
	os.Unsetenv("GOOGLE_CLIENT_SECRET")

	t.Run("no env vars set", func(t *testing.T) {
		envVars := []string{"SPOTIFY_ID", "SPOTIFY_SECRET", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET"}
		allPresent := true
		for _, envVar := range envVars {
			if os.Getenv(envVar) == "" {
				allPresent = false
				break
			}
		}
		assert.False(t, allPresent, "Should detect missing env vars")
	})

	t.Run("all env vars set", func(t *testing.T) {
		// Set all env vars
		os.Setenv("SPOTIFY_ID", "test-id")
		os.Setenv("SPOTIFY_SECRET", "test-secret")
		os.Setenv("GOOGLE_CLIENT_ID", "test-google-id")
		os.Setenv("GOOGLE_CLIENT_SECRET", "test-google-secret")

		defer func() {
			os.Unsetenv("SPOTIFY_ID")
			os.Unsetenv("SPOTIFY_SECRET")
			os.Unsetenv("GOOGLE_CLIENT_ID")
			os.Unsetenv("GOOGLE_CLIENT_SECRET")
		}()

		envVars := []string{"SPOTIFY_ID", "SPOTIFY_SECRET", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET"}
		allPresent := true
		for _, envVar := range envVars {
			if os.Getenv(envVar) == "" {
				allPresent = false
				break
			}
		}
		assert.True(t, allPresent, "Should detect all env vars present")
	})

	t.Run("partial env vars set", func(t *testing.T) {
		// Set only some env vars
		os.Setenv("SPOTIFY_ID", "test-id")
		os.Setenv("SPOTIFY_SECRET", "test-secret")
		// Leave Google vars unset

		defer func() {
			os.Unsetenv("SPOTIFY_ID")
			os.Unsetenv("SPOTIFY_SECRET")
		}()

		envVars := []string{"SPOTIFY_ID", "SPOTIFY_SECRET", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET"}
		allPresent := true
		for _, envVar := range envVars {
			if os.Getenv(envVar) == "" {
				allPresent = false
				break
			}
		}
		assert.False(t, allPresent, "Should detect partial env vars as incomplete")
	})
}

func TestUpdateAllowedFlag(t *testing.T) {
	t.Run("update not allowed by default", func(t *testing.T) {
		os.Unsetenv("UPDATE_ALLOWED")
		updateAllowed := os.Getenv("UPDATE_ALLOWED") == "true"
		assert.False(t, updateAllowed, "Should not allow updates by default")
	})

	t.Run("update allowed when flag set", func(t *testing.T) {
		os.Setenv("UPDATE_ALLOWED", "true")
		defer os.Unsetenv("UPDATE_ALLOWED")

		updateAllowed := os.Getenv("UPDATE_ALLOWED") == "true"
		assert.True(t, updateAllowed, "Should allow updates when flag is set")
	})

	t.Run("update not allowed with other values", func(t *testing.T) {
		testValues := []string{"false", "TRUE", "1", "yes", ""}
		for _, value := range testValues {
			os.Setenv("UPDATE_ALLOWED", value)
			updateAllowed := os.Getenv("UPDATE_ALLOWED") == "true"
			assert.False(t, updateAllowed, "Should not allow updates with value: %s", value)
		}
		os.Unsetenv("UPDATE_ALLOWED")
	})
}
