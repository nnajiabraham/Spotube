package spotifyauth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/manlikeabro/spotube/internal/testhelpers"
	"github.com/stretchr/testify/assert"
)

// TestOAuthWithSettingsCollection tests that OAuth handlers use credentials from settings collection
// when no environment variables are set
func TestOAuthWithSettingsCollection(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	// Setup settings collection with credentials (no env vars)
	setupSettingsWithCredentials(t, testApp, "db_spotify_id", "db_spotify_secret")

	t.Run("login handler uses settings collection credentials", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/auth/spotify/login", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := loginHandlerWithInterface(testApp)
		err := handler(c)

		// Should succeed with settings collection credentials
		assert.NoError(t, err)
		assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)

		// Check redirect URL contains the correct client ID from settings
		location := rec.Header().Get("Location")
		assert.Contains(t, location, "client_id=db_spotify_id")
	})

	t.Run("callback handler uses settings collection credentials", func(t *testing.T) {
		// Setup HTTP mocks for token exchange
		testhelpers.SetupAPIHttpMocks(t)
		defer func() {
			// Clean up HTTP mocks if they exist
			// httpmock.DeactivateAndReset() will be called by testhelpers
		}()

		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/auth/spotify/callback?state=test-state&code=test-code", nil)
		req.AddCookie(&http.Cookie{Name: cookieName, Value: "test-state:test-verifier"})
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := callbackHandlerWithInterface(testApp)
		err := handler(c)

		// Should succeed with settings collection credentials
		assert.NoError(t, err)
		assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
		assert.Contains(t, rec.Header().Get("Location"), "spotify=connected")
	})
}

// TestOAuthWithEnvironmentVariables tests that OAuth handlers use environment variables
// when settings collection is empty
func TestOAuthWithEnvironmentVariables(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	// Set environment variables
	t.Setenv("SPOTIFY_CLIENT_ID", "env_spotify_id")
	t.Setenv("SPOTIFY_CLIENT_SECRET", "env_spotify_secret")
	t.Setenv("PUBLIC_URL", "http://localhost:8090")

	// Create empty settings record (no credentials)
	setupSettingsWithCredentials(t, testApp, "", "")

	t.Run("login handler falls back to environment variables", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/auth/spotify/login", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := loginHandlerWithInterface(testApp)
		err := handler(c)

		// Should succeed with environment variables
		assert.NoError(t, err)
		assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)

		// Check redirect URL contains the correct client ID from environment
		location := rec.Header().Get("Location")
		assert.Contains(t, location, "client_id=env_spotify_id")
	})
}

// TestOAuthSettingsCollectionPriority tests that settings collection takes priority over environment variables
func TestOAuthSettingsCollectionPriority(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	// Set environment variables
	t.Setenv("SPOTIFY_CLIENT_ID", "env_spotify_id")
	t.Setenv("SPOTIFY_CLIENT_SECRET", "env_spotify_secret")

	// Set different values in settings collection (should take priority)
	setupSettingsWithCredentials(t, testApp, "priority_spotify_id", "priority_spotify_secret")

	t.Run("settings collection takes priority over environment variables", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/auth/spotify/login", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := loginHandlerWithInterface(testApp)
		err := handler(c)

		// Should succeed
		assert.NoError(t, err)
		assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)

		// Check redirect URL contains the correct client ID from settings (not environment)
		location := rec.Header().Get("Location")
		assert.Contains(t, location, "client_id=priority_spotify_id")
		assert.NotContains(t, location, "client_id=env_spotify_id")
	})
}

// TestOAuthWithNoCredentials tests that OAuth handlers fail gracefully when no credentials are available
func TestOAuthWithNoCredentials(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	// Create empty settings record and no environment variables
	setupSettingsWithCredentials(t, testApp, "", "")

	t.Run("login handler fails when no credentials available", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/auth/spotify/login", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := loginHandlerWithInterface(testApp)
		err := handler(c)

		// Should return error
		assert.NoError(t, err) // Handler doesn't return error, just sends JSON response
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Contains(t, rec.Body.String(), "Spotify client credentials not configured")
	})
}

// Helper function to setup settings collection with given credentials is defined in spotifyauth_test.go
