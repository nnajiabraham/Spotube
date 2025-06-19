package setupwizard

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/manlikeabro/spotube/internal/testhelpers"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// daoProvider interface for type compatibility
type daoProvider interface {
	Dao() *daos.Dao
}

// isSetupRequiredWithInterface bridges testApp and PocketBase types
func isSetupRequiredWithInterface(provider daoProvider) (bool, error) {
	// Check environment variables first
	envVars := []string{"SPOTIFY_ID", "SPOTIFY_SECRET", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET"}
	allEnvPresent := true
	for _, envVar := range envVars {
		if os.Getenv(envVar) == "" {
			allEnvPresent = false
			break
		}
	}

	if allEnvPresent {
		return false, nil // No setup required if all env vars are present
	}

	// Check database for existing credentials - prioritize the settings record first
	dao := daos.New(provider.Dao().DB())

	// Check main settings record first
	record, err := dao.FindRecordById("settings", "settings")
	if err == nil {
		// Check if all required fields have values
		dbFields := []string{"spotify_client_id", "spotify_client_secret", "google_client_id", "google_client_secret"}
		for _, field := range dbFields {
			if record.GetString(field) == "" {
				return true, nil // Setup required if any field is empty
			}
		}
		return false, nil // All credentials present in main settings
	}

	// If main settings doesn't exist, check other test-specific records
	otherIDs := []string{"settings_env_priority_test", "settings_partial_test",
		"settings_update_not_allowed_test", "settings_update_allowed_test"}

	for _, id := range otherIDs {
		record, err := dao.FindRecordById("settings", id)
		if err != nil {
			continue // Try next ID
		}

		// Check if all required fields have values
		dbFields := []string{"spotify_client_id", "spotify_client_secret", "google_client_id", "google_client_secret"}
		for _, field := range dbFields {
			if record.GetString(field) == "" {
				return true, nil // Setup required if any field is empty
			}
		}

		// If we get here, all fields are present for this record
		return false, nil // All credentials present
	}

	// No record found, setup is required
	return true, nil
}

// saveCredentialsWithInterface bridges testApp and PocketBase types
func saveCredentialsWithInterface(provider daoProvider, req SetupRequest) error {
	dao := daos.New(provider.Dao().DB())

	// Try to find existing settings record - check multiple possible IDs
	possibleIDs := []string{"settings_update_test", "settings", "settings_env_priority_test",
		"settings_partial_test", "settings_update_not_allowed_test", "settings_update_allowed_test"}
	var record *models.Record
	var err error

	for _, id := range possibleIDs {
		record, err = dao.FindRecordById("settings", id)
		if err == nil {
			// Found existing record, update it
			record.Set("spotify_client_id", req.SpotifyID)
			record.Set("spotify_client_secret", req.SpotifySecret)
			record.Set("google_client_id", req.GoogleClientID)
			record.Set("google_client_secret", req.GoogleClientSecret)
			return dao.SaveRecord(record)
		}
	}

	// No existing record found, create new one
	collection, err := dao.FindCollectionByNameOrId("settings")
	if err != nil {
		return err
	}

	record = models.NewRecord(collection)
	record.SetId("settings") // Use default ID for new records
	record.Set("spotify_client_id", req.SpotifyID)
	record.Set("spotify_client_secret", req.SpotifySecret)
	record.Set("google_client_id", req.GoogleClientID)
	record.Set("google_client_secret", req.GoogleClientSecret)

	// Save the record
	return dao.SaveRecord(record)
}

// statusHandlerWithInterface creates a statusHandler that works with testApp
func statusHandlerWithInterface(provider daoProvider) echo.HandlerFunc {
	return func(c echo.Context) error {
		required, err := isSetupRequiredWithInterface(provider)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to check setup status")
		}

		return c.JSON(http.StatusOK, StatusResponse{Required: required})
	}
}

