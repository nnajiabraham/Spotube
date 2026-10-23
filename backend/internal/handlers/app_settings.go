package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/manlikeabro/spotube/internal/settings"
)

// AppSettingsHandler serves the runtime settings (the Settings page).
type AppSettingsHandler struct {
	Store *settings.Store
}

func RegisterAppSettingsRoutes(g *echo.Group, h *AppSettingsHandler) {
	g.GET("", h.List)
	g.PATCH("", h.Update)
}

func (h *AppSettingsHandler) List(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]any{"items": h.Store.List()})
}

// Update applies {"key": value, ...}. Values may be JSON booleans, numbers or strings. All are
// validated before any is applied.
func (h *AppSettingsHandler) Update(c echo.Context) error {
	var body map[string]any
	if err := c.Bind(&body); err != nil || len(body) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "send an object of settings to change")
	}
	raw := make(map[string]string, len(body))
	for key, v := range body {
		switch t := v.(type) {
		case bool:
			raw[key] = strconv.FormatBool(t)
		case float64:
			raw[key] = strconv.FormatFloat(t, 'f', -1, 64)
		case string:
			raw[key] = t
		default:
			return echo.NewHTTPError(http.StatusUnprocessableEntity, key+": unsupported value")
		}
	}
	if err := h.Store.Validate(raw); err != nil {
		code := http.StatusUnprocessableEntity
		if errors.Is(err, settings.ErrUnknown) {
			code = http.StatusBadRequest
		}
		return echo.NewHTTPError(code, err.Error())
	}
	for key, v := range raw {
		if _, err := h.Store.Set(key, v); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}
	return h.List(c)
}
