package googleauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/manlikeabro/spotube/internal/auth"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/youtube/v3"
)

// daoProvider is an interface that matches the methods we need from pocketbase.PocketBase
// to allow for easier testing.
type daoProvider interface {
	Dao() *daos.Dao
}

const (
	cookieName     = "google_auth_state"
	cookieDuration = 5 * time.Minute
	scope          = youtube.YoutubeReadonlyScope
)

func Register(app *pocketbase.PocketBase) {
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.GET("/api/auth/google/login", loginHandler(app))
		e.Router.GET("/api/auth/google/callback", callbackHandler(app))
		e.Router.GET("/api/youtube/playlists", playlistsHandler(app))
		return nil
	})
}

func getGoogleOAuthConfig() (*oauth2.Config, error) {
	// Settings collection integration is now handled by unified auth factory
	// This function maintains backward compatibility for OAuth flow setup
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("Google client credentials not configured")
	}

	publicURL := os.Getenv("PUBLIC_URL")
	if publicURL == "" {
		publicURL = "http://localhost:8090"
	}
	redirectURL := fmt.Sprintf("%s/api/auth/google/callback", publicURL)

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{scope},
		Endpoint:     google.Endpoint,
	}, nil
}

func loginHandler(app daoProvider) echo.HandlerFunc {
	return func(c echo.Context) error {
		conf, err := getGoogleOAuthConfig()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		state, err := generateRandomString(16)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate state"})
		}

		verifier := oauth2.GenerateVerifier()

		cookieValue := fmt.Sprintf("%s:%s", state, verifier)
		c.SetCookie(&http.Cookie{
			Name:     cookieName,
			Value:    cookieValue,
			Path:     "/",
			HttpOnly: true,
			Secure:   c.Request().URL.Scheme == "https",
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(cookieDuration),
		})

		url := conf.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier))
		return c.Redirect(http.StatusTemporaryRedirect, url)
	}
}

func callbackHandler(app daoProvider) echo.HandlerFunc {
	return func(c echo.Context) error {
		state := c.QueryParam("state")
		code := c.QueryParam("code")
		errorParam := c.QueryParam("error")

		if errorParam != "" {
			return c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/dashboard?youtube=error&message=%s", errorParam))
		}
		if state == "" || code == "" {
			return c.Redirect(http.StatusTemporaryRedirect, "/dashboard?youtube=error&message=Missing+state+or+code")
		}

		cookie, err := c.Cookie(cookieName)
		if err != nil {
			return c.Redirect(http.StatusTemporaryRedirect, "/dashboard?youtube=error&message=Missing+auth+cookie")
		}

		parts := strings.Split(cookie.Value, ":")
		if len(parts) != 2 || parts[0] != state {
			return c.Redirect(http.StatusTemporaryRedirect, "/dashboard?youtube=error&message=Invalid+state")
		}
		verifier := parts[1]

		c.SetCookie(&http.Cookie{Name: cookieName, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})

		conf, err := getGoogleOAuthConfig()
		if err != nil {
			return c.Redirect(http.StatusTemporaryRedirect, "/dashboard?youtube=error&message=Auth+config+error")
		}

		token, err := conf.Exchange(c.Request().Context(), code, oauth2.VerifierOption(verifier))
		if err != nil {
			return c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/dashboard?youtube=error&message=Token+exchange+failed: %v", err))
		}

		if err := saveGoogleTokens(app.Dao(), token); err != nil {
			return c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("/dashboard?youtube=error&message=Failed+to+save+tokens: %v", err))
		}

		return c.Redirect(http.StatusTemporaryRedirect, "/dashboard?youtube=connected")
	}
}

func playlistsHandler(app daoProvider) echo.HandlerFunc {
	return playlistsHandlerWithClient(app, nil)
}

func playlistsHandlerWithClient(app daoProvider, httpClient *http.Client) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc, err := withGoogleClientCustom(c.Request().Context(), app, httpClient)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
		}

		call := svc.Playlists.List([]string{"id", "snippet", "contentDetails"}).Mine(true).MaxResults(50)
		resp, err := call.Do()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch playlists from YouTube"})
		}

		type PlaylistItem struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			ItemCount   uint64 `json:"itemCount"`
			Description string `json:"description"`
		}

		items := make([]PlaylistItem, 0, len(resp.Items))
		for _, item := range resp.Items {
			items = append(items, PlaylistItem{
				ID:          item.Id,
				Title:       item.Snippet.Title,
				ItemCount:   uint64(item.ContentDetails.ItemCount),
				Description: item.Snippet.Description,
			})
		}

		return c.JSON(http.StatusOK, map[string]any{"items": items})
	}
}

func saveGoogleTokens(dao *daos.Dao, token *oauth2.Token) error {
	collection, err := dao.FindCollectionByNameOrId("oauth_tokens")
	if err != nil {
		return err
	}

	rec, _ := dao.FindFirstRecordByFilter(collection.Name, "provider = 'google'")
	if rec == nil {
		rec = models.NewRecord(collection)
		rec.Set("provider", "google")
	}

	rec.Set("access_token", token.AccessToken)
	rec.Set("refresh_token", token.RefreshToken)
	rec.Set("expiry", token.Expiry)
	rec.Set("scopes", scope)

	return dao.SaveRecord(rec)
}

// withGoogleClient creates an authenticated YouTube service using the unified auth factory
// This function now delegates to the unified factory while maintaining backward compatibility
func withGoogleClient(ctx context.Context, app daoProvider) (*youtube.Service, error) {
	return auth.WithGoogleClient(ctx, app)
}

// withGoogleClientCustom creates an authenticated YouTube service using the unified auth factory with custom HTTP client
// This function now delegates to the unified factory while maintaining backward compatibility
func withGoogleClientCustom(ctx context.Context, app daoProvider, httpClient *http.Client) (*youtube.Service, error) {
	return auth.WithGoogleClientCustom(ctx, app, httpClient)
}

func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

func generateCodeChallenge(verifier string) string {
	h := sha256.New()
	h.Write([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}
