package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	spotify "github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/youtube/v3"
)

var (
	ErrNoTokenFound = errors.New("no token found for provider")
	ErrTokenExpired = errors.New("token expired and refresh failed")
)

// ClientFactory provides authenticated clients for external APIs.
type ClientFactory struct {
	db        *sql.DB
	credsRepo CredentialProvider
	tokenRepo TokenRepository
}

func NewClientFactory(db *sql.DB, credsRepo CredentialProvider, tokenRepo TokenRepository) *ClientFactory {
	return &ClientFactory{
		db:        db,
		credsRepo: credsRepo,
		tokenRepo: tokenRepo,
	}
}

// GetSpotifyClient creates an authenticated Spotify client for jobs.
func (f *ClientFactory) GetSpotifyClient(ctx context.Context) (*spotify.Client, error) {
	token, err := f.tokenRepo.GetToken("spotify")
	if err != nil {
		return nil, err
	}

	if token == nil || !token.AccessToken.Valid {
		return nil, ErrNoTokenFound
	}

	clientID, clientSecret, err := LoadCredentials(f.credsRepo, "spotify")
	if err != nil {
		return nil, err
	}

	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  spotifyauth.AuthURL,
			TokenURL: spotifyauth.TokenURL,
		},
	}

	oauthToken := &oauth2.Token{
		AccessToken:  token.AccessToken.String,
		RefreshToken: token.RefreshToken.String,
		Expiry:       time.Unix(token.Expiry.Int64, 0),
	}

	// Create HTTP client with automatic token refresh
	httpClient := config.Client(ctx, oauthToken)
	return spotify.New(httpClient), nil
}

// GetYouTubeService creates an authenticated YouTube service for jobs.
func (f *ClientFactory) GetYouTubeService(ctx context.Context) (*youtube.Service, error) {
	token, err := f.tokenRepo.GetToken("google")
	if err != nil {
		return nil, err
	}

	if token == nil || !token.AccessToken.Valid {
		return nil, ErrNoTokenFound
	}

	clientID, clientSecret, err := LoadCredentials(f.credsRepo, "google")
	if err != nil {
		return nil, err
	}

	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{youtube.YoutubeScope, youtube.YoutubeReadonlyScope},
	}

	oauthToken := &oauth2.Token{
		AccessToken:  token.AccessToken.String,
		RefreshToken: token.RefreshToken.String,
		Expiry:       time.Unix(token.Expiry.Int64, 0),
	}

	// Create HTTP client with automatic token refresh
	httpClient := config.Client(ctx, oauthToken)
	return youtube.New(httpClient)
}

// RefreshTokenIfNeeded checks token expiry and refreshes if needed (utility for jobs).
func (f *ClientFactory) RefreshTokenIfNeeded(ctx context.Context, provider string) error {
	token, err := f.tokenRepo.GetToken(provider)
	if err != nil {
		return err
	}

	if token == nil || !token.AccessToken.Valid {
		return ErrNoTokenFound
	}

	// Check if token is expired (with 5 minute buffer)
	if time.Unix(token.Expiry.Int64, 0).Add(-5 * time.Minute).Before(time.Now()) {
		// Token needs refresh - this will be handled automatically by oauth2.Config.Client
		// We just need to make a dummy call to trigger refresh
		if provider == "spotify" {
			_, err := f.GetSpotifyClient(ctx)
			return err
		} else if provider == "google" {
			_, err := f.GetYouTubeService(ctx)
			return err
		}
	}

	return nil
}
