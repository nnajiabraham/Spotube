package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/youtube/v3"

	localauth "github.com/manlikeabro/spotube/internal/auth"
)

const (
	youtubeSessionName = "youtube_oauth"
)

type YouTubeOAuthHandler struct {
	Repo         localauth.CredentialProvider
	TokenRepo    localauth.TokenRepository
	SessionStore sessions.Store
	RedirectURI  string
	FrontendURL  string
	Scopes       []string
}

func NewYouTubeOAuthHandler(repo localauth.CredentialProvider, tokenRepo localauth.TokenRepository, store sessions.Store, redirectURI, frontendURL string) *YouTubeOAuthHandler {
	return &YouTubeOAuthHandler{
		Repo:         repo,
		TokenRepo:    tokenRepo,
		SessionStore: store,
		RedirectURI:  redirectURI,
		FrontendURL:  frontendURL,
		Scopes: []string{
			youtube.YoutubeScope,
			youtube.YoutubeReadonlyScope,
		},
	}
}

func RegisterYouTubeRoutes(group *echo.Group, handler *YouTubeOAuthHandler) {
	group.GET("/login", handler.Login)
	group.GET("/callback", handler.Callback)
	group.GET("/playlists", handler.ListPlaylists)
}

func (h *YouTubeOAuthHandler) Login(c echo.Context) error {
	state, err := generateState()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create state")
	}

	clientID, clientSecret, err := localauth.LoadCredentials(h.Repo, "google")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  h.RedirectURI,
		Scopes:       h.Scopes,
		Endpoint:     google.Endpoint,
	}

	session, err := h.SessionStore.Get(c.Request(), youtubeSessionName)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create session")
	}

	session.Values[sessionStateKey] = state
	session.Values[sessionCreatedAtKey] = time.Now().Unix()
	if err := session.Save(c.Request(), c.Response()); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to persist session")
	}

	authURL := config.AuthCodeURL(state, oauth2.AccessTypeOffline)
	return c.Redirect(http.StatusFound, authURL)
}

func (h *YouTubeOAuthHandler) Callback(c echo.Context) error {
	session, err := h.SessionStore.Get(c.Request(), youtubeSessionName)
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

	clientID, clientSecret, err := localauth.LoadCredentials(h.Repo, "google")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  h.RedirectURI,
		Scopes:       h.Scopes,
		Endpoint:     google.Endpoint,
	}

	oauthToken, err := config.Exchange(c.Request().Context(), code)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "token exchange failed")
	}

	token := &localauth.Token{
		AccessToken:  sqlNullString(oauthToken.AccessToken),
		RefreshToken: sqlNullString(oauthToken.RefreshToken),
		Expiry:       sqlNullInt64(oauthToken.Expiry.Unix()),
		Scopes:       sqlNullString(strings.Join(h.Scopes, " ")),
	}

	if err := h.TokenRepo.UpsertToken("google", *token); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to persist token")
	}

	session.Options.MaxAge = -1
	_ = session.Save(c.Request(), c.Response())

	return c.Redirect(http.StatusFound, h.FrontendURL+"/dashboard?youtube=connected")
}

func (h *YouTubeOAuthHandler) ListPlaylists(c echo.Context) error {
	token, err := h.TokenRepo.GetToken("google")
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err.Error())
	}

	if token == nil || !token.AccessToken.Valid {
		return echo.NewHTTPError(http.StatusUnauthorized, "youtube account not connected")
	}

	playlists, err := fetchYouTubePlaylists(c.Request().Context(), token, h.Repo, h.TokenRepo)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "failed to fetch playlists")
	}

	return c.JSON(http.StatusOK, playlists)
}

func fetchYouTubePlaylists(ctx context.Context, token *localauth.Token, creds localauth.CredentialProvider, repo localauth.TokenRepository) ([]YouTubePlaylist, error) {
	if !token.AccessToken.Valid || token.AccessToken.String == "" {
		return nil, errors.New("invalid access token")
	}

	clientID, clientSecret, err := localauth.LoadCredentials(creds, "google")
	if err != nil {
		return nil, err
	}

	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
	}

	oauthToken := &oauth2.Token{
		AccessToken:  token.AccessToken.String,
		RefreshToken: token.RefreshToken.String,
		Expiry:       time.Unix(token.Expiry.Int64, 0),
	}

	httpClient := config.Client(ctx, oauthToken)
	service, err := youtube.New(httpClient)
	if err != nil {
		return nil, err
	}

	call := service.Playlists.List([]string{"id", "snippet", "contentDetails"}).Mine(true).MaxResults(50)
	response, err := call.Do()
	if err != nil {
		return nil, err
	}

	result := make([]YouTubePlaylist, len(response.Items))
	for i, item := range response.Items {
		var itemCount int64
		if item.ContentDetails != nil {
			itemCount = item.ContentDetails.ItemCount
		}
		var description string
		if item.Snippet != nil {
			description = item.Snippet.Description
		}
		result[i] = YouTubePlaylist{
			ID:          item.Id,
			Name:        item.Snippet.Title,
			Title:       item.Snippet.Title,
			Description: description,
			ItemCount:   itemCount,
		}
	}

	return result, nil
}

type YouTubePlaylist struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ItemCount   int64  `json:"itemCount"`
}
