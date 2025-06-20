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
var youtubeScopes = []string{
	youtube.YoutubeReadonlyScope,      // YouTube readonly access
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
		Scopes:       youtubeScopes,
		Endpoint:     google.Endpoint,
	}
	log.Printf("Created OAuth config with scopes: %v", youtubeScopes)

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

	// Use YouTube package's built-in authentication with token source
	// This is the recommended approach according to Google's documentation
	tokenSource := config.TokenSource(ctx, refreshedToken)
	log.Println("Created token source for YouTube service")

	// Create YouTube service using the recommended approach with token source
	// This allows the YouTube package to handle authentication internally
	svc, err := youtube.NewService(ctx, option.WithTokenSource(tokenSource))
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
		Scopes:       youtubeScopes,
		Endpoint:     google.Endpoint,
	}

	// Refresh token if needed
	refreshedToken, err := refreshTokenIfNeeded(ctx, dbProvider, token, config, "google")
	if err != nil {
		return nil, fmt.Errorf("failed to refresh Google token: %w", err)
	}

	// Create token source with custom HTTP client if provided (for testing)
	var tokenSource oauth2.TokenSource
	if httpClient != nil {
		// For testing - create context with custom client for OAuth operations
		ctxWithClient := context.WithValue(ctx, oauth2.HTTPClient, httpClient)
		tokenSource = config.TokenSource(ctxWithClient, refreshedToken)
	} else {
		// Normal case - use default token source
		tokenSource = config.TokenSource(ctx, refreshedToken)
	}

	// Create YouTube service with token source (simpler than custom HTTP client)
	var svc *youtube.Service
	if httpClient != nil {
		// For testing - use custom HTTP client
		svc, err = youtube.NewService(ctx, option.WithTokenSource(tokenSource), option.WithHTTPClient(httpClient))
	} else {
		// Normal case - let YouTube package handle HTTP client
		svc, err = youtube.NewService(ctx, option.WithTokenSource(tokenSource))
	}

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