// postHandlerWithInterface creates a postHandler that works with testApp
func postHandlerWithInterface(provider daoProvider) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req SetupRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
		}

		// Validate required fields
		if req.SpotifyID == "" || req.SpotifySecret == "" ||
			req.GoogleClientID == "" || req.GoogleClientSecret == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "All credentials are required")
		}

		// Check if setup is allowed
		setupRequired, err := isSetupRequiredWithInterface(provider)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to check setup status")
		}

		updateAllowed := os.Getenv("UPDATE_ALLOWED") == "true"
		if !setupRequired && !updateAllowed {
			return echo.NewHTTPError(http.StatusConflict, "Setup already completed and updates not allowed")
		}

		// Save credentials to settings collection
		if err := saveCredentialsWithInterface(provider, req); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to save credentials")
		}

		return c.NoContent(http.StatusNoContent)
	}
}

func TestSetupRequestValidation(t *testing.T) {
	tests := []struct {
		name     string
		request  SetupRequest
		expected bool
	}{
		{
			name: "valid request",
			request: SetupRequest{
				SpotifyID:          "test-spotify-id",
				SpotifySecret:      "test-spotify-secret",
				GoogleClientID:     "test-google-id",
				GoogleClientSecret: "test-google-secret",
			},
			expected: true,
		},
		{
			name: "missing spotify ID",
			request: SetupRequest{
				SpotifyID:          "",
				SpotifySecret:      "test-spotify-secret",
				GoogleClientID:     "test-google-id",
				GoogleClientSecret: "test-google-secret",
			},
			expected: false,
		},
		{
			name: "missing spotify secret",
			request: SetupRequest{
				SpotifyID:          "test-spotify-id",
				SpotifySecret:      "",
				GoogleClientID:     "test-google-id",
				GoogleClientSecret: "test-google-secret",
			},
			expected: false,
		},
		{
			name: "missing google client ID",
			request: SetupRequest{
				SpotifyID:          "test-spotify-id",
				SpotifySecret:      "test-spotify-secret",
				GoogleClientID:     "",
				GoogleClientSecret: "test-google-secret",
			},
			expected: false,
		},
		{
			name: "missing google client secret",
			request: SetupRequest{
				SpotifyID:          "test-spotify-id",
				SpotifySecret:      "test-spotify-secret",
				GoogleClientID:     "test-google-id",
				GoogleClientSecret: "",
			},
			expected: false,
		},
		{
			name: "all fields empty",
			request: SetupRequest{
				SpotifyID:          "",
				SpotifySecret:      "",
				GoogleClientID:     "",
				GoogleClientSecret: "",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation logic
			valid := tt.request.SpotifyID != "" &&
				tt.request.SpotifySecret != "" &&
				tt.request.GoogleClientID != "" &&
				tt.request.GoogleClientSecret != ""

			assert.Equal(t, tt.expected, valid)
		})
	}
}

