package googleauth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/manlikeabro/spotube/internal/testhelpers"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGoogleOAuthWithSettingsCollection tests that OAuth handlers use credentials from settings collection
// when no environment variables are set
func TestGoogleOAuthWithSettingsCollection(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	// Setup settings collection with credentials (no env vars)
	setupGoogleSettingsWithCredentials(t, testApp, "db_google_id", "db_google_secret")

	t.Run("login handler uses settings collection credentials", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/auth/google/login", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := loginHandler(testApp)
		err := handler(c)

		// Should succeed with settings collection credentials
		assert.NoError(t, err)
		assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)

		// Check redirect URL contains the correct client ID from settings
		location := rec.Header().Get("Location")
		assert.Contains(t, location, "client_id=db_google_id")
	})
}

// TestGoogleOAuthWithEnvironmentVariables tests that OAuth handlers use environment variables
// when settings collection is empty
func TestGoogleOAuthWithEnvironmentVariables(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	// Set environment variables
	t.Setenv("GOOGLE_CLIENT_ID", "env_google_id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "env_google_secret")
	t.Setenv("PUBLIC_URL", "http://localhost:8090")

	// Create empty settings record (no credentials)
	setupGoogleSettingsWithCredentials(t, testApp, "", "")

	t.Run("login handler falls back to environment variables", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/auth/google/login", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := loginHandler(testApp)
		err := handler(c)

		// Should succeed with environment variables
		assert.NoError(t, err)
		assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)

		// Check redirect URL contains the correct client ID from environment
		location := rec.Header().Get("Location")
		assert.Contains(t, location, "client_id=env_google_id")
	})
}

// TestGoogleOAuthSettingsCollectionPriority tests that settings collection takes priority over environment variables
func TestGoogleOAuthSettingsCollectionPriority(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	// Set environment variables
	t.Setenv("GOOGLE_CLIENT_ID", "env_google_id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "env_google_secret")

	// Set different values in settings collection (should take priority)
	setupGoogleSettingsWithCredentials(t, testApp, "priority_google_id", "priority_google_secret")

	t.Run("settings collection takes priority over environment variables", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/auth/google/login", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := loginHandler(testApp)
		err := handler(c)

		// Should succeed
		assert.NoError(t, err)
		assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)

		// Check redirect URL contains the correct client ID from settings (not environment)
		location := rec.Header().Get("Location")
		assert.Contains(t, location, "client_id=priority_google_id")
		assert.NotContains(t, location, "client_id=env_google_id")
	})
}

// TestGoogleOAuthWithNoCredentials tests that OAuth handlers fail gracefully when no credentials are available
func TestGoogleOAuthWithNoCredentials(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	// Create empty settings record and no environment variables
	setupGoogleSettingsWithCredentials(t, testApp, "", "")

	t.Run("login handler fails when no credentials available", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/auth/google/login", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		handler := loginHandler(testApp)
		err := handler(c)

		// Should return error
		assert.NoError(t, err) // Handler doesn't return error, just sends JSON response
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Contains(t, rec.Body.String(), "Google client credentials not configured")
	})
}

// Helper function to setup settings collection with given Google credentials
func setupGoogleSettingsWithCredentials(t *testing.T, testApp *tests.TestApp, googleID, googleSecret string) {
	dao := testApp.Dao()
	collection, err := dao.FindCollectionByNameOrId("settings")
	require.NoError(t, err)

	// Try to find existing record or create new one
	record, err := dao.FindRecordById("settings", "settings")
	if err != nil {
		// Create new record if it doesn't exist
		record = models.NewRecord(collection)
		record.SetId("settings")
	}

	// Set Google credentials
	record.Set("google_client_id", googleID)
	record.Set("google_client_secret", googleSecret)

	err = dao.SaveRecord(record)
	require.NoError(t, err)
}
