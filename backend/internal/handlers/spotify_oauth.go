package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	spotify "github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"

	localauth "github.com/manlikeabro/spotube/internal/auth"
)

const (
	spotifySessionName  = "spotify_oauth"
	sessionStateKey     = "state"
	sessionVerifierKey  = "verifier"
	sessionCreatedAtKey = "created_at"
)

type SpotifyOAuthHandler struct {
	Repo         localauth.CredentialProvider
	TokenRepo    localauth.TokenRepository
	SessionStore sessions.Store
	RedirectURI  string
	FrontendURL  string
	Scopes       []string
}

func NewSpotifyOAuthHandler(repo localauth.CredentialProvider, tokenRepo localauth.TokenRepository, store sessions.Store, redirectURI, frontendURL string) *SpotifyOAuthHandler {
	return &SpotifyOAuthHandler{
		Repo:         repo,
		TokenRepo:    tokenRepo,
		SessionStore: store,
		RedirectURI:  redirectURI,
		FrontendURL:  frontendURL,
		Scopes: []string{
			"playlist-read-private",
			"playlist-modify-private",
		},
	}
}

func RegisterSpotifyRoutes(group *echo.Group, handler *SpotifyOAuthHandler) {
	group.GET("/login", handler.Login)
	group.GET("/callback", handler.Callback)
	group.GET("/playlists", handler.ListPlaylists)
}

func (h *SpotifyOAuthHandler) Login(c echo.Context) error {
	verifier, err := localauth.GenerateCodeVerifier()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create verifier")
	}

	challenge := localauth.CodeChallenge(verifier)
	state, err := generateState()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create state")
	}

	clientID, _, err := localauth.LoadCredentials(h.Repo, "spotify")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	session, err := h.SessionStore.Get(c.Request(), spotifySessionName)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create session")
	}

	session.Values[sessionStateKey] = state
	session.Values[sessionVerifierKey] = verifier
	session.Values[sessionCreatedAtKey] = time.Now().Unix()
	if err := session.Save(c.Request(), c.Response()); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to persist session")
	}

	redirectURL := buildSpotifyAuthURL(clientID, h.RedirectURI, state, challenge, h.Scopes)
	return c.Redirect(http.StatusFound, redirectURL)
}

func (h *SpotifyOAuthHandler) Callback(c echo.Context) error {
	session, err := h.SessionStore.Get(c.Request(), spotifySessionName)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid session")
	}

	expectedState, _ := session.Values[sessionStateKey].(string)
	if expectedState == "" || expectedState != c.QueryParam("state") {
		return echo.NewHTTPError(http.StatusUnauthorized, "state mismatch")
	}

	code := c.QueryParam("code")
	if code == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing code")
	}

	verifier, _ := session.Values[sessionVerifierKey].(string)
	if verifier == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing verifier")
	}

	clientID, clientSecret, err := localauth.LoadCredentials(h.Repo, "spotify")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	token, err := exchangeSpotifyCode(c.Request().Context(), code, verifier, clientID, clientSecret, h.RedirectURI)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "token exchange failed")
	}

	if err := h.TokenRepo.UpsertToken("spotify", *token); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to persist token")
	}

	session.Options.MaxAge = -1
	_ = session.Save(c.Request(), c.Response())

	return c.Redirect(http.StatusFound, buildFrontendDashboardRedirect(h.FrontendURL, "spotify", "connected", ""))
}

func (h *SpotifyOAuthHandler) ListPlaylists(c echo.Context) error {
	token, err := h.TokenRepo.GetToken("spotify")
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}

	if token == nil || !token.AccessToken.Valid {
		return echo.NewHTTPError(http.StatusUnauthorized, "spotify account not connected")
	}

	playlists, err := fetchSpotifyPlaylists(c.Request().Context(), token, h.Repo, h.TokenRepo)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "failed to fetch playlists")
	}

	return c.JSON(http.StatusOK, playlists)
}

func generateState() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func buildSpotifyAuthURL(clientID, redirectURI, state, challenge string, scopes []string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	if len(scopes) > 0 {
		q.Set("scope", strings.Join(scopes, " "))
	}
	return spotifyauth.AuthURL + "?" + q.Encode()
}

func exchangeSpotifyCode(ctx context.Context, code, verifier, clientID, clientSecret, redirectURI string) (*localauth.Token, error) {
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURI,
		Scopes: []string{
			"playlist-read-private",
			"playlist-modify-private",
		},
		Endpoint: oauth2.Endpoint{
			AuthURL:  spotifyauth.AuthURL,
			TokenURL: spotifyauth.TokenURL,
		},
	}

	oauthToken, err := config.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		return nil, err
	}

	scopes := ""
	if len(oauthToken.Extra("scope").(string)) > 0 {
		scopes = oauthToken.Extra("scope").(string)
	}

	return &localauth.Token{
		AccessToken:  sqlNullString(oauthToken.AccessToken),
		RefreshToken: sqlNullString(oauthToken.RefreshToken),
		Expiry:       sqlNullInt64(oauthToken.Expiry.Unix()),
		Scopes:       sqlNullString(scopes),
	}, nil
}

func fetchSpotifyPlaylists(ctx context.Context, token *localauth.Token, creds localauth.CredentialProvider, repo localauth.TokenRepository) ([]SpotifyPlaylist, error) {
	if !token.AccessToken.Valid || token.AccessToken.String == "" {
		return nil, errors.New("invalid access token")
	}

	oauthToken := &oauth2.Token{
		AccessToken:  token.AccessToken.String,
		RefreshToken: token.RefreshToken.String,
		Expiry:       time.Unix(token.Expiry.Int64, 0),
	}

	clientID, clientSecret, err := localauth.LoadCredentials(creds, "spotify")
	if err != nil {
		return nil, err
	}

	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  spotifyauth.AuthURL,
			TokenURL: spotifyauth.TokenURL,
		},
	}

	httpClient := config.Client(ctx, oauthToken)
	client := spotify.New(httpClient)

	playlists, err := client.CurrentUsersPlaylists(ctx, spotify.Limit(50))
	if err != nil {
		return nil, err
	}

	result := make([]SpotifyPlaylist, len(playlists.Playlists))
	for i, p := range playlists.Playlists {
		images := make([]SpotifyPlaylistImage, 0, len(p.Images))
		for _, image := range p.Images {
			images = append(images, SpotifyPlaylistImage{
				URL:    image.URL,
				Width:  int(image.Width),
				Height: int(image.Height),
			})
		}

		result[i] = SpotifyPlaylist{
			ID:          p.ID.String(),
			Name:        p.Name,
			Description: p.Description,
			Images:      images,
			TrackCount:  int(p.Tracks.Total),
			Public:      p.IsPublic,
			Owner: SpotifyPlaylistOwner{
				ID:          p.Owner.ID,
				DisplayName: p.Owner.DisplayName,
			},
		}
	}

	return result, nil
}

type SpotifyPlaylist struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Images      []SpotifyPlaylistImage `json:"images,omitempty"`
	TrackCount  int                    `json:"track_count"`
	Public      bool                   `json:"public"`
	Owner       SpotifyPlaylistOwner   `json:"owner"`
}

type SpotifyPlaylistImage struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type SpotifyPlaylistOwner struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

func sqlNullString(value string) sql.NullString {
	if strings.TrimSpace(value) == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: value, Valid: true}
}

func sqlNullInt64(value int64) sql.NullInt64 {
	return sql.NullInt64{Int64: value, Valid: true}
}
