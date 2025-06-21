package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googleoauth2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// YouTube API requires user identity establishment for proper authentication
// Using constants from Google API packages instead of hardcoded strings
// Updated to include full YouTube permissions for playlist modification (RFC-010 BF1)
var YouTubeScopes = []string{
	youtube.YoutubeScope,              // Full YouTube access for playlist modification
	googleoauth2.UserinfoProfileScope, // User profile for identity
	googleoauth2.UserinfoEmailScope,   // User email for identity
}

// GetYouTubeService creates an authenticated YouTube service using the unified factory
func GetYouTubeService(ctx context.Context, dbProvider DatabaseProvider, authCtx AuthContext) (*youtube.Service, error) {
	log.Println("Creating YouTube service with enhanced scopes and simplified authentication...")

	// Load credentials using the auth context
	clientID, clientSecret, err := authCtx.GetCredentials("google")
	if err != nil {
		return nil, fmt.Errorf("failed to load Google credentials: %w", err)
	}
	log.Printf("Loaded Google credentials - ClientID: %s...", clientID[:min(10, len(clientID))])

	// Load token from database
	token, err := loadTokenFromDatabase(dbProvider, "google")
	if err != nil {
		return nil, fmt.Errorf("failed to load Google token: %w", err)
	}
	log.Printf("Loaded Google token - AccessToken: %s..., Expiry: %v",
		token.AccessToken[:min(10, len(token.AccessToken))], token.Expiry)

	// Create OAuth2 config for token management
	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       YouTubeScopes,
		Endpoint:     google.Endpoint,
	}
	log.Printf("Created OAuth config with scopes: %v", YouTubeScopes)

	// Refresh token if needed
	refreshedToken, err := refreshTokenIfNeeded(ctx, dbProvider, token, config, "google")
	if err != nil {
		return nil, fmt.Errorf("failed to refresh Google token: %w", err)
	}

	if refreshedToken.AccessToken != token.AccessToken {
		log.Println("Token was refreshed")
	} else {
		log.Println("Token was still valid, no refresh needed")
	}

	// Create an OAuth2 client that uses the default transport for httpmock compatibility
	// The client will automatically handle token refreshes.
	oauthClient := config.Client(ctx, refreshedToken)
	log.Println("Created OAuth2 client with automatic authentication headers")

	// Create YouTube service with the authenticated HTTP client
	svc, err := youtube.NewService(ctx, option.WithHTTPClient(oauthClient))
	if err != nil {
		return nil, fmt.Errorf("failed to create YouTube service: %w", err)
	}

	log.Println("Successfully created YouTube service with built-in authentication")
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
		Scopes:       YouTubeScopes,
		Endpoint:     google.Endpoint,
	}

	// Refresh token if needed
	refreshedToken, err := refreshTokenIfNeeded(ctx, dbProvider, token, config, "google")
	if err != nil {
		return nil, fmt.Errorf("failed to refresh Google token: %w", err)
	}

	// If custom HTTP client provided, use it for OAuth client
	var oauthClient *http.Client
	if httpClient != nil {
		// For testing, inject the mock client into the context so refreshes are mocked
		ctxWithClient := context.WithValue(ctx, oauth2.HTTPClient, httpClient)
		oauthClient = config.Client(ctxWithClient, refreshedToken)
	} else {
		// Normal case - use default OAuth client
		oauthClient = config.Client(ctx, refreshedToken)
	}

	// Create YouTube service with the OAuth client
	svc, err := youtube.NewService(ctx, option.WithHTTPClient(oauthClient))
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
