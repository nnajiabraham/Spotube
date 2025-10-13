package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"

	"github.com/manlikeabro/spotube/internal/auth"
)

type fakeCredentialProvider struct {
	id     string
	secret string
	err    error
}

func (f *fakeCredentialProvider) GetSettings() (*auth.SettingsRecord, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &auth.SettingsRecord{
		SpotifyClientID:     nullableStringTest(f.id),
		SpotifyClientSecret: nullableStringTest(f.secret),
	}, nil
}

func TestSpotifyLoginRedirects(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-key"))
	handler := &SpotifyOAuthHandler{
		Repo:         &fakeCredentialProvider{id: "spotify-id", secret: "secret"},
		SessionStore: store,
		RedirectURI:  "http://localhost:8090/callback",
		Scopes:       []string{"playlist-read-private"},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/spotify/login", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := handler.Login(ctx); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d", rec.Code)
	}

	location := rec.Header().Get("Location")
	if location == "" || !strings.Contains(location, "spotify-id") {
		t.Fatalf("expected redirect location containing client id, got %s", location)
	}
}
