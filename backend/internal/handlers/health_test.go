package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"

	"github.com/manlikeabro/spotube/internal/config"
)

type mockDB struct {
	err error
}

func (m *mockDB) PingContext(context.Context) error {
	return m.err
}

func TestHealthHandlerSuccess(t *testing.T) {
	e := echo.New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	ctx := e.NewContext(req, rec)

	cfg := &config.Config{Version: "test"}
	handler := &HealthHandler{
		DB:     &mockDB{},
		Logger: zerolog.Nop(),
		Config: cfg,
	}

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestHealthHandlerDBFailure(t *testing.T) {
	e := echo.New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	ctx := e.NewContext(req, rec)

	handler := &HealthHandler{
		DB:     &mockDB{err: errors.New("failed")},
		Logger: zerolog.Nop(),
		Config: &config.Config{Version: "test"},
	}

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rec.Code)
	}
}
