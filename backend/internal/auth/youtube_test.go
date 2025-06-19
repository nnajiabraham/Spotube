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

func TestYouTubeServiceFactory(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)

	// Setup test credentials in settings
	setupGoogleTestCredentials(t, testApp)

	// Setup test OAuth token
	setupGoogleToken(t, testApp)

	t.Run("job context", func(t *testing.T) {
		// Activate HTTP mocking
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// Mock YouTube API calls
		httpmock.RegisterResponder("GET", "https://youtube.googleapis.com/youtube/v3/playlists",
			httpmock.NewStringResponder(200, `{"items": []}`))

		ctx := context.Background()
		authCtx := NewJobAuthContext(testApp)

		service, err := GetYouTubeService(ctx, testApp, authCtx)
		require.NoError(t, err)
		require.NotNil(t, service)
	})

	t.Run("API context", func(t *testing.T) {
		// Activate HTTP mocking
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// Mock YouTube API calls
		httpmock.RegisterResponder("GET", "https://youtube.googleapis.com/youtube/v3/playlists",
			httpmock.NewStringResponder(200, `{"items": []}`))

		// Create mock Echo context
		e := echo.New()
		req, _ := http.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		authCtx := NewAPIAuthContext(c, testApp)

		service, err := GetYouTubeService(c.Request().Context(), testApp, authCtx)
		require.NoError(t, err)
		require.NotNil(t, service)
	})

	t.Run("helper functions", func(t *testing.T) {
		// Activate HTTP mocking
		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		// Mock YouTube API calls
		httpmock.RegisterResponder("GET", "https://youtube.googleapis.com/youtube/v3/playlists",
			httpmock.NewStringResponder(200, `{"items": []}`))

		// Test job helper
		ctx := context.Background()
		service, err := GetYouTubeServiceForJob(ctx, testApp)
		require.NoError(t, err)
		require.NotNil(t, service)

		// Test API helper
		ctx = context.Background()
		service, err = WithGoogleClient(ctx, testApp)
		require.NoError(t, err)
		require.NotNil(t, service)

		// Test custom client helper
		customClient := &http.Client{}
		service, err = WithGoogleClientCustom(ctx, testApp, customClient)
		require.NoError(t, err)
		require.NotNil(t, service)
	})
}

func TestYouTubeServiceErrors(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)

	t.Run("missing credentials", func(t *testing.T) {
		ctx := context.Background()
		authCtx := NewJobAuthContext(testApp)

		_, err := GetYouTubeService(ctx, testApp, authCtx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to load Google credentials")
	})

	t.Run("missing token", func(t *testing.T) {
		// Setup credentials but no token
		setupGoogleTestCredentials(t, testApp)

		ctx := context.Background()
		authCtx := NewJobAuthContext(testApp)

		_, err := GetYouTubeService(ctx, testApp, authCtx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to load Google token")
	})
}

func setupGoogleTestCredentials(t *testing.T, testApp *tests.TestApp) {
	dao := testApp.Dao()
	collection, err := dao.FindCollectionByNameOrId("settings")
	require.NoError(t, err)

	record := models.NewRecord(collection)
	record.SetId("settings")
	record.Set("google_client_id", "test_google_client_id")
	record.Set("google_client_secret", "test_google_client_secret")

	err = dao.SaveRecord(record)
	require.NoError(t, err)
}

func setupGoogleToken(t *testing.T, testApp *tests.TestApp) {
	dao := testApp.Dao()
	collection, err := dao.FindCollectionByNameOrId("oauth_tokens")
	require.NoError(t, err)

	record := models.NewRecord(collection)
	record.Set("provider", "google")
	record.Set("access_token", "test_google_access_token")
	record.Set("refresh_token", "test_google_refresh_token")
	record.Set("expiry", time.Now().Add(time.Hour))
	record.Set("scopes", "test-scope")

	err = dao.SaveRecord(record)
	require.NoError(t, err)
}
