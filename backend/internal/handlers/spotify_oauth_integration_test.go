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

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "oauth_test.db")

	db, err := sqliteconn.OpenWithPragmas(dbPath)
	require.NoError(t, err)

	require.NoError(t, migrate.Up(db))

	cleanup := func() {
		migrate.Down(db)
		db.Close()
	}
	return db, cleanup
}

func TestSpotifyOAuthFullFlow(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// Mock Spotify token exchange
	httpmock.RegisterResponder("POST", "https://accounts.spotify.com/api/token",
		httpmock.NewStringResponder(200, `{
			"access_token": "test-access-token",
			"refresh_token": "test-refresh-token",
			"expires_in": 3600,
			"scope": "playlist-read-private playlist-modify-private"
		}`))

	// Mock Spotify playlists API
	httpmock.RegisterResponder("GET", "https://api.spotify.com/v1/me/playlists",
		httpmock.NewStringResponder(200, `{
			"items": [
				{"id": "playlist1", "name": "My Playlist 1"},
				{"id": "playlist2", "name": "My Playlist 2"}
			]
		}`))

	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Setup settings with test credentials
	_, err := db.Exec(`INSERT INTO settings (id, spotify_client_id, spotify_client_secret, created, updated) 
		VALUES ('1', 'test-client-id', 'test-client-secret', ?, ?)`, time.Now().Unix(), time.Now().Unix())
	require.NoError(t, err)

	store := sessions.NewCookieStore([]byte("test-session-key"))
	tokenRepo := auth.NewSQLiteTokenRepository(db)
	settingsRepo := &testSettingsRepo{db: db}

	handler := NewSpotifyOAuthHandler(settingsRepo, tokenRepo, store, "http://localhost:8090/callback", "http://localhost:5173")

	e := echo.New()
	RegisterSpotifyRoutes(e.Group("/api/auth/spotify"), handler)

	// Step 1: Test login (generates state, redirects)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/spotify/login", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	location := rec.Header().Get("Location")
	assert.Contains(t, location, "accounts.spotify.com/authorize")
	assert.Contains(t, location, "test-client-id")

	// Extract session cookie for next request
	cookies := rec.Header()["Set-Cookie"]
	require.NotEmpty(t, cookies)

	// Step 2: Test callback (validates state, exchanges token)
	req = httptest.NewRequest(http.MethodGet, "/api/auth/spotify/callback?code=test-code&state=", nil)
	for _, cookie := range cookies {
		req.Header.Add("Cookie", cookie)
	}

	// Extract state from redirect URL for callback
	u, err := url.Parse(location)
	require.NoError(t, err)
	state := u.Query().Get("state")
	req.URL.RawQuery = "code=test-code&state=" + state

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Contains(t, rec.Header().Get("Location"), "http://localhost:5173/dashboard")
	assert.Contains(t, rec.Header().Get("Location"), "spotify=connected")

	// Verify token was stored in database
	var tokenCount int
	err = db.QueryRow("SELECT COUNT(*) FROM oauth_tokens WHERE provider = 'spotify'").Scan(&tokenCount)
	require.NoError(t, err)
	assert.Equal(t, 1, tokenCount)

	// Step 3: Test playlists endpoint
	req = httptest.NewRequest(http.MethodGet, "/api/auth/spotify/playlists", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var playlists []SpotifyPlaylist
	err = json.Unmarshal(rec.Body.Bytes(), &playlists)
	require.NoError(t, err)
	assert.Len(t, playlists, 2)
	assert.Equal(t, "playlist1", playlists[0].ID)
	assert.Equal(t, "My Playlist 1", playlists[0].Name)
}

func TestSpotifyCallbackStateMismatch(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	store := sessions.NewCookieStore([]byte("test-key"))
	handler := &SpotifyOAuthHandler{
		Repo:         &testSettingsRepo{db: db},
		TokenRepo:    auth.NewSQLiteTokenRepository(db),
		SessionStore: store,
		RedirectURI:  "http://localhost:8090/callback",
		FrontendURL:  "http://localhost:5173",
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/spotify/callback?code=test&state=wrong-state", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := handler.Callback(ctx)
	require.NoError(t, err)
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Contains(t, rec.Header().Get("Location"), "http://localhost:5173/dashboard")
	assert.Contains(t, rec.Header().Get("Location"), "spotify=error")
	assert.Contains(t, rec.Header().Get("Location"), "message=state+mismatch")
}

func TestSpotifyTokenExchangeFailure(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// Mock failed token exchange
	httpmock.RegisterResponder("POST", "https://accounts.spotify.com/api/token",
		httpmock.NewStringResponder(400, `{"error": "invalid_grant"}`))

	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Setup session with state
	store := sessions.NewCookieStore([]byte("test-key"))
	tokenRepo := auth.NewSQLiteTokenRepository(db)
	handler := &SpotifyOAuthHandler{
		Repo:         &testSettingsRepo{db: db},
		TokenRepo:    tokenRepo,
		SessionStore: store,
		RedirectURI:  "http://localhost:8090/callback",
		FrontendURL:  "http://localhost:5173",
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/spotify/callback", nil)
	rec := httptest.NewRecorder()

	// Simulate session with state
	session, _ := store.New(req, spotifySessionName)
	session.Values[sessionStateKey] = "test-state"
	session.Values[sessionVerifierKey] = "test-verifier"
	session.Save(req, rec)

	// Make callback request with matching state
	req = httptest.NewRequest(http.MethodGet, "/api/auth/spotify/callback?code=test&state=test-state", nil)
	req.Header.Set("Cookie", rec.Header().Get("Set-Cookie"))
	rec = httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := handler.Callback(ctx)
	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadGateway, httpErr.Code)
}

type testSettingsRepo struct {
	db *sql.DB
}

func (r *testSettingsRepo) GetSettings() (*auth.SettingsRecord, error) {
	row := r.db.QueryRow(`SELECT spotify_client_id, spotify_client_secret, google_client_id, google_client_secret FROM settings WHERE id = '1'`)

	var record auth.SettingsRecord
	err := row.Scan(&record.SpotifyClientID, &record.SpotifyClientSecret, &record.GoogleClientID, &record.GoogleClientSecret)
	if err != nil {
		if err == sql.ErrNoRows {
			return &auth.SettingsRecord{
				SpotifyClientID:     sql.NullString{String: "test-client-id", Valid: true},
				SpotifyClientSecret: sql.NullString{String: "test-client-secret", Valid: true},
			}, nil
		}
		return nil, err
	}

	return &record, nil
}
