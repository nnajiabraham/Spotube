package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedirectIfOAuthHostMismatch(t *testing.T) {
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/spotify/login?foo=bar", nil)
	req.Host = "localhost:8090"
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := redirectIfOAuthHostMismatch(ctx, "http://127.0.0.1:8090")
	require.NoError(t, err)
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "http://127.0.0.1:8090/api/auth/spotify/login?foo=bar", rec.Header().Get("Location"))
}

func TestRedirectIfOAuthHostMismatchSkipsWhenAligned(t *testing.T) {
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/api/auth/spotify/login", nil)
	req.Host = "127.0.0.1:8090"
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := redirectIfOAuthHostMismatch(ctx, "http://127.0.0.1:8090")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPublicURLFromRedirectURI(t *testing.T) {
	assert.Equal(t, "http://127.0.0.1:8090", publicURLFromRedirectURI("http://127.0.0.1:8090/api/auth/spotify/callback"))
}
