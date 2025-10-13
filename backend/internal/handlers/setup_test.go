package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

type fakeSettingsRepo struct {
	settings    *SettingsRecord
	getErr      error
	upsertErr   error
	lastPayload SaveSettingsRequest
}

func (f *fakeSettingsRepo) GetSettings() (*SettingsRecord, error) {
	return f.settings, f.getErr
}

func (f *fakeSettingsRepo) UpsertSettings(req SaveSettingsRequest) error {
	f.lastPayload = req
	return f.upsertErr
}

func TestSetupRequiredTrueWhenNoSettings(t *testing.T) {
	repo := &fakeSettingsRepo{}
	handler := &SetupHandler{Repo: repo}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := handler.SetupRequired(ctx); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	if rec.Body.String() != "{\"required\":true}\n" {
		t.Fatalf("expected required response true, got %s", rec.Body.String())
	}
}

func TestSetupRequiredFalseWhenSettingsPresent(t *testing.T) {
	repo := &fakeSettingsRepo{settings: &SettingsRecord{
		SpotifyClientID:     nullableStringTest("client"),
		SpotifyClientSecret: nullableStringTest("secret"),
		GoogleClientID:      nullableStringTest("gid"),
		GoogleClientSecret:  nullableStringTest("gsecret"),
	}}
	handler := &SetupHandler{Repo: repo}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := handler.SetupRequired(ctx); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Body.String() != "{\"required\":false}\n" {
		t.Fatalf("expected required response false, got %s", rec.Body.String())
	}
}

func TestSaveSettingsValidation(t *testing.T) {
	repo := &fakeSettingsRepo{}
	handler := &SetupHandler{Repo: repo}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{\"spotify_client_id\":\"id\"}"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	err := handler.SaveSettings(ctx)
	if err == nil {
		t.Fatalf("expected validation error")
	}

	httpErr, ok := err.(*echo.HTTPError)
	if !ok || httpErr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 http error, got %#v", err)
	}
}

func TestSaveSettingsSuccess(t *testing.T) {
	repo := &fakeSettingsRepo{}
	handler := &SetupHandler{Repo: repo}

	payload := `{"spotify_client_id":"id","spotify_client_secret":"sec","google_client_id":"gid","google_client_secret":"gsec"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := handler.SaveSettings(ctx); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}

	if repo.lastPayload.SpotifyClientID != "id" {
		t.Fatalf("payload not passed to repo")
	}
}

func nullableStringTest(value string) sql.NullString {
	return sql.NullString{String: value, Valid: true}
}
