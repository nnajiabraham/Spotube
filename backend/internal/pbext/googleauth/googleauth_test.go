package googleauth

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/models/schema"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoginHandler(t *testing.T) {
	os.Setenv("GOOGLE_CLIENT_ID", "test_id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test_secret")
	defer os.Unsetenv("GOOGLE_CLIENT_ID")
	defer os.Unsetenv("GOOGLE_CLIENT_SECRET")

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/login", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// loginHandler doesn't use the app instance for db access, so we can pass a mock/nil daoProvider
	err := loginHandler(nil)(c)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	redirectURL := rec.Header().Get("Location")
	assert.Contains(t, redirectURL, "accounts.google.com/o/oauth2/auth")
	assert.Contains(t, redirectURL, "scope=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fyoutube.readonly")

	cookie := rec.Result().Cookies()[0]
	assert.Equal(t, cookieName, cookie.Name)
	assert.NotEmpty(t, cookie.Value)
}

func TestCallbackHandler_Success(t *testing.T) {
	testApp, err := tests.NewTestApp()
	require.NoError(t, err)
	defer testApp.Cleanup()

	os.Setenv("GOOGLE_CLIENT_ID", "test_id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test_secret")
	defer os.Unsetenv("GOOGLE_CLIENT_ID")
	defer os.Unsetenv("GOOGLE_CLIENT_SECRET")

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("POST", "https://oauth2.googleapis.com/token",
		httpmock.NewJsonResponderOrPanic(200, httpmock.File("testdata/token_response.json")))

	// Manually create the collection for the test database
	collection := &models.Collection{}
	collection.Name = "oauth_tokens"
	collection.Type = models.CollectionTypeBase
	collection.Schema = schema.NewSchema(
		&schema.SchemaField{Name: "provider", Type: schema.FieldTypeText},
		&schema.SchemaField{Name: "access_token", Type: schema.FieldTypeText},
		&schema.SchemaField{Name: "refresh_token", Type: schema.FieldTypeText},
		&schema.SchemaField{Name: "expiry", Type: schema.FieldTypeDate},
		&schema.SchemaField{Name: "scopes", Type: schema.FieldTypeText},
	)
	err = testApp.Dao().SaveCollection(collection)
	require.NoError(t, err)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback?state=test_state&code=test_code", nil)
	req.AddCookie(&http.Cookie{Name: cookieName, Value: "test_state:test_verifier"})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// testApp satisfies the daoProvider interface
	err = callbackHandler(testApp)(c)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	assert.Equal(t, "/dashboard?youtube=connected", rec.Header().Get("Location"))

	record, err := testApp.Dao().FindFirstRecordByFilter("oauth_tokens", "provider = 'google'")
	assert.NoError(t, err)
	assert.NotNil(t, record)
	assert.Equal(t, "test_access_token", record.GetString("access_token"))
}

func TestPlaylistsHandler_Success(t *testing.T) {
	// Activate httpmock FIRST before any HTTP client usage
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	testApp, err := tests.NewTestApp()
	require.NoError(t, err)
	defer testApp.Cleanup()

	os.Setenv("GOOGLE_CLIENT_ID", "test_id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "test_secret")
	defer os.Unsetenv("GOOGLE_CLIENT_ID")
	defer os.Unsetenv("GOOGLE_CLIENT_SECRET")

	// Read the mock response files
	playlistsData, err := os.ReadFile("testdata/playlists_response.json")
	require.NoError(t, err)

	// Mock the YouTube API endpoint - use a more flexible matcher
	httpmock.RegisterResponder("GET", `=~^https://.*youtube.*playlists`,
		func(req *http.Request) (*http.Response, error) {
			t.Logf("Mock hit for URL: %s", req.URL.String())
			return httpmock.NewBytesResponse(200, playlistsData), nil
		})

	// Also mock the token endpoint in case it's called
	httpmock.RegisterResponder("POST", "https://oauth2.googleapis.com/token",
		httpmock.NewJsonResponderOrPanic(200, map[string]interface{}{
			"access_token": "new_access_token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		}))

	// Manually create the collection for the test database
	collection := &models.Collection{}
	collection.Name = "oauth_tokens"
	collection.Type = models.CollectionTypeBase
	collection.Schema = schema.NewSchema(
		&schema.SchemaField{Name: "provider", Type: schema.FieldTypeText},
		&schema.SchemaField{Name: "access_token", Type: schema.FieldTypeText},
		&schema.SchemaField{Name: "refresh_token", Type: schema.FieldTypeText},
		&schema.SchemaField{Name: "expiry", Type: schema.FieldTypeDate},
		&schema.SchemaField{Name: "scopes", Type: schema.FieldTypeText},
	)
	err = testApp.Dao().SaveCollection(collection)
	require.NoError(t, err)

	collectionAfter, err := testApp.Dao().FindCollectionByNameOrId("oauth_tokens")
	require.NoError(t, err)

	rec := models.NewRecord(collectionAfter)
	rec.Set("provider", "google")
	rec.Set("access_token", "fake_access_token")
	rec.Set("refresh_token", "fake_refresh_token")
	rec.Set("expiry", time.Now().Add(1*time.Hour).Format(time.RFC3339))
	err = testApp.Dao().SaveRecord(rec)
	require.NoError(t, err)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/youtube/playlists", nil)
	res := httptest.NewRecorder()
	c := e.NewContext(req, res)

	// Use the handler with httpmock's HTTP client - this ensures httpmock intercepts requests
	mockClient := &http.Client{
		Transport: httpmock.DefaultTransport,
	}
	handler := playlistsHandlerWithClient(testApp, mockClient)
	err = handler(c)

	// Log the response for debugging
	t.Logf("Response status: %d", res.Code)
	t.Logf("Response body: %s", res.Body.String())

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.Code)
	assert.Contains(t, res.Body.String(), "Test Playlist")

	// Verify httpmock was called
	info := httpmock.GetCallCountInfo()
	t.Logf("HTTP mock call counts: %+v", info)
}
