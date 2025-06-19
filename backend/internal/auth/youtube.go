package auth

import (
	"context"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// GetYouTubeService creates an authenticated YouTube service using the unified factory
func GetYouTubeService(ctx context.Context, dbProvider DatabaseProvider, authCtx AuthContext) (*youtube.Service, error) {
	// Load credentials using the auth context
	clientID, clientSecret, err := authCtx.GetCredentials("google")
	if err != nil {
		return nil, fmt.Errorf("failed to load Google credentials: %w", err)
	}

	// Load token from database
	token, err := loadTokenFromDatabase(dbProvider, "google")
	if err != nil {
		return nil, fmt.Errorf("failed to load Google token: %w", err)
	}

	// Create OAuth2 config for token refresh
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{youtube.YoutubeReadonlyScope},
		Endpoint:     google.Endpoint,
	}

	// Refresh token if needed
	refreshedToken, err := refreshTokenIfNeeded(ctx, dbProvider, token, config, "google")
	if err != nil {
		return nil, fmt.Errorf("failed to refresh Google token: %w", err)
	}

	// Create HTTP client that will use the default transport (for httpmock compatibility)
	httpClient := &http.Client{
		Transport: http.DefaultTransport,
	}

	// Create a context with the custom client for OAuth operations
	ctxWithClient := context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	tokenSource := config.TokenSource(ctxWithClient, refreshedToken)

	// Create YouTube service with the HTTP client and token source
	opts := []option.ClientOption{
		option.WithTokenSource(tokenSource),
		option.WithHTTPClient(httpClient),
	}

	svc, err := youtube.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create YouTube service: %w", err)
	}

	return svc, nil
}

// WithGoogleClient is a helper function for API handlers to maintain backward compatibility
// This maintains the existing function signature expected by API handlers
func WithGoogleClient(ctx context.Context, dbProvider DatabaseProvider) (*youtube.Service, error) {
	authCtx := NewJobAuthContext(dbProvider) // Use job context as it doesn't need Echo
	return GetYouTubeService(ctx, dbProvider, authCtx)
}

// WithGoogleClientCustom is a helper function for API handlers with custom HTTP client
func WithGoogleClientCustom(ctx context.Context, dbProvider DatabaseProvider, httpClient *http.Client) (*youtube.Service, error) {
	// Load credentials
	authCtx := NewJobAuthContext(dbProvider)
	clientID, clientSecret, err := authCtx.GetCredentials("google")
	if err != nil {
		return nil, fmt.Errorf("failed to load Google credentials: %w", err)
	}

	// Load token from database
	token, err := loadTokenFromDatabase(dbProvider, "google")
	if err != nil {
		return nil, fmt.Errorf("failed to load Google token: %w", err)
	}

	// Create OAuth2 config
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{youtube.YoutubeReadonlyScope},
		Endpoint:     google.Endpoint,
	}

	// Use custom HTTP client if provided (for testing)
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	// Create a context with the custom client
	ctxWithClient := context.WithValue(ctx, oauth2.HTTPClient, httpClient)

	tokenSource := config.TokenSource(ctxWithClient, token)
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %w", err)
	}

	if newToken.AccessToken != token.AccessToken {
		if err := saveTokenToDatabase(dbProvider, "google", newToken); err != nil {
			return nil, fmt.Errorf("failed to save refreshed token: %w", err)
		}
	}

	// Create YouTube service with custom HTTP client
	opts := []option.ClientOption{
		option.WithTokenSource(tokenSource),
		option.WithHTTPClient(httpClient),
	}

	svc, err := youtube.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create YouTube service: %w", err)
	}

	return svc, nil
}

// GetYouTubeServiceForJob is a helper function for background jobs
// This replaces the duplicate function in analysis.go
func GetYouTubeServiceForJob(ctx context.Context, dbProvider DatabaseProvider) (*youtube.Service, error) {
	authCtx := NewJobAuthContext(dbProvider)
	return GetYouTubeService(ctx, dbProvider, authCtx)
}
