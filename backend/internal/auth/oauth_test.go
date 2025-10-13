package auth

import (
	"database/sql"
	"testing"
)

type fakeSettingsProvider struct {
	record *SettingsRecord
	err    error
}

func (f *fakeSettingsProvider) GetSettings() (*SettingsRecord, error) {
	return f.record, f.err
}

func TestLoadCredentialsFromSettings(t *testing.T) {
	repo := &fakeSettingsProvider{record: &SettingsRecord{
		SpotifyClientID:     sql.NullString{String: "id", Valid: true},
		SpotifyClientSecret: sql.NullString{String: "secret", Valid: true},
	}}

	id, secret, err := LoadCredentials(repo, "spotify")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if id != "id" || secret != "secret" {
		t.Fatalf("expected credentials from settings, got %s/%s", id, secret)
	}
}

func TestLoadCredentialsFallsBackToEnv(t *testing.T) {
	t.Setenv("SPOTIFY_CLIENT_ID", "env-id")
	t.Setenv("SPOTIFY_CLIENT_SECRET", "env-secret")

	repo := &fakeSettingsProvider{record: &SettingsRecord{}}

	id, secret, err := LoadCredentials(repo, "spotify")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if id != "env-id" || secret != "env-secret" {
		t.Fatalf("expected env credentials, got %s/%s", id, secret)
	}
}

func TestLoadCredentialsErrorWhenMissing(t *testing.T) {
	t.Setenv("SPOTIFY_CLIENT_ID", "")
	t.Setenv("SPOTIFY_CLIENT_SECRET", "")

	repo := &fakeSettingsProvider{record: &SettingsRecord{}}

	if _, _, err := LoadCredentials(repo, "spotify"); err != ErrNoSpotifyCredentials {
		t.Fatalf("expected ErrNoSpotifyCredentials, got %v", err)
	}
}
