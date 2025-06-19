package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/testhelpers"
)

func TestSpotifyClientFactory(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)

	// Setup test credentials in settings
	setupTestCredentials(t, testApp)

	// Setup test OAuth token
	setupSpotifyToken(t, testApp)

	t.Run("job context", func(t *testing.T) {
		// Activate HTTP mocking
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// Mock Spotify API calls
		httpmock.RegisterResponder("GET", "https://api.spotify.com/v1/me",
			httpmock.NewStringResponder(200, `{"id": "testuser"}`))

		ctx := context.Background()
		authCtx := NewJobAuthContext(testApp)

		client, err := GetSpotifyClient(ctx, testApp, authCtx)
		require.NoError(t, err)
		require.NotNil(t, client)
	})

	t.Run("API context", func(t *testing.T) {
		// Activate HTTP mocking
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// Mock Spotify API calls
		httpmock.RegisterResponder("GET", "https://api.spotify.com/v1/me",
			httpmock.NewStringResponder(200, `{"id": "testuser"}`))

		// Create mock Echo context
		e := echo.New()
		req, _ := http.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		authCtx := NewAPIAuthContext(c, testApp)

		client, err := GetSpotifyClient(c.Request().Context(), testApp, authCtx)
		require.NoError(t, err)
		require.NotNil(t, client)
	})

	t.Run("helper functions", func(t *testing.T) {
		// Activate HTTP mocking
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// Mock Spotify API calls
		httpmock.RegisterResponder("GET", "https://api.spotify.com/v1/me",
			httpmock.NewStringResponder(200, `{"id": "testuser"}`))

		// Test job helper
		ctx := context.Background()
		client, err := GetSpotifyClientForJob(ctx, testApp)
		require.NoError(t, err)
		require.NotNil(t, client)

		// Test API helper
		e := echo.New()
		req, _ := http.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		client, err = WithSpotifyClient(testApp, c)
		require.NoError(t, err)
		require.NotNil(t, client)
	})
}

func TestSpotifyClientErrors(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)

	t.Run("missing credentials", func(t *testing.T) {
		ctx := context.Background()
		authCtx := NewJobAuthContext(testApp)

		_, err := GetSpotifyClient(ctx, testApp, authCtx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to load Spotify credentials")
	})

	t.Run("missing token", func(t *testing.T) {
		// Setup credentials but no token
		setupTestCredentials(t, testApp)

		ctx := context.Background()
		authCtx := NewJobAuthContext(testApp)

		_, err := GetSpotifyClient(ctx, testApp, authCtx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to load Spotify token")
	})
}

func setupTestCredentials(t *testing.T, testApp *tests.TestApp) {
	dao := testApp.Dao()
	collection, err := dao.FindCollectionByNameOrId("settings")
	require.NoError(t, err)

	record := models.NewRecord(collection)
	record.SetId("settings")
	record.Set("spotify_client_id", "test_client_id")
	record.Set("spotify_client_secret", "test_client_secret")

	err = dao.SaveRecord(record)
	require.NoError(t, err)
}

func setupSpotifyToken(t *testing.T, testApp *tests.TestApp) {
	dao := testApp.Dao()
	collection, err := dao.FindCollectionByNameOrId("oauth_tokens")
	require.NoError(t, err)

	record := models.NewRecord(collection)
	record.Set("provider", "spotify")
	record.Set("access_token", "test_access_token")
	record.Set("refresh_token", "test_refresh_token")
	record.Set("expiry", time.Now().Add(time.Hour))
	record.Set("scopes", "test-scope")

	err = dao.SaveRecord(record)
	require.NoError(t, err)
}
