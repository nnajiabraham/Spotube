package auth

import (
	"fmt"
	"os"
)

// loadCredentialsFromSettings loads OAuth credentials from settings collection with env fallback
func loadCredentialsFromSettings(dbProvider DatabaseProvider, service string) (clientID, clientSecret string, err error) {
	dao := dbProvider.Dao()

	// Try to load from settings collection first
	record, err := dao.FindRecordById("settings", "settings")
	if err == nil {
		// Successfully found settings record
		var clientIDField, clientSecretField string

		switch service {
		case "spotify":
			clientIDField = "spotify_client_id"
			clientSecretField = "spotify_client_secret"
		case "google":
			clientIDField = "google_client_id"
			clientSecretField = "google_client_secret"
		default:
			return "", "", fmt.Errorf("unsupported service: %s", service)
		}

		clientID = record.GetString(clientIDField)
		clientSecret = record.GetString(clientSecretField)

		// If both credentials are present in settings, use them
		if clientID != "" && clientSecret != "" {
			return clientID, clientSecret, nil
		}
	}

	// Fallback to environment variables
	var envClientID, envClientSecret string
	switch service {
	case "spotify":
		envClientID = os.Getenv("SPOTIFY_CLIENT_ID")
		envClientSecret = os.Getenv("SPOTIFY_CLIENT_SECRET")
	case "google":
		envClientID = os.Getenv("GOOGLE_CLIENT_ID")
		envClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	default:
		return "", "", fmt.Errorf("unsupported service: %s", service)
	}

	if envClientID == "" || envClientSecret == "" {
		return "", "", fmt.Errorf("%s client credentials not configured in settings or environment", service)
	}

	return envClientID, envClientSecret, nil
}
