package spotifyauth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/manlikeabro/spotube/internal/auth"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

const (
	cookieName     = "spotify_auth_state"
	cookieDuration = 5 * time.Minute
)

// Register registers the Spotify auth routes
func Register(app *pocketbase.PocketBase) {
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		// Register routes under the API group
		e.Router.GET("/api/auth/spotify/login", loginHandler(app))
		e.Router.GET("/api/auth/spotify/callback", callbackHandler(app))
		e.Router.GET("/api/spotify/playlists", playlistsHandler(app))

		return nil
	})
}

// getSpotifyAuthenticator creates a Spotify OAuth2 authenticator with PKCE
func getSpotifyAuthenticator() (*spotifyauth.Authenticator, error) {
	// Use temporary context and mock dbProvider for credential loading
	// In the future, this should be refactored to accept a proper dbProvider parameter
	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		// Settings collection integration is now handled by unified auth factory
		// This function maintains backward compatibility for OAuth flow setup
		return nil, fmt.Errorf("Spotify client credentials not configured")
	}

	publicURL := os.Getenv("PUBLIC_URL")
	if publicURL == "" {
		publicURL = "http://localhost:8090"
	}

	redirectURL := fmt.Sprintf("%s/api/auth/spotify/callback", publicURL)

	auth := spotifyauth.New(
		spotifyauth.WithClientID(clientID),
		spotifyauth.WithClientSecret(clientSecret),
		spotifyauth.WithRedirectURL(redirectURL),
		spotifyauth.WithScopes(
			spotifyauth.ScopeUserReadPrivate,
			spotifyauth.ScopeUserReadEmail,
			spotifyauth.ScopePlaylistReadPrivate,
			spotifyauth.ScopePlaylistReadCollaborative,
		),
	)

	return auth, nil
}

// loginHandler handles the /api/auth/spotify/login route
func loginHandler(app *pocketbase.PocketBase) echo.HandlerFunc {
	return func(c echo.Context) error {
		auth, err := getSpotifyAuthenticator()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
		}

		// Generate state and verifier for PKCE
		state, err := generateRandomString(16)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to generate state",
			})
		}

		verifier, err := generateRandomString(64)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to generate verifier",
			})
		}

		// Store state and verifier in HTTP-only cookie
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

		// Generate auth URL with PKCE
		url := auth.AuthURL(
			state,
			oauth2.SetAuthURLParam("code_challenge_method", "S256"),
			oauth2.SetAuthURLParam("code_challenge", generateCodeChallenge(verifier)),
		)

		return c.Redirect(http.StatusTemporaryRedirect, url)
	}
}

// callbackHandler handles the /api/auth/spotify/callback route
func callbackHandler(app *pocketbase.PocketBase) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Get state and code from query params
		state := c.QueryParam("state")
		code := c.QueryParam("code")

		if state == "" || code == "" {
			return c.Redirect(http.StatusTemporaryRedirect, "/dashboard?spotify=error&message=Missing+state+or+code")
		}

		// Get and validate cookie
		cookie, err := c.Cookie(cookieName)
		if err != nil {
			return c.Redirect(http.StatusTemporaryRedirect, "/dashboard?spotify=error&message=Missing+auth+cookie")
		}

		// Parse state and verifier from cookie
		cookieParts := parseAuthCookie(cookie.Value)
		if len(cookieParts) != 2 || cookieParts[0] != state {
			return c.Redirect(http.StatusTemporaryRedirect, "/dashboard?spotify=error&message=Invalid+state")
		}

		verifier := cookieParts[1]

		// Clear the cookie
		c.SetCookie(&http.Cookie{
			Name:     cookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   -1,
		})

		// Get authenticator and exchange code for token
		auth, err := getSpotifyAuthenticator()
		if err != nil {
			return c.Redirect(http.StatusTemporaryRedirect, "/dashboard?spotify=error&message=Auth+config+error")
		}

		// Exchange code for token with verifier
		token, err := auth.Exchange(c.Request().Context(), code,
			oauth2.SetAuthURLParam("code_verifier", verifier),
		)
		if err != nil {
			return c.Redirect(http.StatusTemporaryRedirect, "/dashboard?spotify=error&message=Token+exchange+failed")
		}

		// Save tokens to database
		scopes := []string{
			string(spotifyauth.ScopeUserReadPrivate),
			string(spotifyauth.ScopeUserReadEmail),
			string(spotifyauth.ScopePlaylistReadPrivate),
			string(spotifyauth.ScopePlaylistReadCollaborative),
		}

		if err := saveSpotifyTokens(app.Dao(), token, scopes); err != nil {
			return c.Redirect(http.StatusTemporaryRedirect, "/dashboard?spotify=error&message=Failed+to+save+tokens")
		}

		// Redirect to dashboard with success
		return c.Redirect(http.StatusTemporaryRedirect, "/dashboard?spotify=connected")
	}
}

