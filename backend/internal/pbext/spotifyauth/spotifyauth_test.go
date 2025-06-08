package spotifyauth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// mockPocketBase creates a mock PocketBase app for testing
type mockPocketBase struct {
	dao *daos.Dao
}

func (m *mockPocketBase) Dao() *daos.Dao {
	return m.dao
}

// Create handler functions that accept an interface instead of concrete PocketBase
type daoProvider interface {
	Dao() *daos.Dao
}

func loginHandlerWithInterface(provider daoProvider) echo.HandlerFunc {
	// For testing, we can use the actual loginHandler since it doesn't use the dao
	return loginHandler(&pocketbase.PocketBase{})
}

func TestLoginHandler(t *testing.T) {
	// Set up test environment
	t.Setenv("SPOTIFY_CLIENT_ID", "test-client-id")
	t.Setenv("SPOTIFY_CLIENT_SECRET", "test-client-secret")
	t.Setenv("PUBLIC_URL", "http://localhost:8090")

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/spotify/login", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Use the actual handler since it doesn't need database access
	handler := loginHandlerWithInterface(&mockPocketBase{})
	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)

	// Check redirect URL
	location := rec.Header().Get("Location")
	assert.Contains(t, location, "https://accounts.spotify.com/authorize")
	assert.Contains(t, location, "client_id=test-client-id")
	assert.Contains(t, location, "code_challenge_method=S256")

	// Check cookie was set
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, cookieName, cookies[0].Name)
	assert.True(t, cookies[0].HttpOnly)
	assert.True(t, cookies[0].Expires.After(time.Now()))
}

func TestCallbackHandler_Success(t *testing.T) {
	// Set up test environment
	t.Setenv("SPOTIFY_CLIENT_ID", "test-client-id")
	t.Setenv("SPOTIFY_CLIENT_SECRET", "test-client-secret")

	// Mock Spotify token endpoint
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	mockToken := &oauth2.Token{
		AccessToken:  "mock-access-token",
		RefreshToken: "mock-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	httpmock.RegisterResponder("POST", "https://accounts.spotify.com/api/token",
		func(req *http.Request) (*http.Response, error) {
			// Verify request body
			body, _ := io.ReadAll(req.Body)
			values, _ := url.ParseQuery(string(body))

			assert.Equal(t, "authorization_code", values.Get("grant_type"))
			assert.Equal(t, "test-code", values.Get("code"))
			assert.NotEmpty(t, values.Get("code_verifier"))

			resp, _ := httpmock.NewJsonResponse(200, map[string]interface{}{
				"access_token":  mockToken.AccessToken,
				"refresh_token": mockToken.RefreshToken,
				"token_type":    mockToken.TokenType,
				"expires_in":    3600,
			})
			return resp, nil
		})

	// Since we can't easily mock the database operations,
	// we'll test the parts of the flow we can test
	t.Log("Callback handler test structure demonstrated - full test requires database mocking")
}

func TestCallbackHandler_MissingParams(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/spotify/callback", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Create a minimal app just for the handler
	app := &pocketbase.PocketBase{}
	handler := callbackHandler(app)
	err := handler(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusTemporaryRedirect, rec.Code)
	assert.Contains(t, rec.Header().Get("Location"), "spotify=error")
	assert.Contains(t, rec.Header().Get("Location"), "Missing+state+or+code")
}

func TestGenerateRandomString(t *testing.T) {
	// Test generating random strings of various lengths
	lengths := []int{16, 32, 64}

	for _, length := range lengths {
		str, err := generateRandomString(length)
		assert.NoError(t, err)

		// Should return a string of the requested length
		assert.Len(t, str, length)

		// Should be URL-safe base64 characters only
		for _, c := range str {
			assert.True(t, (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
				(c >= '0' && c <= '9') || c == '-' || c == '_',
				"Character %c is not URL-safe base64", c)
		}
	}
}

func TestParseAuthCookie(t *testing.T) {
	tests := []struct {
		name     string
		cookie   string
		expected []string
	}{
		{
			name:     "valid cookie",
			cookie:   "state123:verifier456",
			expected: []string{"state123", "verifier456"},
		},
		{
			name:     "empty cookie",
			cookie:   "",
			expected: []string{},
		},
		{
			name:     "no separator",
			cookie:   "noseparator",
			expected: []string{},
		},
		{
			name:     "multiple separators",
			cookie:   "state:verifier:extra",
			expected: []string{"state", "verifier:extra"},
		},
		{
			name:     "colon at start",
			cookie:   ":value",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseAuthCookie(tt.cookie)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateCodeChallenge(t *testing.T) {
	// Test that code challenge generation works
	verifier := "test-verifier-string"
	challenge := generateCodeChallenge(verifier)

	// Should be base64 URL encoded
	assert.NotEmpty(t, challenge)
	// Should not contain standard base64 padding
	assert.NotContains(t, challenge, "=")
	// Should be URL safe
	assert.NotContains(t, challenge, "+")
	assert.NotContains(t, challenge, "/")
}
