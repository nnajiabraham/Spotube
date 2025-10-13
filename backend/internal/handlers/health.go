package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"github.com/manlikeabro/spotube/internal/config"
)

// HealthDatabase defines the minimal contract required for health checks.
type HealthDatabase interface {
	PingContext(context.Context) error
}

// HealthHandler returns basic status information about the service.
type HealthHandler struct {
	DB     HealthDatabase
	Logger zerolog.Logger
	Config *config.Config
}

// RegisterHealth registers the health endpoint on the Echo server.
func RegisterHealth(e *echo.Echo, handler *HealthHandler) {
	e.GET("/api/health", handler.Handle)
}

// Handle responds to GET /api/health requests.
func (h *HealthHandler) Handle(c echo.Context) error {
	if err := h.DB.PingContext(c.Request().Context()); err != nil {
		h.Logger.Error().Err(err).Msg("database ping failed")
		return c.JSON(http.StatusInternalServerError, map[string]any{
			"error": map[string]string{
				"code":    "database_unavailable",
				"message": "database connection unavailable",
			},
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"version":   h.Config.Version,
		"service":   "spotube",
	})
}