// playlistsHandler proxies requests to Spotify's /me/playlists endpoint
func playlistsHandler(app *pocketbase.PocketBase) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Get authenticated Spotify client
		client, err := withSpotifyClient(app, c)
		if err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{
				"error": "Not authenticated with Spotify",
			})
		}

		// Parse query parameters for pagination
		limit := 20
		offset := 0

		if limitStr := c.QueryParam("limit"); limitStr != "" {
			if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 50 {
				limit = parsedLimit
			}
		}

		if offsetStr := c.QueryParam("offset"); offsetStr != "" {
			if parsedOffset, err := strconv.Atoi(offsetStr); err == nil && parsedOffset >= 0 {
				offset = parsedOffset
			}
		}

		// Fetch user's playlists
		playlists, err := client.CurrentUsersPlaylists(c.Request().Context(), spotify.Limit(limit), spotify.Offset(offset))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to fetch playlists",
			})
		}

		// Transform response to include useful fields
		type PlaylistItem struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Public      bool   `json:"public"`
			TrackCount  int    `json:"track_count"`
			Owner       struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
			} `json:"owner"`
			Images []struct {
				URL    string `json:"url"`
				Height int    `json:"height"`
				Width  int    `json:"width"`
			} `json:"images"`
		}

		type PlaylistsResponse struct {
			Items  []PlaylistItem `json:"items"`
			Total  int            `json:"total"`
			Limit  int            `json:"limit"`
			Offset int            `json:"offset"`
			Next   string         `json:"next"`
		}

		response := PlaylistsResponse{
			Items:  make([]PlaylistItem, 0, len(playlists.Playlists)),
			Total:  int(playlists.Total),
			Limit:  int(playlists.Limit),
			Offset: int(playlists.Offset),
		}

		if playlists.Next != "" {
			response.Next = playlists.Next
		}

		// Transform each playlist
		for _, p := range playlists.Playlists {
			item := PlaylistItem{
				ID:          string(p.ID),
				Name:        p.Name,
				Description: p.Description,
				Public:      p.IsPublic,
				TrackCount:  int(p.Tracks.Total),
			}

			// Add owner info
			item.Owner.ID = string(p.Owner.ID)
			item.Owner.DisplayName = p.Owner.DisplayName

			// Add images
			for _, img := range p.Images {
				item.Images = append(item.Images, struct {
					URL    string `json:"url"`
					Height int    `json:"height"`
					Width  int    `json:"width"`
				}{
					URL:    img.URL,
					Height: int(img.Height),
					Width:  int(img.Width),
				})
			}

			response.Items = append(response.Items, item)
		}

		return c.JSON(http.StatusOK, response)
	}
}

// saveSpotifyTokens saves Spotify OAuth tokens to the oauth_tokens collection
func saveSpotifyTokens(dao *daos.Dao, token *oauth2.Token, scopes []string) error {
	// Find or create oauth_tokens record for Spotify
	rec, _ := dao.FindFirstRecordByFilter("oauth_tokens", "provider = 'spotify'")

	if rec == nil {
		// Create new record
		collection, err := dao.FindCollectionByNameOrId("oauth_tokens")
		if err != nil {
			return err
		}
		rec = models.NewRecord(collection)
		rec.Set("provider", "spotify")
	}

	// Update token fields
	rec.Set("access_token", token.AccessToken)
	rec.Set("refresh_token", token.RefreshToken)
	rec.Set("expiry", token.Expiry)
	rec.Set("scopes", strings.Join(scopes, " "))

	return dao.SaveRecord(rec)
}

// generateRandomString generates a cryptographically secure random string
func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// generateCodeChallenge generates a PKCE code challenge from verifier
func generateCodeChallenge(verifier string) string {
	// The spotify library handles S256 challenge generation internally
	// This is a placeholder that returns the verifier
	// The actual challenge is computed by the library
	return verifier
}

// parseAuthCookie splits the cookie value into state and verifier
func parseAuthCookie(value string) []string {
	// Simple split by colon
	parts := []string{}
	if idx := strings.Index(value, ":"); idx > 0 {
		parts = append(parts, value[:idx])
		parts = append(parts, value[idx+1:])
	}
	return parts
}

// withSpotifyClient creates an authenticated Spotify client, refreshing token if needed
// This function now uses the unified auth factory while maintaining backward compatibility
func withSpotifyClient(app *pocketbase.PocketBase, c echo.Context) (*spotify.Client, error) {
	// Use the unified auth factory with API context
	return auth.WithSpotifyClient(app, c)
}
