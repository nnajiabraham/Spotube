package googleauth

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/labstack/echo/v5"
	"github.com/manlikeabro/spotube/internal/testhelpers"
	"github.com/pocketbase/pocketbase/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginHandler(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	os.Setenv("GOOGLE_CLIENT_ID", "test_id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test_secret")
	defer os.Unsetenv("GOOGLE_CLIENT_ID")
	defer os.Unsetenv("GOOGLE_CLIENT_SECRET")

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/login", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Test actual loginHandler function with real PocketBase app
	err := loginHandler(testApp)(c)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	redirectURL := rec.Header().Get("Location")
	assert.Contains(t, redirectURL, "accounts.google.com/o/oauth2/auth")
	assert.Contains(t, redirectURL, "scope=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fyoutube.readonly")

	// Validate PKCE challenge in redirect URL
	assert.Contains(t, redirectURL, "code_challenge")
	assert.Contains(t, redirectURL, "code_challenge_method=S256")

	// Validate cookie handling
	cookie := rec.Result().Cookies()[0]
	assert.Equal(t, cookieName, cookie.Name)
	assert.NotEmpty(t, cookie.Value)
	assert.True(t, cookie.HttpOnly)
	assert.True(t, cookie.Expires.After(time.Now()))
}

func TestCallbackHandler_Success(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	os.Setenv("GOOGLE_CLIENT_ID", "test_id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test_secret")
	defer os.Unsetenv("GOOGLE_CLIENT_ID")
	defer os.Unsetenv("GOOGLE_CLIENT_SECRET")

	testhelpers.SetupAPIHttpMocks(t)
	defer httpmock.DeactivateAndReset()

	// Mock Google OAuth token exchange endpoint
	httpmock.RegisterResponder("POST", "https://oauth2.googleapis.com/token",
		httpmock.NewJsonResponderOrPanic(200, httpmock.File("testdata/token_response.json")))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?state=test_state&code=test_code", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "test_state:test_verifier"})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Test actual callbackHandler function with real PocketBase integration
	err := callbackHandler(testApp)(c)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	assert.Equal(t, "http://localhost:5173/dashboard?youtube=connected", rec.Header().Get("Location"))

	// Validate token was stored in database
	record, err := testApp.Dao().FindFirstRecordByFilter("oauth_tokens", "provider = 'google'")
	assert.NoError(t, err)
	assert.NotNil(t, record)
	assert.Equal(t, "test_access_token", record.GetString("access_token"))
	assert.Equal(t, "test_refresh_token", record.GetString("refresh_token"))
	assert.NotEmpty(t, record.GetString("scopes"))
}

func TestCallbackHandler_TokenRefresh(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	os.Setenv("GOOGLE_CLIENT_ID", "test_id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test_secret")
	defer os.Unsetenv("GOOGLE_CLIENT_ID")
	defer os.Unsetenv("GOOGLE_CLIENT_SECRET")

	testhelpers.SetupAPIHttpMocks(t)
	defer httpmock.DeactivateAndReset()

	// Mock token refresh
	httpmock.RegisterResponder("POST", "https://oauth2.googleapis.com/token",
		httpmock.NewJsonResponderOrPanic(200, map[string]interface{}{
			"access_token":  "new_access_token",
			"refresh_token": "new_refresh_token",
			"token_type":    "Bearer",
			"expires_in":    3600,
		}))

	// Create existing expired token
	collection, err := testApp.Dao().FindCollectionByNameOrId("oauth_tokens")
	require.NoError(t, err)
	expiredToken := models.NewRecord(collection)
	expiredToken.Set("provider", "google")
	expiredToken.Set("access_token", "expired_token")
	expiredToken.Set("refresh_token", "old_refresh_token")
	expiredToken.Set("expiry", time.Now().Add(-1*time.Hour).Format("2006-01-02 15:04:05.000Z"))
	err = testApp.Dao().SaveRecord(expiredToken)
	require.NoError(t, err)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?state=test_state&code=test_code", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "test_state:test_verifier"})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = callbackHandler(testApp)(c)
	assert.NoError(t, err)

	// Verify token was updated (not created as new)
	records, err := testApp.Dao().FindRecordsByFilter("oauth_tokens", "provider = 'google'", "", 10, 0)
	assert.NoError(t, err)
	assert.Len(t, records, 1, "Should update existing token, not create new one")
	assert.Equal(t, "new_access_token", records[0].GetString("access_token"))
}

