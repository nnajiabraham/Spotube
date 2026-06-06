package activitylogger

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/migrate"
	"github.com/manlikeabro/spotube/internal/sqliteconn"
	"github.com/manlikeabro/spotube/internal/synccontext"
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

	require.NoError(t, logger.RecordInfo("Info message", "mapping1", "analysis"))
	require.NoError(t, logger.RecordWarn("Warning message", "mapping2", "executor"))
	require.NoError(t, logger.RecordError("Error message", "", "system"))

	var count int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM activity_logs`).Scan(&count))
	assert.Equal(t, 3, count)

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
	require.NoError(t, logger.Record("info", "Timestamp test", "mapping1", "analysis"))
	afterTime := time.Now().Unix()

	row := db.QueryRow(`SELECT created FROM activity_logs WHERE message = ?`, "Timestamp test")
	var created int64
	require.NoError(t, row.Scan(&created))

	assert.GreaterOrEqual(t, created, beforeTime)
	assert.LessOrEqual(t, created, afterTime)
}

func TestRecordWithDetails_PersistsJSONAndSyncItemID(t *testing.T) {
	db, cleanup := setupActivityLoggerTestDB(t)
	defer cleanup()
	logger := New(db)

	now := time.Now().Unix()
	_, err := db.Exec(
		`INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, sync_name, sync_tracks, interval_minutes, tracks_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"map-1", "sp", "yt", 1, 1, 60, 0, now, now,
	)
	require.NoError(t, err)
	_, err = db.Exec(
		`INSERT INTO sync_items (id, mapping_id, operation, service, track_id, track_title, status, attempt_count, created, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"item-1", "map-1", "add", "youtube", "sp-1", "Song", "pending", 0, now, now,
	)
	require.NoError(t, err)

	details := synccontext.Envelope{
		Version: synccontext.Version,
		Kind:    "executor_run",
		Data: synccontext.ExecutorRun{
			SyncItemID: "item-1",
			Outcome:    "added",
		},
	}
	require.NoError(t, logger.RecordWithDetails("info", "Executor added track", "map-1", "executor", "item-1", details))

	var message, detailsJSON, syncItemID string
	err = db.QueryRow(
		`SELECT message, COALESCE(details_json, ''), COALESCE(sync_item_id, '') FROM activity_logs ORDER BY created DESC LIMIT 1`,
	).Scan(&message, &detailsJSON, &syncItemID)
	require.NoError(t, err)
	assert.Equal(t, "Executor added track", message)
	assert.Equal(t, "item-1", syncItemID)

	var stored synccontext.Envelope
	require.NoError(t, json.Unmarshal([]byte(detailsJSON), &stored))
	assert.Equal(t, "executor_run", stored.Kind)
}
