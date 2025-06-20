package spotifyauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
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
func getSpotifyAuthenticator(dbProvider auth.DatabaseProvider) (*spotifyauth.Authenticator, error) {
	// Use unified auth system to load credentials from settings collection with env fallback
	clientID, clientSecret, err := auth.LoadCredentialsFromSettings(dbProvider, "spotify")
	if err != nil {
		return nil, fmt.Errorf("Spotify client credentials not configured: %w", err)
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
		log.Printf("Adjusted OAuth redirect URL from frontend to backend: %s", publicURL)
	}

	redirectURL := fmt.Sprintf("%s/api/auth/spotify/callback", publicURL)
	log.Printf("Using Spotify OAuth redirect URL: %s", redirectURL)

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
		log.Println("Starting Spotify OAuth login flow")

		auth, err := getSpotifyAuthenticator(app)
		if err != nil {
			log.Printf("Failed to create Spotify authenticator: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": err.Error(),
			})
		}

		// Generate state and verifier for PKCE
		state, err := generateRandomString(16)
		if err != nil {
			log.Printf("Failed to generate OAuth state: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to generate state",
			})
		}

		verifier, err := generateRandomString(64)
		if err != nil {
			log.Printf("Failed to generate PKCE verifier: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "Failed to generate verifier",
			})
		}

		log.Printf("Generated OAuth state and verifier - state: %s, verifier: %s...", state, verifier[:10])

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
		challenge := generateCodeChallenge(verifier)
		log.Printf("Generated PKCE code challenge: %s", challenge)

		url := auth.AuthURL(
			state,
			oauth2.SetAuthURLParam("code_challenge_method", "S256"),
			oauth2.SetAuthURLParam("code_challenge", challenge),
		)

		log.Printf("Redirecting to Spotify authorization: %s", url)
		return c.Redirect(http.StatusTemporaryRedirect, url)
	}
}

// callbackHandler handles the /api/auth/spotify/callback route
func callbackHandler(app *pocketbase.PocketBase) echo.HandlerFunc {
	return func(c echo.Context) error {
		log.Println("Handling Spotify OAuth callback")

		// Get state and code from query params
		state := c.QueryParam("state")
		code := c.QueryParam("code")
		errorParam := c.QueryParam("error")

		if errorParam != "" {
			log.Printf("OAuth error from Spotify: %s", errorParam)
			return c.Redirect(http.StatusTemporaryRedirect, "/dashboard?spotify=error&message="+errorParam)
		}

		if state == "" || code == "" {
			log.Printf("Missing state or code in callback - state: %s, code: %s", state, code)
			return c.Redirect(http.StatusTemporaryRedirect, "/dashboard?spotify=error&message=Missing+state+or+code")
		}

		log.Printf("Received OAuth callback with state: %s, code: %s...", state, code[:10])

		// Get and validate cookie
		cookie, err := c.Cookie(cookieName)
		if err != nil {
			log.Printf("Missing auth cookie in callback: %v", err)
			return c.Redirect(http.StatusTemporaryRedirect, "/dashboard?spotify=error&message=Missing+auth+cookie")
		}

		// Parse state and verifier from cookie
		cookieParts := parseAuthCookie(cookie.Value)
		if len(cookieParts) != 2 || cookieParts[0] != state {
			log.Printf("State mismatch in OAuth callback - expected: %s, received: %s", cookieParts[0], state)
			return c.Redirect(http.StatusTemporaryRedirect, "/dashboard?spotify=error&message=Invalid+state")
		}

		verifier := cookieParts[1]
		log.Printf("Retrieved PKCE verifier from cookie: %s...", verifier[:10])

		// Clear the cookie
		c.SetCookie(&http.Cookie{
			Name:     cookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   -1,
		})

		// Get authenticator and exchange code for token
		auth, err := getSpotifyAuthenticator(app)
		if err != nil {
			log.Printf("Failed to create authenticator in callback: %v", err)
			return c.Redirect(http.StatusTemporaryRedirect, "/dashboard?spotify=error&message=Auth+config+error")
		}

		// Exchange code for token with verifier
		log.Println("Exchanging authorization code for access token")
		token, err := auth.Exchange(c.Request().Context(), code,
			oauth2.SetAuthURLParam("code_verifier", verifier),
		)
		if err != nil {
			log.Printf("Token exchange failed: %v", err)
			return c.Redirect(http.StatusTemporaryRedirect, "/dashboard?spotify=error&message=Token+exchange+failed")
		}

		log.Printf("Successfully exchanged code for token - expiry: %v", token.Expiry)

		// Save tokens to database
		scopes := []string{
			string(spotifyauth.ScopeUserReadPrivate),
			string(spotifyauth.ScopeUserReadEmail),
			string(spotifyauth.ScopePlaylistReadPrivate),
			string(spotifyauth.ScopePlaylistReadCollaborative),
		}

		if err := saveSpotifyTokens(app.Dao(), token, scopes); err != nil {
			log.Printf("Failed to save tokens to database: %v", err)
			return c.Redirect(http.StatusTemporaryRedirect, "/dashboard?spotify=error&message=Failed+to+save+tokens")
		}

		log.Println("Spotify OAuth flow completed successfully")
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

// generateCodeChallenge generates a PKCE code challenge from verifier using SHA256
func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])
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