func TestCallbackHandler_ErrorScenarios(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	os.Setenv("GOOGLE_CLIENT_ID", "test_id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test_secret")
	defer os.Unsetenv("GOOGLE_CLIENT_ID")
	defer os.Unsetenv("GOOGLE_CLIENT_SECRET")

	t.Run("missing state parameter", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?code=test_code", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := callbackHandler(testApp)(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
		assert.Contains(t, rec.Header().Get("Location"), "youtube=error")
	})

	t.Run("missing code parameter", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?state=test_state", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := callbackHandler(testApp)(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
		assert.Contains(t, rec.Header().Get("Location"), "youtube=error")
	})

	t.Run("missing cookie", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?state=test_state&code=test_code", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := callbackHandler(testApp)(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
		assert.Contains(t, rec.Header().Get("Location"), "youtube=error")
	})
}

func TestPlaylistsHandler_Integration(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	os.Setenv("GOOGLE_CLIENT_ID", "test_id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test_secret")
	defer os.Unsetenv("GOOGLE_CLIENT_ID")
	defer os.Unsetenv("GOOGLE_CLIENT_SECRET")

	testhelpers.SetupAPIHttpMocks(t)
	defer httpmock.DeactivateAndReset()

	// Read the mock response files
	playlistsData, err := os.ReadFile("testdata/playlists_response.json")
	require.NoError(t, err)

	// Mock the YouTube API endpoint with actual test data
	httpmock.RegisterResponder("GET", `=~^https://.*youtube.*playlists`,
		func(req *http.Request) (*http.Response, error) {
			t.Logf("YouTube playlists API called: %s", req.URL.String())
			return httpmock.NewBytesResponse(200, playlistsData), nil
		})

	// Setup OAuth tokens using shared helper
	testhelpers.SetupOAuthTokens(t, testApp)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/youtube/playlists", nil)
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	// Test actual playlistsHandler function
	err = playlistsHandler(testApp)(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Contains(t, res.Body.String(), "Test Playlist")

	// Verify httpmock was called
	info := httpmock.GetCallCountInfo()
	t.Logf("HTTP mock call counts: %+v", info)
	// Check that YouTube API was called (the exact key might vary)
	youtubeCalled := false
	for key := range info {
		if strings.Contains(key, "youtube") && info[key] > 0 {
			youtubeCalled = true
			break
		}
	}
	assert.True(t, youtubeCalled, "YouTube API should be called")
}

func TestWithGoogleClient_ValidToken(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	os.Setenv("GOOGLE_CLIENT_ID", "test_id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test_secret")
	defer os.Unsetenv("GOOGLE_CLIENT_ID")
	defer os.Unsetenv("GOOGLE_CLIENT_SECRET")

	// Setup valid OAuth tokens using shared helper
	testhelpers.SetupOAuthTokens(t, testApp)

	// Test WithGoogleClient function with valid token
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	client, err := WithGoogleClient(testApp, c)
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestWithGoogleClient_MissingToken(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	os.Setenv("GOOGLE_CLIENT_ID", "test_id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test_secret")
	defer os.Unsetenv("GOOGLE_CLIENT_ID")
	defer os.Unsetenv("GOOGLE_CLIENT_SECRET")

	// Don't setup OAuth tokens - test missing token scenario
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	client, err := WithGoogleClient(testApp, c)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "no google token found")
}

func TestPlaylistsHandler_ErrorScenarios(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	os.Setenv("GOOGLE_CLIENT_ID", "test_id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test_secret")
	defer os.Unsetenv("GOOGLE_CLIENT_ID")
	defer os.Unsetenv("GOOGLE_CLIENT_SECRET")

	t.Run("missing OAuth token", func(t *testing.T) {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/youtube/playlists", nil)
		res := httptest.NewRecorder()
		c := e.NewContext(req, res)

		err := playlistsHandler(testApp)(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, res.Code)
	})

	t.Run("API failure", func(t *testing.T) {
		testhelpers.SetupOAuthTokens(t, testApp)
		testhelpers.SetupAPIHttpMocks(t)
		defer httpmock.DeactivateAndReset()

		// Mock API failure
		httpmock.RegisterResponder("GET", `=~^https://.*youtube.*playlists`,
			httpmock.NewStringResponder(500, "Internal Server Error"))

		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/youtube/playlists", nil)
		res := httptest.NewRecorder()
		c := e.NewContext(req, res)

		err := playlistsHandler(testApp)(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusInternalServerError, res.Code)
	})
}
