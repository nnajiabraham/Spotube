package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"github.com/manlikeabro/spotube/internal/config"
)

func TestNewServerConfiguresMiddleware(t *testing.T) {
	cfg := &config.Config{
		CORSAllowOrigins: []string{"http://localhost"},
	}
	logger := zerolog.Nop()

	server := New(cfg, logger)

	if !server.HideBanner || !server.HidePort {
		t.Fatalf("expected HideBanner and HidePort to be true")
	}

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	server.GET("/test", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	server.ServeHTTP(rec, req)

	if rec.Header().Get(echo.HeaderXRequestID) == "" {
		t.Errorf("expected X-Request-ID header to be set")
	}
}