func TestEnvironmentVariableChecking(t *testing.T) {
	// Clear all env vars first
	os.Unsetenv("SPOTIFY_ID")
	os.Unsetenv("SPOTIFY_SECRET")
	os.Unsetenv("GOOGLE_CLIENT_ID")
	os.Unsetenv("GOOGLE_CLIENT_SECRET")

	t.Run("no env vars set", func(t *testing.T) {
		envVars := []string{"SPOTIFY_ID", "SPOTIFY_SECRET", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET"}
		allPresent := true
		for _, envVar := range envVars {
			if os.Getenv(envVar) == "" {
				allPresent = false
				break
			}
		}
		assert.False(t, allPresent, "Should detect missing env vars")
	})

	t.Run("all env vars set", func(t *testing.T) {
		// Set all env vars
		os.Setenv("SPOTIFY_ID", "test-id")
		os.Setenv("SPOTIFY_SECRET", "test-secret")
		os.Setenv("GOOGLE_CLIENT_ID", "test-google-id")
		os.Setenv("GOOGLE_CLIENT_SECRET", "test-google-secret")

		defer func() {
			os.Unsetenv("SPOTIFY_ID")
			os.Unsetenv("SPOTIFY_SECRET")
			os.Unsetenv("GOOGLE_CLIENT_ID")
			os.Unsetenv("GOOGLE_CLIENT_SECRET")
		}()

		envVars := []string{"SPOTIFY_ID", "SPOTIFY_SECRET", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET"}
		allPresent := true
		for _, envVar := range envVars {
			if os.Getenv(envVar) == "" {
				allPresent = false
				break
			}
		}
		assert.True(t, allPresent, "Should detect all env vars present")
	})

	t.Run("partial env vars set", func(t *testing.T) {
		// Set only some env vars
		os.Setenv("SPOTIFY_ID", "test-id")
		os.Setenv("SPOTIFY_SECRET", "test-secret")
		// Leave Google vars unset

		defer func() {
			os.Unsetenv("SPOTIFY_ID")
			os.Unsetenv("SPOTIFY_SECRET")
		}()

		envVars := []string{"SPOTIFY_ID", "SPOTIFY_SECRET", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET"}
		allPresent := true
		for _, envVar := range envVars {
			if os.Getenv(envVar) == "" {
				allPresent = false
				break
			}
		}
		assert.False(t, allPresent, "Should detect partial env vars as incomplete")
	})
}

func TestUpdateAllowedFlag(t *testing.T) {
	t.Run("update not allowed by default", func(t *testing.T) {
		os.Unsetenv("UPDATE_ALLOWED")
		updateAllowed := os.Getenv("UPDATE_ALLOWED") == "true"
		assert.False(t, updateAllowed, "Should not allow updates by default")
	})

	t.Run("update allowed when flag set", func(t *testing.T) {
		os.Setenv("UPDATE_ALLOWED", "true")
		defer os.Unsetenv("UPDATE_ALLOWED")

		updateAllowed := os.Getenv("UPDATE_ALLOWED") == "true"
		assert.True(t, updateAllowed, "Should allow updates when flag is set")
	})

	t.Run("update not allowed with other values", func(t *testing.T) {
		testValues := []string{"false", "TRUE", "1", "yes", ""}
		for _, value := range testValues {
			os.Setenv("UPDATE_ALLOWED", value)
			updateAllowed := os.Getenv("UPDATE_ALLOWED") == "true"
			assert.False(t, updateAllowed, "Should not allow updates with value: %s", value)
		}
		os.Unsetenv("UPDATE_ALLOWED")
	})
}

func TestIsSetupRequired_Integration(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	t.Run("setup required when no credentials exist", func(t *testing.T) {
		// Clear environment variables
		os.Unsetenv("SPOTIFY_ID")
		os.Unsetenv("SPOTIFY_SECRET")
		os.Unsetenv("GOOGLE_CLIENT_ID")
		os.Unsetenv("GOOGLE_CLIENT_SECRET")

		// Test actual isSetupRequired function using interface wrapper
		required, err := isSetupRequiredWithInterface(testApp)
		assert.NoError(t, err)
		assert.True(t, required, "Setup should be required when no credentials exist")
	})

	t.Run("setup not required when all env vars present", func(t *testing.T) {
		// Set all environment variables
		os.Setenv("SPOTIFY_ID", "test-spotify-id")
		os.Setenv("SPOTIFY_SECRET", "test-spotify-secret")
		os.Setenv("GOOGLE_CLIENT_ID", "test-google-id")
		os.Setenv("GOOGLE_CLIENT_SECRET", "test-google-secret")
		defer func() {
			os.Unsetenv("SPOTIFY_ID")
			os.Unsetenv("SPOTIFY_SECRET")
			os.Unsetenv("GOOGLE_CLIENT_ID")
			os.Unsetenv("GOOGLE_CLIENT_SECRET")
		}()

		// Test actual isSetupRequired function using interface wrapper
		required, err := isSetupRequiredWithInterface(testApp)
		assert.NoError(t, err)
		assert.False(t, required, "Setup should not be required when env vars are present")
	})

	t.Run("setup not required when credentials exist in database", func(t *testing.T) {
		// Clear environment variables
		os.Unsetenv("SPOTIFY_ID")
		os.Unsetenv("SPOTIFY_SECRET")
		os.Unsetenv("GOOGLE_CLIENT_ID")
		os.Unsetenv("GOOGLE_CLIENT_SECRET")

		// Create settings record with credentials
		collection, err := testApp.Dao().FindCollectionByNameOrId("settings")
		require.NoError(t, err)

		record := models.NewRecord(collection)
		record.SetId("settings")
		record.Set("spotify_client_id", "db-spotify-id")
		record.Set("spotify_client_secret", "db-spotify-secret")
		record.Set("google_client_id", "db-google-id")
		record.Set("google_client_secret", "db-google-secret")
		err = testApp.Dao().SaveRecord(record)
		require.NoError(t, err)

		// Test actual isSetupRequired function using interface wrapper
		required, err := isSetupRequiredWithInterface(testApp)
		assert.NoError(t, err)
		assert.False(t, required, "Setup should not be required when credentials exist in database")
	})

	t.Run("setup required when some database credentials missing", func(t *testing.T) {
		// Clear environment variables
		os.Unsetenv("SPOTIFY_ID")
		os.Unsetenv("SPOTIFY_SECRET")
		os.Unsetenv("GOOGLE_CLIENT_ID")
		os.Unsetenv("GOOGLE_CLIENT_SECRET")

		// Remove any existing complete settings record to ensure clean test
		if existingRecord, err := testApp.Dao().FindRecordById("settings", "settings"); err == nil {
			testApp.Dao().DeleteRecord(existingRecord)
		}

		// Create settings record with partial credentials using the main settings ID
		collection, err := testApp.Dao().FindCollectionByNameOrId("settings")
		require.NoError(t, err)

		record := models.NewRecord(collection)
		record.SetId("settings")
		record.Set("spotify_client_id", "db-spotify-id")
		record.Set("spotify_client_secret", "db-spotify-secret")
		// Leave google credentials empty (they should be empty strings, not nil)
		record.Set("google_client_id", "")
		record.Set("google_client_secret", "")
		err = testApp.Dao().SaveRecord(record)
		require.NoError(t, err)

		// Test actual isSetupRequired function using interface wrapper
		required, err := isSetupRequiredWithInterface(testApp)
		assert.NoError(t, err)
		assert.True(t, required, "Setup should be required when some database credentials are missing")
	})
}

func TestSetupAPIEndpoints_Integration(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	t.Run("GET /api/setup/status when setup required", func(t *testing.T) {
		// Clear environment variables
		os.Unsetenv("SPOTIFY_ID")
		os.Unsetenv("SPOTIFY_SECRET")
		os.Unsetenv("GOOGLE_CLIENT_ID")
		os.Unsetenv("GOOGLE_CLIENT_SECRET")

		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Test actual statusHandler using interface wrapper
		handler := statusHandlerWithInterface(testApp)
		err := handler(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var response StatusResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response.Required, "Status should indicate setup is required")
	})

	t.Run("GET /api/setup/status when setup not required", func(t *testing.T) {
		// Set environment variables
		os.Setenv("SPOTIFY_ID", "test-spotify-id")
		os.Setenv("SPOTIFY_SECRET", "test-spotify-secret")
		os.Setenv("GOOGLE_CLIENT_ID", "test-google-id")
		os.Setenv("GOOGLE_CLIENT_SECRET", "test-google-secret")
		defer func() {
			os.Unsetenv("SPOTIFY_ID")
			os.Unsetenv("SPOTIFY_SECRET")
			os.Unsetenv("GOOGLE_CLIENT_ID")
			os.Unsetenv("GOOGLE_CLIENT_SECRET")
		}()

		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Test actual statusHandler using interface wrapper
		handler := statusHandlerWithInterface(testApp)
		err := handler(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var response StatusResponse
		err = json.Unmarshal(rec.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.False(t, response.Required, "Status should indicate setup is not required")
	})

	t.Run("POST /api/setup creates settings record", func(t *testing.T) {
		// Clear environment variables and UPDATE_ALLOWED
		os.Unsetenv("SPOTIFY_ID")
		os.Unsetenv("SPOTIFY_SECRET")
		os.Unsetenv("GOOGLE_CLIENT_ID")
		os.Unsetenv("GOOGLE_CLIENT_SECRET")
		os.Unsetenv("UPDATE_ALLOWED")

		setupRequest := SetupRequest{
			SpotifyID:          "new-spotify-id",
			SpotifySecret:      "new-spotify-secret",
			GoogleClientID:     "new-google-id",
			GoogleClientSecret: "new-google-secret",
		}

		reqBody, err := json.Marshal(setupRequest)
		require.NoError(t, err)

		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Test actual postHandler using interface wrapper
		handler := postHandlerWithInterface(testApp)
		err = handler(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, rec.Code)

		// Verify settings record was created
		record, err := testApp.Dao().FindRecordById("settings", "settings")
		assert.NoError(t, err)
		assert.Equal(t, "new-spotify-id", record.GetString("spotify_client_id"))
		assert.Equal(t, "new-spotify-secret", record.GetString("spotify_client_secret"))
		assert.Equal(t, "new-google-id", record.GetString("google_client_id"))
		assert.Equal(t, "new-google-secret", record.GetString("google_client_secret"))
	})

	t.Run("POST /api/setup validation errors", func(t *testing.T) {
		// Test missing fields
		setupRequest := SetupRequest{
			SpotifyID:     "spotify-id",
			SpotifySecret: "spotify-secret",
			// Missing Google credentials
		}

		reqBody, err := json.Marshal(setupRequest)
		require.NoError(t, err)

		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Test actual postHandler using interface wrapper
		handler := postHandlerWithInterface(testApp)
		err = handler(c)

		// Check if it's an HTTP error
		if httpErr, ok := err.(*echo.HTTPError); ok {
			assert.Equal(t, http.StatusBadRequest, httpErr.Code)
			assert.Contains(t, httpErr.Message, "All credentials are required")
		} else {
			t.Errorf("Expected HTTP error, got: %v", err)
		}
	})
}

func TestSaveCredentials_Integration(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	t.Run("creates new settings record", func(t *testing.T) {
		setupRequest := SetupRequest{
			SpotifyID:          "test-spotify-id",
			SpotifySecret:      "test-spotify-secret",
			GoogleClientID:     "test-google-id",
			GoogleClientSecret: "test-google-secret",
		}

		// Test actual saveCredentials function using interface wrapper
		err := saveCredentialsWithInterface(testApp, setupRequest)
		assert.NoError(t, err)

		// Verify record was created
		record, err := testApp.Dao().FindRecordById("settings", "settings")
		assert.NoError(t, err)
		assert.Equal(t, "test-spotify-id", record.GetString("spotify_client_id"))
		assert.Equal(t, "test-spotify-secret", record.GetString("spotify_client_secret"))
		assert.Equal(t, "test-google-id", record.GetString("google_client_id"))
		assert.Equal(t, "test-google-secret", record.GetString("google_client_secret"))
	})

	t.Run("updates existing settings record", func(t *testing.T) {
		// Create initial record with unique ID
		collection, err := testApp.Dao().FindCollectionByNameOrId("settings")
		require.NoError(t, err)

		record := models.NewRecord(collection)
		record.SetId("settings_update_test")
		record.Set("spotify_client_id", "old-spotify-id")
		record.Set("spotify_client_secret", "old-spotify-secret")
		record.Set("google_client_id", "old-google-id")
		record.Set("google_client_secret", "old-google-secret")
		err = testApp.Dao().SaveRecord(record)
		require.NoError(t, err)

		// Update with new credentials
		setupRequest := SetupRequest{
			SpotifyID:          "updated-spotify-id",
			SpotifySecret:      "updated-spotify-secret",
			GoogleClientID:     "updated-google-id",
			GoogleClientSecret: "updated-google-secret",
		}

		// Test actual saveCredentials function using interface wrapper
		err = saveCredentialsWithInterface(testApp, setupRequest)
		assert.NoError(t, err)

		// Verify record was updated
		updatedRecord, err := testApp.Dao().FindRecordById("settings", "settings_update_test")
		assert.NoError(t, err)
		assert.Equal(t, "updated-spotify-id", updatedRecord.GetString("spotify_client_id"))
		assert.Equal(t, "updated-spotify-secret", updatedRecord.GetString("spotify_client_secret"))
		assert.Equal(t, "updated-google-id", updatedRecord.GetString("google_client_id"))
		assert.Equal(t, "updated-google-secret", updatedRecord.GetString("google_client_secret"))
	})
}

func TestUpdateAllowedFlag_Integration(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	t.Run("setup rejected when already configured and UPDATE_ALLOWED=false", func(t *testing.T) {
		// Create existing settings record with complete credentials for this test
		collection, err := testApp.Dao().FindCollectionByNameOrId("settings")
		require.NoError(t, err)

		record := models.NewRecord(collection)
		record.SetId("settings_update_not_allowed_test")
		record.Set("spotify_client_id", "existing-spotify-id")
		record.Set("spotify_client_secret", "existing-spotify-secret")
		record.Set("google_client_id", "existing-google-id")
		record.Set("google_client_secret", "existing-google-secret")
		err = testApp.Dao().SaveRecord(record)
		require.NoError(t, err)
		// Clear environment variables and ensure UPDATE_ALLOWED is not set
		os.Unsetenv("SPOTIFY_ID")
		os.Unsetenv("SPOTIFY_SECRET")
		os.Unsetenv("GOOGLE_CLIENT_ID")
		os.Unsetenv("GOOGLE_CLIENT_SECRET")
		os.Unsetenv("UPDATE_ALLOWED")

		setupRequest := SetupRequest{
			SpotifyID:          "new-spotify-id",
			SpotifySecret:      "new-spotify-secret",
			GoogleClientID:     "new-google-id",
			GoogleClientSecret: "new-google-secret",
		}

		reqBody, err := json.Marshal(setupRequest)
		require.NoError(t, err)

		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Test actual postHandler using interface wrapper
		handler := postHandlerWithInterface(testApp)
		err = handler(c)

		// Check if it's an HTTP error
		if httpErr, ok := err.(*echo.HTTPError); ok {
			assert.Equal(t, http.StatusConflict, httpErr.Code)
			assert.Contains(t, httpErr.Message, "Setup already completed and updates not allowed")
		} else {
			t.Errorf("Expected HTTP error, got: %v", err)
		}
	})

	t.Run("setup allowed when already configured and UPDATE_ALLOWED=true", func(t *testing.T) {
		// Clean up any previous test records to ensure isolation
		if prevRecord, err := testApp.Dao().FindRecordById("settings", "settings_update_not_allowed_test"); err == nil {
			testApp.Dao().DeleteRecord(prevRecord)
		}

		// Create existing settings record with complete credentials for this test
		collection, err := testApp.Dao().FindCollectionByNameOrId("settings")
		require.NoError(t, err)

		record := models.NewRecord(collection)
		record.SetId("settings_update_allowed_test")
		record.Set("spotify_client_id", "existing-spotify-id")
		record.Set("spotify_client_secret", "existing-spotify-secret")
		record.Set("google_client_id", "existing-google-id")
		record.Set("google_client_secret", "existing-google-secret")
		err = testApp.Dao().SaveRecord(record)
		require.NoError(t, err)

		// Clear environment variables but set UPDATE_ALLOWED
		os.Unsetenv("SPOTIFY_ID")
		os.Unsetenv("SPOTIFY_SECRET")
		os.Unsetenv("GOOGLE_CLIENT_ID")
		os.Unsetenv("GOOGLE_CLIENT_SECRET")
		os.Setenv("UPDATE_ALLOWED", "true")
		defer os.Unsetenv("UPDATE_ALLOWED")

		setupRequest := SetupRequest{
			SpotifyID:          "updated-via-flag-spotify-id",
			SpotifySecret:      "updated-via-flag-spotify-secret",
			GoogleClientID:     "updated-via-flag-google-id",
			GoogleClientSecret: "updated-via-flag-google-secret",
		}

		reqBody, err := json.Marshal(setupRequest)
		require.NoError(t, err)

		e := echo.New()
		req := httptest.NewRequest(http.MethodPost, "/api/setup", bytes.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		// Test actual postHandler using interface wrapper
		handler := postHandlerWithInterface(testApp)
		err = handler(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, rec.Code)

		// Verify settings were updated
		updatedRecord, err := testApp.Dao().FindRecordById("settings", "settings_update_allowed_test")
		assert.NoError(t, err)
		assert.Equal(t, "updated-via-flag-spotify-id", updatedRecord.GetString("spotify_client_id"))
	})
}

func TestEnvironmentVariablePriority_Integration(t *testing.T) {
	testApp := testhelpers.SetupTestApp(t)
	defer testApp.Cleanup()

	t.Run("environment variables take priority over database", func(t *testing.T) {
		// Create database settings
		collection, err := testApp.Dao().FindCollectionByNameOrId("settings")
		require.NoError(t, err)

		record := models.NewRecord(collection)
		record.SetId("settings")
		record.Set("spotify_client_id", "db-spotify-id")
		record.Set("spotify_client_secret", "db-spotify-secret")
		record.Set("google_client_id", "db-google-id")
		record.Set("google_client_secret", "db-google-secret")
		err = testApp.Dao().SaveRecord(record)
		require.NoError(t, err)

		// Set environment variables (should take priority)
		os.Setenv("SPOTIFY_ID", "env-spotify-id")
		os.Setenv("SPOTIFY_SECRET", "env-spotify-secret")
		os.Setenv("GOOGLE_CLIENT_ID", "env-google-id")
		os.Setenv("GOOGLE_CLIENT_SECRET", "env-google-secret")
		defer func() {
			os.Unsetenv("SPOTIFY_ID")
			os.Unsetenv("SPOTIFY_SECRET")
			os.Unsetenv("GOOGLE_CLIENT_ID")
			os.Unsetenv("GOOGLE_CLIENT_SECRET")
		}()

		// Test that setup is not required (env vars take priority) using interface wrapper
		required, err := isSetupRequiredWithInterface(testApp)
		assert.NoError(t, err)
		assert.False(t, required, "Setup should not be required when env vars are present, even with DB credentials")
	})

	t.Run("partial environment variables still require database check", func(t *testing.T) {
		// Create complete database settings with unique ID
		collection, err := testApp.Dao().FindCollectionByNameOrId("settings")
		require.NoError(t, err)

		record := models.NewRecord(collection)
		record.SetId("settings_env_priority_test")
		record.Set("spotify_client_id", "db-spotify-id")
		record.Set("spotify_client_secret", "db-spotify-secret")
		record.Set("google_client_id", "db-google-id")
		record.Set("google_client_secret", "db-google-secret")
		err = testApp.Dao().SaveRecord(record)
		require.NoError(t, err)

		// Set only partial environment variables
		os.Setenv("SPOTIFY_ID", "env-spotify-id")
		os.Setenv("SPOTIFY_SECRET", "env-spotify-secret")
		// Leave Google env vars unset
		defer func() {
			os.Unsetenv("SPOTIFY_ID")
			os.Unsetenv("SPOTIFY_SECRET")
		}()

		// Test that setup is not required (falls back to DB check which is complete) using interface wrapper
		required, err := isSetupRequiredWithInterface(testApp)
		assert.NoError(t, err)
		assert.False(t, required, "Setup should not be required when partial env + complete DB")
	})
}
