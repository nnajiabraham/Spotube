package handlers

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
)

// publicURLFromRedirectURI returns the scheme+host base for an OAuth redirect URI.
func publicURLFromRedirectURI(redirectURI string) string {
	parsed, err := url.Parse(strings.TrimSpace(redirectURI))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

// redirectIfOAuthHostMismatch sends the browser to the canonical OAuth host when the
// request arrived on a different host (e.g. localhost vs 127.0.0.1). OAuth state cookies
// are host-scoped, so login and callback must share the same host as PUBLIC_URL.
func redirectIfOAuthHostMismatch(c echo.Context, publicURL string) error {
	base := strings.TrimSpace(publicURL)
	if base == "" {
		return nil
	}

	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" {
		return nil
	}

	if strings.EqualFold(c.Request().Host, parsed.Host) {
		return nil
	}

	destination := *parsed
	destination.Path = c.Request().URL.Path
	destination.RawQuery = c.Request().URL.RawQuery
	return c.Redirect(http.StatusFound, destination.String())
}
