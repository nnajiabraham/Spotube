package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("PORT", "")
	t.Setenv("DB_PATH", "")
	t.Setenv("FRONTEND_URL", "")
	t.Setenv("CORS_ALLOW_ORIGINS", "")
	t.Setenv("SESSION_COOKIE_NAME", "")
	t.Setenv("SESSION_TTL_SECONDS", "")
	t.Setenv("SESSION_SECURE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.AppEnv != "development" {
		t.Errorf("expected AppEnv default 'development', got %q", cfg.AppEnv)
	}
	if cfg.Port != "8090" {
		t.Errorf("expected Port default '8090', got %q", cfg.Port)
	}
	if cfg.DBPath != "./data/spotube.db" {
		t.Errorf("expected DBPath default './data/spotube.db', got %q", cfg.DBPath)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("expected LogLevel default 'info', got %q", cfg.LogLevel)
	}
	if cfg.FrontendURL != "http://localhost:5173" {
		t.Errorf("expected FrontendURL default 'http://localhost:5173', got %q", cfg.FrontendURL)
	}
	if cfg.SessionCookieName != "spotube_session" {
		t.Errorf("expected SessionCookieName default 'spotube_session', got %q", cfg.SessionCookieName)
	}
	if cfg.SessionTTLSeconds != 2592000 {
		t.Errorf("expected SessionTTLSeconds default 2592000, got %d", cfg.SessionTTLSeconds)
	}
	if cfg.SessionSecure {
		t.Errorf("expected SessionSecure default false")
	}
	if len(cfg.CORSAllowOrigins) != 1 || cfg.CORSAllowOrigins[0] != "http://localhost:5173" {
		t.Errorf("expected default CORS origin 'http://localhost:5173', got %v", cfg.CORSAllowOrigins)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("PORT", "9000")
	t.Setenv("DB_PATH", "/tmp/test.db")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("FRONTEND_URL", "https://example.com")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://a.com, https://b.com ")
	t.Setenv("SESSION_COOKIE_NAME", "custom")
	t.Setenv("SESSION_TTL_SECONDS", "120")
	t.Setenv("SESSION_SECURE", "true")
	t.Setenv("VERSION", "1.2.3")
	t.Setenv("SPOTIFY_CLIENT_ID", "id")
	t.Setenv("SPOTIFY_CLIENT_SECRET", "secret")
	t.Setenv("GOOGLE_CLIENT_ID", "gid")
	t.Setenv("GOOGLE_CLIENT_SECRET", "gsecret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.AppEnv != "production" {
		t.Errorf("expected AppEnv 'production', got %q", cfg.AppEnv)
	}
	if cfg.Port != "9000" {
		t.Errorf("expected Port '9000', got %q", cfg.Port)
	}
	if cfg.DBPath != "/tmp/test.db" {
		t.Errorf("expected DBPath '/tmp/test.db', got %q", cfg.DBPath)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected LogLevel 'debug', got %q", cfg.LogLevel)
	}
	if cfg.FrontendURL != "https://example.com" {
		t.Errorf("expected FrontendURL 'https://example.com', got %q", cfg.FrontendURL)
	}
	if cfg.SessionCookieName != "custom" {
		t.Errorf("expected SessionCookieName 'custom', got %q", cfg.SessionCookieName)
	}
	if cfg.SessionTTLSeconds != 120 {
		t.Errorf("expected SessionTTLSeconds 120, got %d", cfg.SessionTTLSeconds)
	}
	if !cfg.SessionSecure {
		t.Errorf("expected SessionSecure true")
	}
	if cfg.Version != "1.2.3" {
		t.Errorf("expected Version '1.2.3', got %q", cfg.Version)
	}
	if cfg.SpotifyClientID != "id" || cfg.SpotifyClientSecret != "secret" {
		t.Errorf("expected Spotify credentials to be set, got %q/%q", cfg.SpotifyClientID, cfg.SpotifyClientSecret)
	}
	if cfg.GoogleClientID != "gid" || cfg.GoogleClientSecret != "gsecret" {
		t.Errorf("expected Google credentials to be set, got %q/%q", cfg.GoogleClientID, cfg.GoogleClientSecret)
	}
	if len(cfg.CORSAllowOrigins) != 2 {
		t.Fatalf("expected 2 CORS origins, got %v", cfg.CORSAllowOrigins)
	}
	if cfg.CORSAllowOrigins[0] != "https://a.com" || cfg.CORSAllowOrigins[1] != "https://b.com" {
		t.Errorf("unexpected CORS origins %v", cfg.CORSAllowOrigins)
	}
}

func TestInvalidValues(t *testing.T) {
	t.Setenv("SESSION_TTL_SECONDS", "-10")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for negative SESSION_TTL_SECONDS")
	}

	t.Setenv("SESSION_TTL_SECONDS", "10")
	t.Setenv("SESSION_SECURE", "notabool")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid SESSION_SECURE")
	}

	t.Setenv("SESSION_SECURE", "true")
	t.Setenv("CORS_ALLOW_ORIGINS", "   ")

	if _, err := Load(); err == nil {
		t.Fatal("expected error for empty CORS origins")
	}

	// clear env to avoid leaking to other tests
	os.Clearenv()
}
