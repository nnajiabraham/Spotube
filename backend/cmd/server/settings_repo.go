package main

import (
	"database/sql"

	"github.com/manlikeabro/spotube/internal/auth"
)

// sqliteSettingsRepo implements auth.CredentialProvider for the main server.
type sqliteSettingsRepo struct {
	db *sql.DB
}

func (r *sqliteSettingsRepo) GetSettings() (*auth.SettingsRecord, error) {
	row := r.db.QueryRow(`SELECT spotify_client_id, spotify_client_secret, google_client_id, google_client_secret FROM settings WHERE id = '1'`)

	var record auth.SettingsRecord
	err := row.Scan(&record.SpotifyClientID, &record.SpotifyClientSecret, &record.GoogleClientID, &record.GoogleClientSecret)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &record, nil
}
