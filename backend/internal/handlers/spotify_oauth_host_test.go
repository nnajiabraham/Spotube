package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpotifyCallbackStateMismatchUsesLocalhostFrontend(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-key"))
	handler := &SpotifyOAuthHandler{
		Repo:         &fakeCredentialProvider{id: "spotify-id", secret: "secret"},
		SessionStore: store,
		RedirectURI:  "http://127.0.0.1:8090/api/auth/spotify/callback",
		FrontendURL:  "http://127.0.0.1:5173",
		Scopes:       []string{"playlist-read-private"},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/spotify/callback?code=test&state=wrong", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := handler.Callback(ctx)
	require.NoError(t, err)
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "http://localhost:5173/dashboard?message=state+mismatch&spotify=error", rec.Header().Get("Location"))
}
