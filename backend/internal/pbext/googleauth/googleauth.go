package googleauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
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
)

// Use unified YouTube scopes from auth package to eliminate duplication (RFC-010 BF1)
// This ensures consistent scopes across the entire application

func Register(app *pocketbase.PocketBase) {
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.GET("/api/auth/google/login", loginHandler(app))
		e.Router.GET("/api/auth/google/callback", callbackHandler(app))
		e.Router.GET("/api/youtube/playlists", playlistsHandler(app))
		return nil
	})
}

func getGoogleOAuthConfig(dbProvider auth.DatabaseProvider) (*oauth2.Config, error) {
	// Use unified auth system to load credentials from settings collection with env fallback
	clientID, clientSecret, err := auth.LoadCredentialsFromSettings(dbProvider, "google")
	if err != nil {
		return nil, fmt.Errorf("Google client credentials not configured: %w", err)
	}

	// For OAuth callbacks, we need the backend URL, not the frontend URL
	// In development: backend=8090, frontend=5173
	// The PUBLIC_URL might point to frontend, but OAuth callbacks must go to backend
	publicURL := os.Getenv("PUBLIC_URL")
	if publicURL == "" {
		publicURL = "http://localhost:8090"
	}

	// If PUBLIC_URL points to frontend (port 5173), adjust it to backend (port 8090)
	if strings.Contains(publicURL, ":5173") {
		publicURL = strings.Replace(publicURL, ":5173", ":8090", 1)
	}

	redirectURL := fmt.Sprintf("%s/api/auth/google/callback", publicURL)

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       auth.YouTubeScopes,
		Endpoint:     google.Endpoint,
	}, nil
}

func loginHandler(app daoProvider) echo.HandlerFunc {
	return func(c echo.Context) error {
		conf, err := getGoogleOAuthConfig(app)
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

		url := conf.AuthCodeURL(state,
			oauth2.AccessTypeOffline,
			oauth2.S256ChallengeOption(verifier),
			oauth2.SetAuthURLParam("prompt", "consent"))
		return c.Redirect(http.StatusTemporaryRedirect, url)
	}
}

func callbackHandler(app daoProvider) echo.HandlerFunc {
	return func(c echo.Context) error {
		state := c.QueryParam("state")
		code := c.QueryParam("code")
		errorParam := c.QueryParam("error")

		// Get frontend URL for redirects
		frontendURL := getFrontendURL()

		if errorParam != "" {
			return c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/dashboard?youtube=error&message=%s", frontendURL, errorParam))
		}
		if state == "" || code == "" {
			return c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/dashboard?youtube=error&message=Missing+state+or+code", frontendURL))
		}

		cookie, err := c.Cookie(cookieName)
		if err != nil {
			return c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/dashboard?youtube=error&message=Missing+auth+cookie", frontendURL))
		}

		parts := strings.Split(cookie.Value, ":")
		if len(parts) != 2 || parts[0] != state {
			return c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/dashboard?youtube=error&message=Invalid+state", frontendURL))
		}
		verifier := parts[1]

		c.SetCookie(&http.Cookie{Name: cookieName, Value: "", Path: "/", HttpOnly: true, MaxAge: -1})

		conf, err := getGoogleOAuthConfig(app)
		if err != nil {
			return c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/dashboard?youtube=error&message=Auth+config+error", frontendURL))
		}

		token, err := conf.Exchange(c.Request().Context(), code, oauth2.VerifierOption(verifier))
		if err != nil {
			return c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/dashboard?youtube=error&message=Token+exchange+failed", frontendURL))
		}

		if err := saveGoogleTokens(app.Dao(), token); err != nil {
			return c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/dashboard?youtube=error&message=Failed+to+save+tokens", frontendURL))
		}

		return c.Redirect(http.StatusTemporaryRedirect, fmt.Sprintf("%s/dashboard?youtube=connected", frontendURL))
	}
}

// getFrontendURL returns the frontend URL for redirects after OAuth
func getFrontendURL() string {
	// In development, frontend runs on different port than backend
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		// Default to development frontend URL
		frontendURL = "http://localhost:5173"
	}
	return frontendURL
}

func playlistsHandler(app daoProvider) echo.HandlerFunc {
	return func(c echo.Context) error {
		log.Println("YouTube playlists request received")

		// Use direct authentication like Spotify does - no complex indirection
		svc, err := WithGoogleClient(app, c)
		if err != nil {
			log.Printf("Failed to create YouTube client: %v", err)
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
		}

		log.Println("Successfully created YouTube client, fetching playlists...")

		call := svc.Playlists.List([]string{"id", "snippet", "contentDetails"}).Mine(true).MaxResults(50)
		resp, err := call.Do()
		if err != nil {
			log.Printf("YouTube API call failed: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch playlists from YouTube"})
		}

		log.Printf("Successfully fetched %d playlists from YouTube", len(resp.Items))

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

// WithGoogleClient creates an authenticated YouTube service for API handlers
// This is a thin adapter that delegates to the unified auth system
func WithGoogleClient(app daoProvider, c echo.Context) (*youtube.Service, error) {
	// Use the unified auth system with API context
	authCtx := auth.NewAPIAuthContext(c, app)
	return auth.GetYouTubeService(c.Request().Context(), app, authCtx)
}

func saveGoogleTokens(dao *daos.Dao, token *oauth2.Token) error {
	log.Printf("Saving Google tokens - AccessToken: %s..., RefreshToken: %s, Expiry: %v",
		token.AccessToken[:min(10, len(token.AccessToken))],
		token.RefreshToken,
		token.Expiry)

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
	rec.Set("scopes", strings.Join(auth.YouTubeScopes, ","))

	err = dao.SaveRecord(rec)
	if err != nil {
		log.Printf("Failed to save Google tokens: %v", err)
		return err
	}

	log.Printf("Successfully saved Google tokens")
	return nil
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
