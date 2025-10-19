package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/manlikeabro/spotube/internal/migrate"
	"github.com/manlikeabro/spotube/internal/sqliteconn"
)

func TestSetupRoutesEndToEnd(t *testing.T) {
	db := openTempDB(t)
	defer db.Close()

	if err := migrate.Up(db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	defer migrate.Down(db)

	e := echo.New()
	handler := NewSetupHandler(db, nil)
	RegisterSetupRoutes(e.Group("/api/setup"), handler)

	// initial state should require setup
	req := httptest.NewRequest(http.MethodGet, "/api/setup/required", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assertJSON(t, rec, http.StatusOK, map[string]any{"required": true})

	payload := map[string]string{
		"spotify_client_id":     "id",
		"spotify_client_secret": "secret",
		"google_client_id":      "gid",
		"google_client_secret":  "gsecret",
	}
	body, _ := json.Marshal(payload)
	req = httptest.NewRequest(http.MethodPost, "/api/setup/save", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/setup/required", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assertJSON(t, rec, http.StatusOK, map[string]any{"required": false})
}

func openTempDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "setup.db")
	db, err := sqliteconn.OpenWithPragmas(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func assertJSON(t *testing.T, rec *httptest.ResponseRecorder, status int, expected map[string]any) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("expected status %d, got %d", status, rec.Code)
	}
	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	for k, v := range expected {
		if got[k] != v {
			t.Fatalf("expected %s=%v, got %v", k, v, got[k])
		}
	}
}
