package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/migrate"
	"github.com/manlikeabro/spotube/internal/sqliteconn"
)

func setupActivityLogsTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "activity_logs_test.db")

	db, err := sqliteconn.OpenWithPragmas(dbPath)
	require.NoError(t, err)

	require.NoError(t, migrate.Up(db))

	cleanup := func() {
		migrate.Down(db)
		db.Close()
	}
	return db, cleanup
}

func TestActivityLogsList(t *testing.T) {
	db, cleanup := setupActivityLogsTestDB(t)
	defer cleanup()

	// Seed test data
	now := time.Now().Unix()

	// Create some activity logs
	_, err := db.Exec(`INSERT INTO activity_logs (id, level, message, mapping_id, job_type, created) VALUES (?, ?, ?, ?, ?, ?)`,
		"log1", "info", "Analysis started", "mapping1", "analysis", now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO activity_logs (id, level, message, mapping_id, job_type, created) VALUES (?, ?, ?, ?, ?, ?)`,
		"log2", "error", "Execution failed", "mapping1", "executor", now+1)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO activity_logs (id, level, message, job_type, created) VALUES (?, ?, ?, ?, ?)`,
		"log3", "info", "System startup", "system", now+2)
	require.NoError(t, err)

	handler := NewActivityLogsHandler(db)
	e := echo.New()
	RegisterActivityLogsRoutes(e.Group("/api/collections/activity_logs/records"), handler)

	// Test list all (should be ordered by created desc)
	req := httptest.NewRequest(http.MethodGet, "/api/collections/activity_logs/records?page=1&per_page=10", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response ActivityLogsListResponse
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, 1, response.Page)
	assert.Equal(t, 10, response.PerPage)
	assert.Equal(t, int64(3), response.TotalItems)
	assert.Len(t, response.Items, 3)

	// Should be ordered by created desc
	assert.Equal(t, "log3", response.Items[0].ID)
	assert.Equal(t, "log2", response.Items[1].ID)
	assert.Equal(t, "log1", response.Items[2].ID)
}

func TestActivityLogsListFiltered(t *testing.T) {
	db, cleanup := setupActivityLogsTestDB(t)
	defer cleanup()

	// Seed test data
	now := time.Now().Unix()

	_, err := db.Exec(`INSERT INTO activity_logs (id, level, message, mapping_id, job_type, created) VALUES (?, ?, ?, ?, ?, ?)`,
		"log1", "info", "Analysis started", "mapping1", "analysis", now)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO activity_logs (id, level, message, mapping_id, job_type, created) VALUES (?, ?, ?, ?, ?, ?)`,
		"log2", "error", "Execution failed", "mapping2", "executor", now+1)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO activity_logs (id, level, message, job_type, created) VALUES (?, ?, ?, ?, ?)`,
		"log3", "warn", "System warning", "system", now+2)
	require.NoError(t, err)

	handler := NewActivityLogsHandler(db)
	e := echo.New()
	RegisterActivityLogsRoutes(e.Group("/api/collections/activity_logs/records"), handler)

	testCases := []struct {
		name      string
		queryPath string
		expected  struct {
			count   int
			firstID string
		}
	}{
		{
			name:      "filter by job_type",
			queryPath: "/api/collections/activity_logs/records?job_type=analysis",
			expected: struct {
				count   int
				firstID string
			}{count: 1, firstID: "log1"},
		},
		{
			name:      "filter by level",
			queryPath: "/api/collections/activity_logs/records?level=error",
			expected: struct {
				count   int
				firstID string
			}{count: 1, firstID: "log2"},
		},
		{
			name:      "filter by mapping_id",
			queryPath: "/api/collections/activity_logs/records?mapping_id=mapping1",
			expected: struct {
				count   int
				firstID string
			}{count: 1, firstID: "log1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.queryPath, nil)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)

			var response ActivityLogsListResponse
			err = json.Unmarshal(rec.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, int64(tc.expected.count), response.TotalItems)
			if tc.expected.count > 0 {
				assert.Equal(t, tc.expected.firstID, response.Items[0].ID)
			}
		})
	}
}
