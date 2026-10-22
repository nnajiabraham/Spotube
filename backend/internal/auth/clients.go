package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	spotify "github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

var (
	ErrNoTokenFound = errors.New("no token found for provider")
	ErrTokenExpired = errors.New("token expired and refresh failed")
	// ErrReconnectRequired means the provider rejected the stored refresh token; the user must reconnect.
	ErrReconnectRequired = errors.New("reconnect required")
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
	httpClient, err := f.HTTPClient(ctx, "spotify")
	if err != nil {
		return nil, err
	}
	return spotify.New(httpClient, spotify.WithRetry(true)), nil
}

// GetYouTubeService creates an authenticated YouTube service for jobs.
func (f *ClientFactory) GetYouTubeService(ctx context.Context) (*youtube.Service, error) {
	httpClient, err := f.HTTPClient(ctx, "google")
	if err != nil {
		return nil, err
	}
	return youtube.NewService(ctx, option.WithHTTPClient(httpClient))
}

// HTTPClient returns an OAuth2 HTTP client for provider ("spotify" or "google").
// Refreshed tokens are written back to the token repository; Spotify rotates refresh
// tokens, so dropping the new one revokes the stored token on the next run.
func (f *ClientFactory) HTTPClient(ctx context.Context, provider string) (*http.Client, error) {
	token, err := f.tokenRepo.GetToken(provider)
	if err != nil {
		return nil, err
	}
	if token == nil || !token.AccessToken.Valid {
		return nil, ErrNoTokenFound
	}

	clientID, clientSecret, err := LoadCredentials(f.credsRepo, provider)
	if err != nil {
		return nil, err
	}

	config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}
	switch provider {
	case "spotify":
		config.Endpoint = oauth2.Endpoint{AuthURL: spotifyauth.AuthURL, TokenURL: spotifyauth.TokenURL}
	case "google":
		config.Endpoint = google.Endpoint
	default:
		return nil, errors.New("unknown provider: " + provider)
	}

	current := &oauth2.Token{
		AccessToken:  token.AccessToken.String,
		RefreshToken: token.RefreshToken.String,
		Expiry:       time.Unix(token.Expiry.Int64, 0),
	}
	source := &persistingTokenSource{
		provider: provider,
		base:     config.TokenSource(ctx, current),
		repo:     f.tokenRepo,
		scopes:   token.Scopes,
		last:     current.AccessToken,
	}
	return oauth2.NewClient(ctx, source), nil
}

// persistingTokenSource saves tokens to the repository whenever the access token changes.
type persistingTokenSource struct {
	provider string
	base     oauth2.TokenSource
	repo     TokenRepository
	scopes   sql.NullString

	mu   sync.Mutex
	last string
}

func (s *persistingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := s.base.Token()
	if err != nil {
		var retrieveErr *oauth2.RetrieveError
		if errors.As(err, &retrieveErr) && retrieveErr.ErrorCode == "invalid_grant" {
			return nil, fmt.Errorf("%w (%s): %v", ErrReconnectRequired, s.provider, err)
		}
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if tok.AccessToken != s.last {
		s.last = tok.AccessToken
		saveErr := s.repo.UpsertToken(s.provider, Token{
			AccessToken:  sql.NullString{String: tok.AccessToken, Valid: tok.AccessToken != ""},
			RefreshToken: sql.NullString{String: tok.RefreshToken, Valid: tok.RefreshToken != ""},
			Expiry:       sql.NullInt64{Int64: tok.Expiry.Unix(), Valid: !tok.Expiry.IsZero()},
			Scopes:       s.scopes,
		})
		if saveErr != nil {
			return nil, fmt.Errorf("persist refreshed %s token: %w", s.provider, saveErr)
		}
	}
	return tok, nil
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
