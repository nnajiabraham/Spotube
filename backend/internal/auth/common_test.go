package auth

import (
	"context"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/models"
	"golang.org/x/oauth2"

	"github.com/manlikeabro/spotube/internal/testhelpers"
)

func TestLoadCredentialsFromSettings(t *testing.T) {
	// Create test app with settings collection
	testApp := testhelpers.SetupTestApp(t)

	tests := []struct {
		name           string
		service        string
		setupData      map[string]string
		envVars        map[string]string
		expectedID     string
		expectedSecret string
		expectedError  bool
	}{
		{
			name:    "spotify credentials from settings",
			service: "spotify",
			setupData: map[string]string{
				"spotify_client_id":     "settings_id",
				"spotify_client_secret": "settings_secret",
			},
			expectedID:     "settings_id",
			expectedSecret: "settings_secret",
			expectedError:  false,
		},
		{
			name:    "google credentials from settings",
			service: "google",
			setupData: map[string]string{
				"google_client_id":     "google_settings_id",
				"google_client_secret": "google_settings_secret",
			},
			expectedID:     "google_settings_id",
			expectedSecret: "google_settings_secret",
			expectedError:  false,
		},
		{
			name:          "unsupported service",
			service:       "invalid",
			expectedError: true,
		},
		{
			name:    "no credentials available",
			service: "spotify",
			setupData: map[string]string{
				"spotify_client_id":     "", // explicitly empty
				"spotify_client_secret": "",
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test data in settings collection if provided
			if len(tt.setupData) > 0 {
				dao := testApp.Dao()
				collection, err := dao.FindCollectionByNameOrId("settings")
				if err != nil {
					t.Fatal("Failed to find settings collection:", err)
				}

				// Try to find existing record or create new one
				record, err := dao.FindRecordById("settings", "settings")
				if err != nil {
					// Create new record if it doesn't exist
					record = models.NewRecord(collection)
					record.SetId("settings")
				}

				for field, value := range tt.setupData {
					record.Set(field, value)
				}

				if err := dao.SaveRecord(record); err != nil {
					t.Fatal("Failed to save settings record:", err)
				}
			}

			// Test credential loading
			clientID, clientSecret, err := loadCredentialsFromSettings(testApp, tt.service)

			if tt.expectedError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatal("Unexpected error:", err)
			}

			if clientID != tt.expectedID {
				t.Errorf("Expected clientID %s, got %s", tt.expectedID, clientID)
			}

			if clientSecret != tt.expectedSecret {
				t.Errorf("Expected clientSecret %s, got %s", tt.expectedSecret, clientSecret)
			}
		})
	}
}

func TestTokenManagement(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)

	// Test saving and loading tokens
	t.Run("save and load token", func(t *testing.T) {
		token := &oauth2.Token{
			AccessToken:  "test_access_token",
			RefreshToken: "test_refresh_token",
			TokenType:    "Bearer",
			Expiry:       time.Now().Add(time.Hour),
		}

		// Create a mock token record first
		dao := testApp.Dao()
		collection, err := dao.FindCollectionByNameOrId("oauth_tokens")
		if err != nil {
			t.Fatal("Failed to find oauth_tokens collection:", err)
		}

		record := models.NewRecord(collection)
		record.Set("provider", "spotify")
		record.Set("access_token", token.AccessToken)
		record.Set("refresh_token", token.RefreshToken)
		record.Set("expiry", token.Expiry)
		record.Set("scopes", "test-scope")

		if err := dao.SaveRecord(record); err != nil {
			t.Fatal("Failed to save initial token record:", err)
		}

		// Test loading token
		loadedToken, err := loadTokenFromDatabase(testApp, "spotify")
		if err != nil {
			t.Fatal("Failed to load token:", err)
		}

		if loadedToken.AccessToken != token.AccessToken {
			t.Errorf("Expected access token %s, got %s", token.AccessToken, loadedToken.AccessToken)
		}

		if loadedToken.RefreshToken != token.RefreshToken {
			t.Errorf("Expected refresh token %s, got %s", token.RefreshToken, loadedToken.RefreshToken)
		}
	})

	t.Run("token not found", func(t *testing.T) {
		_, err := loadTokenFromDatabase(testApp, "nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent token")
		}
	})
}

func TestRefreshTokenIfNeeded(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)

	// Create expired token record
	dao := testApp.Dao()
	collection, err := dao.FindCollectionByNameOrId("oauth_tokens")
	if err != nil {
		t.Fatal("Failed to find oauth_tokens collection:", err)
	}

	record := models.NewRecord(collection)
	record.Set("provider", "spotify")
	record.Set("access_token", "old_token")
	record.Set("refresh_token", "refresh_token")
	record.Set("expiry", time.Now().Add(-time.Hour)) // Expired token
	record.Set("scopes", "test-scope")

	if err := dao.SaveRecord(record); err != nil {
		t.Fatal("Failed to save token record:", err)
	}

	t.Run("token not expired", func(t *testing.T) {
		futureToken := &oauth2.Token{
			AccessToken:  "valid_token",
			RefreshToken: "refresh_token",
			Expiry:       time.Now().Add(time.Hour),
		}

		config := &oauth2.Config{}
		ctx := context.Background()

		refreshedToken, err := refreshTokenIfNeeded(ctx, testApp, futureToken, config, "spotify")
		if err != nil {
			t.Fatal("Unexpected error:", err)
		}

		// Should return same token since it's not expired
		if refreshedToken.AccessToken != "valid_token" {
			t.Error("Token should not have been refreshed")
		}
	})
}
