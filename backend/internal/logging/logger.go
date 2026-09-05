package logging

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Init creates a zerolog.Logger configured based on environment settings.
// Console formatting is used only when stdout is a TTY (interactive terminal).
// Redirected output (e.g. tee/dev.log) uses plain JSON without ANSI escape codes.
func Init(appEnv, logLevel, version string) zerolog.Logger {
	level := parseLevel(logLevel)
	zerolog.SetGlobalLevel(level)

	var writer io.Writer = os.Stdout
	if strings.ToLower(appEnv) == "development" && isCharDevice(os.Stdout) {
		writer = zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	}

	logger := zerolog.New(writer).With().Timestamp().Str("version", version).Logger()
	return logger.Level(level)
}

func isCharDevice(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func parseLevel(level string) zerolog.Level {
	lvl, err := zerolog.ParseLevel(strings.ToLower(strings.TrimSpace(level)))
	if err != nil {
		return zerolog.InfoLevel
	}
	return lvl
}
