package handlers

import (
	"net"
	"net/url"
	"path"
	"strings"
)

// normalizeLoopbackFrontendHost rewrites 127.0.0.1 to localhost for UI redirects.
// Vite's dev server often accepts localhost:5173 but not 127.0.0.1:5173 on macOS.
func normalizeLoopbackFrontendHost(frontendURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(frontendURL))
	if err != nil || parsed.Host == "" {
		return frontendURL
	}

	host, port, err := net.SplitHostPort(parsed.Host)
	if err != nil {
		return frontendURL
	}

	if host != "127.0.0.1" {
		return frontendURL
	}

	parsed.Host = net.JoinHostPort("localhost", port)
	return parsed.String()
}

func buildFrontendDashboardRedirect(frontendURL, provider, status, message string) string {
	base := normalizeLoopbackFrontendHost(strings.TrimSpace(frontendURL))
	if base == "" {
		return buildRelativeDashboardRedirect(provider, status, message)
	}

	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return buildRelativeDashboardRedirect(provider, status, message)
	}

	basePath := strings.TrimSuffix(parsed.Path, "/")
	switch {
	case basePath == "":
		parsed.Path = "/dashboard"
	default:
		parsed.Path = path.Join(basePath, "dashboard")
		if !strings.HasPrefix(parsed.Path, "/") {
			parsed.Path = "/" + parsed.Path
		}
	}

	query := parsed.Query()
	if provider != "" && status != "" {
		query.Set(provider, status)
	}
	if strings.TrimSpace(message) != "" {
		query.Set("message", message)
	}
	parsed.RawQuery = query.Encode()
	parsed.Fragment = ""

	return parsed.String()
}

func buildRelativeDashboardRedirect(provider, status, message string) string {
	destination := "/dashboard"
	query := url.Values{}
	if provider != "" && status != "" {
		query.Set(provider, status)
	}
	if strings.TrimSpace(message) != "" {
		query.Set("message", message)
	}
	if encoded := query.Encode(); encoded != "" {
		return destination + "?" + encoded
	}
	return destination
}
