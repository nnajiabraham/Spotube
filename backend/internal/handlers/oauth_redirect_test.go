package handlers

import "testing"

func TestBuildFrontendDashboardRedirect(t *testing.T) {
	tests := []struct {
		name        string
		frontendURL string
		provider    string
		status      string
		message     string
		expected    string
	}{
		{
			name:        "simple frontend URL",
			frontendURL: "http://localhost:5173",
			provider:    "spotify",
			status:      "connected",
			expected:    "http://localhost:5173/dashboard?spotify=connected",
		},
		{
			name:        "127.0.0.1 frontend URL uses localhost for dev redirect",
			frontendURL: "http://127.0.0.1:5173",
			provider:    "spotify",
			status:      "connected",
			expected:    "http://localhost:5173/dashboard?spotify=connected",
		},
		{
			name:        "frontend URL with base path",
			frontendURL: "https://example.com/app",
			provider:    "youtube",
			status:      "connected",
			expected:    "https://example.com/app/dashboard?youtube=connected",
		},
		{
			name:        "invalid frontend URL falls back to relative",
			frontendURL: "localhost:5173",
			provider:    "spotify",
			status:      "connected",
			expected:    "/dashboard?spotify=connected",
		},
		{
			name:        "includes optional message",
			frontendURL: "",
			provider:    "spotify",
			status:      "error",
			message:     "bad callback",
			expected:    "/dashboard?message=bad+callback&spotify=error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := buildFrontendDashboardRedirect(test.frontendURL, test.provider, test.status, test.message)
			if actual != test.expected {
				t.Fatalf("unexpected redirect URL: expected %q, got %q", test.expected, actual)
			}
		})
	}
}
