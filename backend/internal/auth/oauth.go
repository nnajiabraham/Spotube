package auth

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
)

type credentialProvider interface {
	GetSettings() (*SettingsRecord, error)
}

type CredentialProvider = credentialProvider

var (
	ErrNoSpotifyCredentials = errors.New("spotify credentials not configured")
	ErrNoGoogleCredentials  = errors.New("google credentials not configured")
)

type SettingsRecord struct {
	SpotifyClientID     sql.NullString
	SpotifyClientSecret sql.NullString
	GoogleClientID      sql.NullString
	GoogleClientSecret  sql.NullString
}

func LoadCredentials(repo credentialProvider, provider string) (clientID, clientSecret string, err error) {
	settings, err := repo.GetSettings()
	if err != nil {
		return "", "", fmt.Errorf("load settings: %w", err)
	}

	if settings == nil {
		switch strings.ToLower(provider) {
		case "spotify":
			return resolveCredentials(sql.NullString{}, sql.NullString{}, "SPOTIFY_CLIENT_ID", "SPOTIFY_CLIENT_SECRET", ErrNoSpotifyCredentials)
		case "google":
			return resolveCredentials(sql.NullString{}, sql.NullString{}, "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", ErrNoGoogleCredentials)
		default:
			return "", "", fmt.Errorf("unknown provider: %s", provider)
		}
	}

	switch strings.ToLower(provider) {
	case "spotify":
		return resolveCredentials(settings.SpotifyClientID, settings.SpotifyClientSecret, "SPOTIFY_CLIENT_ID", "SPOTIFY_CLIENT_SECRET", ErrNoSpotifyCredentials)
	case "google":
		return resolveCredentials(settings.GoogleClientID, settings.GoogleClientSecret, "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", ErrNoGoogleCredentials)
	default:
		return "", "", fmt.Errorf("unknown provider: %s", provider)
	}
}

func resolveCredentials(id sql.NullString, secret sql.NullString, envID, envSecret string, fallBackErr error) (string, string, error) {
	credID := strings.TrimSpace(id.String)
	credSecret := strings.TrimSpace(secret.String)

	if id.Valid && secret.Valid && credID != "" && credSecret != "" {
		return credID, credSecret, nil
	}

	envClientID := strings.TrimSpace(os.Getenv(envID))
	envClientSecret := strings.TrimSpace(os.Getenv(envSecret))

	if envClientID != "" && envClientSecret != "" {
		return envClientID, envClientSecret, nil
	}

	return "", "", fallBackErr
}

// Token repository helpers

type Token struct {
	AccessToken  sql.NullString
	RefreshToken sql.NullString
	Expiry       sql.NullInt64
	Scopes       sql.NullString
}

type TokenRepository interface {
	GetToken(provider string) (*Token, error)
	UpsertToken(provider string, token Token) error
}

// Package-level repository for handlers to access without global state.
var tokenRepo TokenRepository

func SetTokenRepository(repo TokenRepository) {
	tokenRepo = repo
}

func LoadToken(provider string) (*Token, error) {
	if tokenRepo == nil {
		return nil, errors.New("token repository not configured")
	}
	return tokenRepo.GetToken(provider)
}

func SaveToken(provider string, token Token) error {
	if tokenRepo == nil {
		return errors.New("token repository not configured")
	}
	return tokenRepo.UpsertToken(provider, token)
}
