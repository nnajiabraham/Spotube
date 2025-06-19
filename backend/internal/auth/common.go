package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/pocketbase/pocketbase/daos"
	"github.com/zmb3/spotify/v2"
	"golang.org/x/oauth2"
	"google.golang.org/api/youtube/v3"
)

// Package auth provides a unified OAuth authentication system for Spotube.
//
// This package implements RFC-008b: Unified OAuth Client Factory System, which eliminates
// code duplication between background jobs and API handlers for Spotify and YouTube authentication.
//
// Key Features:
// - Settings collection integration with environment variable fallback
// - Unified client factories for both job and API contexts
// - Automatic token refresh with 30-second expiry buffer
// - Thread-safe token operations for concurrent job execution
// - Backward compatibility with all existing OAuth endpoints
//
// The system supports two execution contexts:
//   - JobAuthContext: Used by background sync jobs (analysis and execution)
//   - APIAuthContext: Used by OAuth endpoints and API handlers with Echo integration
//
// Credential loading priority: settings collection → environment variables
//
// Usage Examples:
//
//	// Background job context
//	spotifyClient, err := auth.GetSpotifyClientForJob(ctx, daoProvider)
//	youtubeService, err := auth.GetYouTubeServiceForJob(ctx, daoProvider)
//
//	// API handler context (maintains backward compatibility)
//	spotifyClient, err := auth.WithSpotifyClient(app, echoContext)
//	youtubeService, err := auth.WithGoogleClient(ctx, daoProvider)

// DatabaseProvider interface compatible with both jobs and API handlers
type DatabaseProvider interface {
	Dao() *daos.Dao
}

// AuthContext interface for different execution environments
type AuthContext interface {
	GetCredentials(service string) (clientID, clientSecret string, err error)
}

// ClientFactory interface for unified OAuth client creation
type ClientFactory interface {
	GetSpotifyClient(ctx context.Context, dbProvider DatabaseProvider, authCtx AuthContext) (*spotify.Client, error)
	GetYouTubeService(ctx context.Context, dbProvider DatabaseProvider, authCtx AuthContext) (*youtube.Service, error)
}

// refreshTokenIfNeeded handles unified OAuth token refresh logic
func refreshTokenIfNeeded(ctx context.Context, dbProvider DatabaseProvider, token *oauth2.Token, config *oauth2.Config, provider string) (*oauth2.Token, error) {
	// Check if token needs refresh (30-second buffer)
	if token.Expiry.Before(time.Now().Add(30 * time.Second)) {
		// Create token source that will auto-refresh
		ts := config.TokenSource(ctx, token)
		newToken, err := ts.Token()
		if err != nil {
			return nil, fmt.Errorf("failed to refresh %s token: %w", provider, err)
		}

		// Save refreshed token if it changed
		if newToken.AccessToken != token.AccessToken {
			if err := saveTokenToDatabase(dbProvider, provider, newToken); err != nil {
				return nil, fmt.Errorf("failed to save refreshed %s token: %w", provider, err)
			}
		}

		return newToken, nil
	}

	return token, nil
}

// saveTokenToDatabase persists OAuth tokens to the oauth_tokens collection
func saveTokenToDatabase(dbProvider DatabaseProvider, provider string, token *oauth2.Token) error {
	dao := dbProvider.Dao()

	// Find or create oauth_tokens record
	record, err := dao.FindFirstRecordByFilter("oauth_tokens", fmt.Sprintf("provider = '%s'", provider))
	if err != nil {
		return fmt.Errorf("failed to find %s token record: %w", provider, err)
	}

	// Update token fields
	record.Set("access_token", token.AccessToken)
	record.Set("refresh_token", token.RefreshToken)
	record.Set("expiry", token.Expiry)

	return dao.SaveRecord(record)
}

// loadTokenFromDatabase retrieves OAuth tokens from the oauth_tokens collection
func loadTokenFromDatabase(dbProvider DatabaseProvider, provider string) (*oauth2.Token, error) {
	dao := dbProvider.Dao()

	record, err := dao.FindFirstRecordByFilter("oauth_tokens", fmt.Sprintf("provider = '%s'", provider))
	if err != nil {
		return nil, fmt.Errorf("no %s token found: %w", provider, err)
	}

	token := &oauth2.Token{
		AccessToken:  record.GetString("access_token"),
		RefreshToken: record.GetString("refresh_token"),
		TokenType:    "Bearer",
	}

	// Parse expiry time
	expiryStr := record.GetString("expiry")
	if expiryStr != "" {
		expiry, err := time.Parse(time.RFC3339, expiryStr)
		if err == nil {
			token.Expiry = expiry
		}
	}

	return token, nil
}
