package activitylogger

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/migrate"
	"github.com/manlikeabro/spotube/internal/sqliteconn"
)

func setupActivityLoggerTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "activity_logger_test.db")

	db, err := sqliteconn.OpenWithPragmas(dbPath)
	require.NoError(t, err)

	require.NoError(t, migrate.Up(db))

	cleanup := func() {
		migrate.Down(db)
		db.Close()
	}
	return db, cleanup
}

func TestActivityLoggerRecord(t *testing.T) {
	db, cleanup := setupActivityLoggerTestDB(t)
	defer cleanup()

	logger := New(db)

	err := logger.Record("info", "Test message", "mapping1", "analysis")
	require.NoError(t, err)

	// Verify the record was inserted
	row := db.QueryRow(`SELECT level, message, mapping_id, job_type FROM activity_logs WHERE message = ?`, "Test message")
	var level, message, mappingID, jobType string
	err = row.Scan(&level, &message, &mappingID, &jobType)
	require.NoError(t, err)

	assert.Equal(t, "info", level)
	assert.Equal(t, "Test message", message)
	assert.Equal(t, "mapping1", mappingID)
	assert.Equal(t, "analysis", jobType)
}

func TestActivityLoggerRecordWithNilMappingID(t *testing.T) {
	db, cleanup := setupActivityLoggerTestDB(t)
	defer cleanup()

	logger := New(db)

	err := logger.Record("warn", "System warning", "", "system")
	require.NoError(t, err)

	// Verify the record was inserted with null mapping_id
	row := db.QueryRow(`SELECT level, message, mapping_id, job_type FROM activity_logs WHERE message = ?`, "System warning")
	var level, message, jobType string
	var mappingID sql.NullString
	err = row.Scan(&level, &message, &mappingID, &jobType)
	require.NoError(t, err)

	assert.Equal(t, "warn", level)
	assert.Equal(t, "System warning", message)
	assert.False(t, mappingID.Valid)
	assert.Equal(t, "system", jobType)
}

func TestActivityLoggerConvenienceMethods(t *testing.T) {
	db, cleanup := setupActivityLoggerTestDB(t)
	defer cleanup()

	logger := New(db)

	// Test convenience methods
	err := logger.RecordInfo("Info message", "mapping1", "analysis")
	require.NoError(t, err)

	err = logger.RecordWarn("Warning message", "mapping2", "executor")
	require.NoError(t, err)

	err = logger.RecordError("Error message", "", "system")
	require.NoError(t, err)

	// Count records
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM activity_logs`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	// Verify levels
	var levels []string
	rows, err := db.Query(`SELECT level FROM activity_logs ORDER BY created`)
	require.NoError(t, err)
	defer rows.Close()

	for rows.Next() {
		var level string
		require.NoError(t, rows.Scan(&level))
		levels = append(levels, level)
	}

	assert.Equal(t, []string{"info", "warn", "error"}, levels)
}

func TestActivityLoggerTimestampsGenerated(t *testing.T) {
	db, cleanup := setupActivityLoggerTestDB(t)
	defer cleanup()

	logger := New(db)

	beforeTime := time.Now().Unix()
	err := logger.Record("info", "Timestamp test", "mapping1", "analysis")
	require.NoError(t, err)
	afterTime := time.Now().Unix()

	// Verify timestamp is reasonable
	row := db.QueryRow(`SELECT created FROM activity_logs WHERE message = ?`, "Timestamp test")
	var created int64
	err = row.Scan(&created)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, created, beforeTime)
	assert.LessOrEqual(t, created, afterTime)
}
