package logging

import (
	"testing"

	"github.com/rs/zerolog"
)

func TestInitDevelopmentMode(t *testing.T) {
	t.Setenv("ZERLOG_GLOBAL_LEVEL", "")
	Init("development", "debug", "test-version")

	if got := zerolog.GlobalLevel(); got != zerolog.DebugLevel {
		t.Fatalf("expected global level debug, got %s", got)
	}
}

func TestInitInvalidLevel(t *testing.T) {
	Init("production", "unknown", "test")

	if zerolog.GlobalLevel() != zerolog.InfoLevel {
		t.Fatalf("expected info level fallback, got %s", zerolog.GlobalLevel())
	}
}
