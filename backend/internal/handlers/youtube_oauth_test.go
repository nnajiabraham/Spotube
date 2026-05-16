package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/gorilla/sessions"
	"github.com/jarcoal/httpmock"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/auth"
	"github.com/manlikeabro/spotube/internal/migrate"
	"github.com/manlikeabro/spotube/internal/sqliteconn"
)

func TestYouTubeOAuthFullFlow(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// Mock Google token exchange
	httpmock.RegisterResponder("POST", "https://oauth2.googleapis.com/token",
		httpmock.NewStringResponder(200, `{
			"access_token": "test-youtube-access",
			"refresh_token": "test-youtube-refresh", 
			"expires_in": 3600,
			"scope": "https://www.googleapis.com/auth/youtube"
		}`))

	// Mock YouTube playlists API
	httpmock.RegisterResponder("GET", "https://youtube.googleapis.com/youtube/v3/playlists",
		httpmock.NewStringResponder(200, `{
			"items": [
				{
					"id": "playlist-yt-1",
					"snippet": {"title": "My YouTube Playlist 1"}
				},
				{
					"id": "playlist-yt-2", 
					"snippet": {"title": "My YouTube Playlist 2"}
				}
			]
		}`))

	db, cleanup := setupTestDBYouTube(t)
	defer cleanup()

	// Setup settings with Google credentials
	_, err := db.Exec(`INSERT INTO settings (id, google_client_id, google_client_secret, created, updated)
		VALUES ('1', 'test-google-client', 'test-google-secret', ?, ?)`, time.Now().Unix(), time.Now().Unix())
	require.NoError(t, err)

	store := sessions.NewCookieStore([]byte("test-session-key"))
	tokenRepo := auth.NewSQLiteTokenRepository(db)
	settingsRepo := &testYouTubeSettingsRepo{db: db}

	handler := NewYouTubeOAuthHandler(settingsRepo, tokenRepo, store, "http://localhost:8090/youtube/callback", "http://localhost:5173")

	e := echo.New()
	RegisterYouTubeRoutes(e.Group("/api/auth/youtube"), handler)

	// Step 1: Login redirects to Google
	req := httptest.NewRequest(http.MethodGet, "/api/auth/youtube/login", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	location := rec.Header().Get("Location")
	assert.Contains(t, location, "accounts.google.com/o/oauth2/auth")
	assert.Contains(t, location, "test-google-client")

	cookies := rec.Header()["Set-Cookie"]
	require.NotEmpty(t, cookies)

	// Step 2: Callback with code exchanges for token
	u, err := url.Parse(location)
	require.NoError(t, err)
	state := u.Query().Get("state")

	req = httptest.NewRequest(http.MethodGet, "/api/auth/youtube/callback?code=test-code&state="+state, nil)
	for _, cookie := range cookies {
		req.Header.Add("Cookie", cookie)
	}
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Contains(t, rec.Header().Get("Location"), "http://localhost:5173/dashboard")
	assert.Contains(t, rec.Header().Get("Location"), "youtube=connected")

	// Verify token stored in database
	var tokenCount int
	err = db.QueryRow("SELECT COUNT(*) FROM oauth_tokens WHERE provider = 'google'").Scan(&tokenCount)
	require.NoError(t, err)
	assert.Equal(t, 1, tokenCount)

	// Step 3: Playlists endpoint returns YouTube data
	req = httptest.NewRequest(http.MethodGet, "/api/auth/youtube/playlists", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var playlists []YouTubePlaylist
	err = json.Unmarshal(rec.Body.Bytes(), &playlists)
	require.NoError(t, err)
	assert.Len(t, playlists, 2)
	assert.Equal(t, "playlist-yt-1", playlists[0].ID)
	assert.Equal(t, "My YouTube Playlist 1", playlists[0].Name)
}

func TestYouTubeCallbackStateMismatch(t *testing.T) {
	db, cleanup := setupTestDBYouTube(t)
	defer cleanup()

	store := sessions.NewCookieStore([]byte("test-key"))
	handler := &YouTubeOAuthHandler{
		Repo:         &testYouTubeSettingsRepo{db: db},
		TokenRepo:    auth.NewSQLiteTokenRepository(db),
		SessionStore: store,
		RedirectURI:  "http://localhost:8090/callback",
		FrontendURL:  "http://localhost:5173",
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/youtube/callback?code=test&state=invalid-state", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := handler.Callback(ctx)
	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusUnauthorized, httpErr.Code)
}

func TestYouTubeTokenExchangeFailure(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// Mock failed Google token exchange
	httpmock.RegisterResponder("POST", "https://oauth2.googleapis.com/token",
		httpmock.NewStringResponder(400, `{"error": "invalid_grant"}`))

	db, cleanup := setupTestDBYouTube(t)
	defer cleanup()

	store := sessions.NewCookieStore([]byte("test-key"))
	handler := &YouTubeOAuthHandler{
		Repo:         &testYouTubeSettingsRepo{db: db},
		TokenRepo:    auth.NewSQLiteTokenRepository(db),
		SessionStore: store,
		RedirectURI:  "http://localhost:8090/callback",
		FrontendURL:  "http://localhost:5173",
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/youtube/callback", nil)
	rec := httptest.NewRecorder()

	// Setup session with state
	session, _ := store.New(req, youtubeSessionName)
	session.Values[sessionStateKey] = "test-state"
	session.Save(req, rec)

	req = httptest.NewRequest(http.MethodGet, "/api/auth/youtube/callback?code=invalid-code&state=test-state", nil)
	req.Header.Set("Cookie", rec.Header().Get("Set-Cookie"))
	rec = httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := handler.Callback(ctx)
	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadGateway, httpErr.Code)
}

func setupTestDBYouTube(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "youtube_oauth_test.db")

	db, err := sqliteconn.OpenWithPragmas(dbPath)
	require.NoError(t, err)

	require.NoError(t, migrate.Up(db))

	cleanup := func() {
		migrate.Down(db)
		db.Close()
	}
	return db, cleanup
}

type testYouTubeSettingsRepo struct {
	db *sql.DB
}

func (r *testYouTubeSettingsRepo) GetSettings() (*auth.SettingsRecord, error) {
	row := r.db.QueryRow(`SELECT spotify_client_id, spotify_client_secret, google_client_id, google_client_secret FROM settings WHERE id = '1'`)

	var record auth.SettingsRecord
	err := row.Scan(&record.SpotifyClientID, &record.SpotifyClientSecret, &record.GoogleClientID, &record.GoogleClientSecret)
	if err != nil {
		if err == sql.ErrNoRows {
			return &auth.SettingsRecord{
				GoogleClientID:     sql.NullString{String: "test-google-client", Valid: true},
				GoogleClientSecret: sql.NullString{String: "test-google-secret", Valid: true},
			}, nil
		}
		return nil, err
	}

	return &record, nil
}
