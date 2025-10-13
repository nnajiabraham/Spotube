package handlers

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

const settingsSingletonID = "1"

// NewSetupHandler constructs a setup handler backed by SQLite.
func NewSetupHandler(db *sql.DB) *SetupHandler {
	return &SetupHandler{Repo: &sqliteSettingsRepo{db: db}}
}

// SettingsRepository defines persistence operations required by setup handlers.
type SettingsRepository interface {
	GetSettings() (*SettingsRecord, error)
	UpsertSettings(req SaveSettingsRequest) error
}

// SettingsRecord represents the persisted settings row.
type SettingsRecord struct {
	SpotifyClientID     sql.NullString
	SpotifyClientSecret sql.NullString
	GoogleClientID      sql.NullString
	GoogleClientSecret  sql.NullString
	Created             int64
	Updated             int64
}

// SaveSettingsRequest is the payload accepted from the frontend.
type SaveSettingsRequest struct {
	SpotifyClientID     string `json:"spotify_client_id"`
	SpotifyClientSecret string `json:"spotify_client_secret"`
	GoogleClientID      string `json:"google_client_id"`
	GoogleClientSecret  string `json:"google_client_secret"`
}

// SetupHandler orchestrates setup-related endpoints.
type SetupHandler struct {
	Repo SettingsRepository
}

// RegisterSetupRoutes registers setup routes on the given echo group.
func RegisterSetupRoutes(group *echo.Group, handler *SetupHandler) {
	group.POST("/save", handler.SaveSettings)
	group.GET("/required", handler.SetupRequired)
}

// SaveSettings persists the OAuth credential configuration.
func (h *SetupHandler) SaveSettings(c echo.Context) error {
	var payload SaveSettingsRequest
	if err := c.Bind(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	if err := validateSettingsPayload(payload); err != nil {
		return err
	}

	if err := h.Repo.UpsertSettings(payload); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save settings")
	}

	return c.NoContent(http.StatusNoContent)
}

// SetupRequired indicates whether the setup wizard should run.
func (h *SetupHandler) SetupRequired(c echo.Context) error {
	record, err := h.Repo.GetSettings()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch settings")
	}

	required := record == nil || !record.SpotifyClientID.Valid || !record.SpotifyClientSecret.Valid || !record.GoogleClientID.Valid || !record.GoogleClientSecret.Valid

	return c.JSON(http.StatusOK, map[string]bool{"required": required})
}

func validateSettingsPayload(payload SaveSettingsRequest) error {
	fields := []string{payload.SpotifyClientID, payload.SpotifyClientSecret, payload.GoogleClientID, payload.GoogleClientSecret}
	filled := 0
	for _, value := range fields {
		if strings.TrimSpace(value) != "" {
			filled++
		}
	}

	if filled != 0 && filled != len(fields) {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "either provide all credentials or leave all empty")
	}
	return nil
}

func nullableString(value string) sql.NullString {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return sql.NullString{String: "", Valid: false}
	}
	return sql.NullString{String: trimmed, Valid: true}
}

func currentTimestamp() int64 {
	return time.Now().Unix()
}

// sqliteSettingsRepo implements SettingsRepository against SQLite.
type sqliteSettingsRepo struct {
	db *sql.DB
}

func (r *sqliteSettingsRepo) GetSettings() (*SettingsRecord, error) {
	row := r.db.QueryRow(`SELECT spotify_client_id, spotify_client_secret, google_client_id, google_client_secret, created, updated FROM settings WHERE id = ?`, settingsSingletonID)
	var record SettingsRecord
	if err := row.Scan(&record.SpotifyClientID, &record.SpotifyClientSecret, &record.GoogleClientID, &record.GoogleClientSecret, &record.Created, &record.Updated); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (r *sqliteSettingsRepo) UpsertSettings(req SaveSettingsRequest) error {
	createdAt := currentTimestamp()
	spotifyID := nullableString(req.SpotifyClientID)
	spotifySecret := nullableString(req.SpotifyClientSecret)
	googleID := nullableString(req.GoogleClientID)
	googleSecret := nullableString(req.GoogleClientSecret)

	_, err := r.db.Exec(`INSERT INTO settings (id, spotify_client_id, spotify_client_secret, google_client_id, google_client_secret, created, updated)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			spotify_client_id = excluded.spotify_client_id,
			spotify_client_secret = excluded.spotify_client_secret,
			google_client_id = excluded.google_client_id,
			google_client_secret = excluded.google_client_secret,
			updated = excluded.updated
	`, settingsSingletonID, spotifyID, spotifySecret, googleID, googleSecret, createdAt, createdAt)
	return err
}
