package auth

import (
	"context"
	"fmt"

	"github.com/labstack/echo/v5"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

// JobAuthContext implements AuthContext for background job execution
type JobAuthContext struct {
	dbProvider DatabaseProvider
}

// GetCredentials loads credentials from settings collection with environment fallback
func (j *JobAuthContext) GetCredentials(service string) (clientID, clientSecret string, err error) {
	return LoadCredentialsFromSettings(j.dbProvider, service)
}

// NewJobAuthContext creates an AuthContext for background jobs
func NewJobAuthContext(dbProvider DatabaseProvider) AuthContext {
	return &JobAuthContext{dbProvider: dbProvider}
}

// APIAuthContext implements AuthContext for Echo-based API handlers
type APIAuthContext struct {
	echoContext echo.Context
	dbProvider  DatabaseProvider
}

// GetCredentials loads credentials from settings collection with environment fallback
func (a *APIAuthContext) GetCredentials(service string) (clientID, clientSecret string, err error) {
	return LoadCredentialsFromSettings(a.dbProvider, service)
}

// NewAPIAuthContext creates an AuthContext for API handlers
func NewAPIAuthContext(echoContext echo.Context, dbProvider DatabaseProvider) AuthContext {
	return &APIAuthContext{
		echoContext: echoContext,
		dbProvider:  dbProvider,
	}
}

// GetSpotifyClient creates an authenticated Spotify client using the unified factory
func GetSpotifyClient(ctx context.Context, dbProvider DatabaseProvider, authCtx AuthContext) (*spotify.Client, error) {
	// Load credentials using the auth context
	clientID, clientSecret, err := authCtx.GetCredentials("spotify")
	if err != nil {
		return nil, fmt.Errorf("failed to load Spotify credentials: %w", err)
	}

	// Load token from database
	token, err := loadTokenFromDatabase(dbProvider, "spotify")
	if err != nil {
		return nil, fmt.Errorf("failed to load Spotify token: %w", err)
	}

	// Create OAuth2 config for token refresh
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  spotifyauth.AuthURL,
			TokenURL: spotifyauth.TokenURL,
		},
	}

	// Refresh token if needed
	refreshedToken, err := refreshTokenIfNeeded(ctx, dbProvider, token, config, "spotify")
	if err != nil {
		return nil, fmt.Errorf("failed to refresh Spotify token: %w", err)
	}

	// Create authenticated HTTP client using the refreshed token
	// This approach ensures that our test mocks can intercept the requests.
	httpClient := config.Client(ctx, refreshedToken)

	// Create and return Spotify client
	client := spotify.New(httpClient)
	return client, nil
}

// WithSpotifyClient is a helper function for API handlers to maintain backward compatibility
// This maintains the existing function signature expected by API handlers
func WithSpotifyClient(dbProvider DatabaseProvider, c echo.Context) (*spotify.Client, error) {
	ctx := c.Request().Context()
	authCtx := NewAPIAuthContext(c, dbProvider)
	return GetSpotifyClient(ctx, dbProvider, authCtx)
}

// GetSpotifyClientForJob is a helper function for background jobs
// This replaces the duplicate function in analysis.go
func GetSpotifyClientForJob(ctx context.Context, dbProvider DatabaseProvider) (*spotify.Client, error) {
	authCtx := NewJobAuthContext(dbProvider)
	return GetSpotifyClient(ctx, dbProvider, authCtx)
}
