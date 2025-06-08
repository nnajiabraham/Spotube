package setupwizard

import (
	"net/http"
	"os"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
)

// StatusResponse represents the response for /api/setup/status
type StatusResponse struct {
	Required bool `json:"required"`
}

// SetupRequest represents the request body for /api/setup
type SetupRequest struct {
	SpotifyID          string `json:"spotify_id"`
	SpotifySecret      string `json:"spotify_secret"`
	GoogleClientID     string `json:"google_client_id"`
	GoogleClientSecret string `json:"google_client_secret"`
}

// Register registers the setup wizard routes with the PocketBase app
func Register(app *pocketbase.PocketBase) {
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.GET("/api/setup/status", statusHandler(app))
		e.Router.POST("/api/setup", postHandler(app))
		return nil
	})
}

// statusHandler returns whether setup is required
func statusHandler(app *pocketbase.PocketBase) echo.HandlerFunc {
	return func(c echo.Context) error {
		required, err := isSetupRequired(app)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to check setup status")
		}

		return c.JSON(http.StatusOK, StatusResponse{Required: required})
	}
}

// postHandler handles setup credential submission
func postHandler(app *pocketbase.PocketBase) echo.HandlerFunc {
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
		setupRequired, err := isSetupRequired(app)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to check setup status")
		}

		updateAllowed := os.Getenv("UPDATE_ALLOWED") == "true"
		if !setupRequired && !updateAllowed {
			return echo.NewHTTPError(http.StatusConflict, "Setup already completed and updates not allowed")
		}

		// Save credentials to settings collection
		if err := saveCredentials(app, req); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to save credentials")
		}

		return c.NoContent(http.StatusNoContent)
	}
}

// isSetupRequired checks if any of the 4 credentials are missing from both env and DB
func isSetupRequired(app *pocketbase.PocketBase) (bool, error) {
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

	// Check database for existing credentials
	dao := daos.New(app.Dao().DB())
	record, err := dao.FindRecordById("settings", "settings")
	if err != nil {
		// If record doesn't exist, setup is required
		return true, nil
	}

	// Check if all required fields have values
	dbFields := []string{"spotify_client_id", "spotify_client_secret", "google_client_id", "google_client_secret"}
	for _, field := range dbFields {
		if record.GetString(field) == "" {
			return true, nil // Setup required if any field is empty
		}
	}

	return false, nil // All credentials present in DB
}

// saveCredentials saves the provided credentials to the settings collection
func saveCredentials(app *pocketbase.PocketBase, req SetupRequest) error {
	dao := daos.New(app.Dao().DB())

	// Try to find existing settings record
	record, err := dao.FindRecordById("settings", "settings")
	if err != nil {
		// Create new record if it doesn't exist
		collection, err := dao.FindCollectionByNameOrId("settings")
		if err != nil {
			return err
		}

		record = models.NewRecord(collection)
		record.SetId("settings")
	}

	// Set the credential fields
	record.Set("spotify_client_id", req.SpotifyID)
	record.Set("spotify_client_secret", req.SpotifySecret)
	record.Set("google_client_id", req.GoogleClientID)
	record.Set("google_client_secret", req.GoogleClientSecret)

	// Save the record
	return dao.SaveRecord(record)
}
