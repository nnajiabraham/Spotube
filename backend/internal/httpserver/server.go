package httpserver

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/rs/zerolog"

	"github.com/manlikeabro/spotube/internal/config"
)

// New creates a configured Echo instance with standard middleware.
func New(cfg *config.Config, logger zerolog.Logger) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = customHTTPErrorHandler(e.DefaultHTTPErrorHandler)

	e.Pre(middleware.RemoveTrailingSlash())

	e.Use(requestIDMiddleware())
	e.Use(accessLogMiddleware(logger))
	e.Use(middleware.Recover())
	e.Use(corsMiddleware(cfg))

	return e
}

func requestIDMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			res := c.Response()

			requestID := req.Header.Get(echo.HeaderXRequestID)
			if strings.TrimSpace(requestID) == "" {
				requestID = uuid.NewString()
			}

			res.Header().Set(echo.HeaderXRequestID, requestID)

			c.Set(echo.HeaderXRequestID, requestID)
			return next(c)
		}
	}
}

func accessLogMiddleware(logger zerolog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)

			status := c.Response().Status
			if he, ok := err.(*echo.HTTPError); ok && he.Code != 0 {
				status = he.Code
			}
			latency := time.Since(start)
			requestID, _ := c.Get(echo.HeaderXRequestID).(string)

			event := logger.Info()
			if err != nil {
				event = logger.Error().Err(err)
			}

			event = event.
				Str("request_id", requestID).
				Str("method", c.Request().Method).
				Str("path", c.Path()).
				Int("status", status).
				Dur("latency_ms", latency)

			ua := c.Request().UserAgent()
			if ua != "" {
				event = event.Str("user_agent", ua)
			}

			event.Msg("request completed")

			return err
		}
	}
}

func corsMiddleware(cfg *config.Config) echo.MiddlewareFunc {
	return middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: cfg.CORSAllowOrigins,
		AllowMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPatch,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
		},
		AllowHeaders: []string{
			echo.HeaderAccept,
			echo.HeaderContentType,
			echo.HeaderAuthorization,
			"X-CSRF-Token",
		},
		AllowCredentials: true,
	})
}

func customHTTPErrorHandler(defaultHandler echo.HTTPErrorHandler) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		requestID, _ := c.Get(echo.HeaderXRequestID).(string)
		c.Response().Header().Set(echo.HeaderXRequestID, requestID)
		defaultHandler(err, c)
	}
}
