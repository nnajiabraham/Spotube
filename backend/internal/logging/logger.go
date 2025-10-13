package logging

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Init creates a zerolog.Logger configured based on environment settings.
func Init(appEnv, logLevel, version string) zerolog.Logger {
	level := parseLevel(logLevel)
	zerolog.SetGlobalLevel(level)

	output := zerolog.New(os.Stdout)
	if strings.ToLower(appEnv) == "development" {
		output = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})
	}

	logger := output.With().Timestamp().Str("version", version).Logger()
	return logger.Level(level)
}

func parseLevel(level string) zerolog.Level {
	lvl, err := zerolog.ParseLevel(strings.ToLower(strings.TrimSpace(level)))
	if err != nil {
		return zerolog.InfoLevel
	}
	return lvl
}
