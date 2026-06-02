package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration for the Spotube backend.
type Config struct {
	AppEnv              string
	Port                string
	DBPath              string
	LogLevel            string
	PublicURL           string
	FrontendURL         string
	Version             string
	CORSAllowOrigins    []string
	SpotifyClientID     string
	SpotifyClientSecret string
	GoogleClientID      string
	GoogleClientSecret  string
	SessionCookieName   string
	SessionSecret       string
	SessionTTLSeconds   int
	SessionSecure       bool
	SyncWorkersEnabled       bool
	SyncAnalysisCronSpec     string
	SyncExecutorAutoEnabled  bool
	SyncExecutorCronSpec     string
}

const (
	defaultPort          = "8090"
	defaultDBPath        = "./data/spotube.db"
	defaultLogLevel      = "info"
	defaultFrontend      = "http://localhost:5173"
	defaultAppEnv        = "development"
	defaultVersion       = "dev"
	defaultSessionCookie = "spotube_session"
	defaultSessionSecret = "spotube-dev-session-secret-change-me"
	defaultSessionTTL         = 30 * 24 * 60 * 60 // 30 days
	defaultSyncAnalysisCron   = "0 * * * * *"     // every minute (6-field cron with seconds)
	defaultSyncExecutorCron   = "*/10 * * * * *"  // every 10 seconds (only if auto executor enabled)
)

// Load reads configuration from environment variables, providing sensible defaults.
func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:              getEnv("APP_ENV", defaultAppEnv),
		Port:                getEnv("PORT", defaultPort),
		DBPath:              getEnv("DB_PATH", defaultDBPath),
		LogLevel:            getEnv("LOG_LEVEL", defaultLogLevel),
		PublicURL:           os.Getenv("PUBLIC_URL"),
		FrontendURL:         getEnv("FRONTEND_URL", defaultFrontend),
		Version:             getEnv("VERSION", defaultVersion),
		CORSAllowOrigins:    splitAndTrim(getEnv("CORS_ALLOW_ORIGINS", defaultFrontend)),
		SpotifyClientID:     os.Getenv("SPOTIFY_CLIENT_ID"),
		SpotifyClientSecret: os.Getenv("SPOTIFY_CLIENT_SECRET"),
		GoogleClientID:      os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret:  os.Getenv("GOOGLE_CLIENT_SECRET"),
		SessionCookieName:   getEnv("SESSION_COOKIE_NAME", defaultSessionCookie),
		SessionSecret:       getEnv("SESSION_SECRET", defaultSessionSecret),
	}

	ttl, err := parseIntEnv("SESSION_TTL_SECONDS", defaultSessionTTL)
	if err != nil {
		return nil, fmt.Errorf("invalid SESSION_TTL_SECONDS: %w", err)
	}
	cfg.SessionTTLSeconds = ttl

	secure, err := parseBoolEnv("SESSION_SECURE", false)
	if err != nil {
		return nil, fmt.Errorf("invalid SESSION_SECURE: %w", err)
	}
	cfg.SessionSecure = secure

	syncWorkers, err := parseBoolEnv("SYNC_WORKERS_ENABLED", false)
	if err != nil {
		return nil, fmt.Errorf("invalid SYNC_WORKERS_ENABLED: %w", err)
	}
	cfg.SyncWorkersEnabled = syncWorkers

	cfg.SyncAnalysisCronSpec = getEnv("SYNC_ANALYSIS_CRON_SPEC", defaultSyncAnalysisCron)

	executorAuto, err := parseBoolEnv("SYNC_EXECUTOR_AUTO_ENABLED", false)
	if err != nil {
		return nil, fmt.Errorf("invalid SYNC_EXECUTOR_AUTO_ENABLED: %w", err)
	}
	cfg.SyncExecutorAutoEnabled = executorAuto
	cfg.SyncExecutorCronSpec = getEnv("SYNC_EXECUTOR_CRON_SPEC", defaultSyncExecutorCron)

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func splitAndTrim(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parseIntEnv(key string, fallback int) (int, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, errors.New("value must be non-negative")
	}
	return value, nil
}

func parseBoolEnv(key string, fallback bool) (bool, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, err
	}
	return value, nil
}

func validate(cfg *Config) error {
	if strings.TrimSpace(cfg.Port) == "" {
		return errors.New("port must not be empty")
	}
	if strings.TrimSpace(cfg.DBPath) == "" {
		return errors.New("db path must not be empty")
	}
	if len(cfg.CORSAllowOrigins) == 0 {
		return errors.New("at least one CORS origin required")
	}
	if cfg.AppEnv == "development" && strings.Contains(cfg.FrontendURL, "127.0.0.1") {
		fmt.Fprintf(os.Stderr, "warning: FRONTEND_URL uses 127.0.0.1; prefer http://localhost:5173 where Vite listens (OAuth redirects will normalize to localhost)\n")
	}
	return nil
}
