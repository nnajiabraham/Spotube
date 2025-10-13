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
