package jobs

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/activitylogger"
	"github.com/manlikeabro/spotube/internal/auth"
	"github.com/manlikeabro/spotube/internal/migrate"
	"github.com/manlikeabro/spotube/internal/sqliteconn"
)

func setupJobsTestDB(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "jobs_test.db")

	db, err := sqliteconn.OpenWithPragmas(dbPath)
	require.NoError(t, err)

	require.NoError(t, migrate.Up(db))

	cleanup := func() {
		migrate.Down(db)
		db.Close()
	}
	return db, cleanup
}

func TestSchedulerStartStop(t *testing.T) {
	db, cleanup := setupJobsTestDB(t)
	defer cleanup()

	logger := zerolog.Nop()
	activityLogger := activitylogger.New(db)

	// Mock client factory for tests
	tokenRepo := auth.NewSQLiteTokenRepository(db)
	credsRepo := &testSchedulerCredentialsRepo{}
	clientFactory := auth.NewClientFactory(db, credsRepo, tokenRepo)

	scheduler := New(JobDeps{
		DB:             db,
		Logger:         logger,
		ActivityLogger: activityLogger,
	}, clientFactory)

	// Initially not running
	assert.False(t, scheduler.IsRunning())

	// Start scheduler
	err := scheduler.Start()
	require.NoError(t, err)
	assert.True(t, scheduler.IsRunning())

	// Starting again should be no-op
	err = scheduler.Start()
	require.NoError(t, err)
	assert.True(t, scheduler.IsRunning())

	// Stop scheduler
	err = scheduler.Stop()
	require.NoError(t, err)
	assert.False(t, scheduler.IsRunning())

	// Stopping again should be no-op
	err = scheduler.Stop()
	require.NoError(t, err)
	assert.False(t, scheduler.IsRunning())
}

func TestSchedulerLogsStartupAndShutdown(t *testing.T) {
	db, cleanup := setupJobsTestDB(t)
	defer cleanup()

	logger := zerolog.Nop()
	activityLogger := activitylogger.New(db)

	// Mock client factory for tests
	tokenRepo := auth.NewSQLiteTokenRepository(db)
	credsRepo := &testSchedulerCredentialsRepo{}
	clientFactory := auth.NewClientFactory(db, credsRepo, tokenRepo)

	scheduler := New(JobDeps{
		DB:             db,
		Logger:         logger,
		ActivityLogger: activityLogger,
	}, clientFactory)

	// Start and stop scheduler
	err := scheduler.Start()
	require.NoError(t, err)

	err = scheduler.Stop()
	require.NoError(t, err)

	// Verify activity logs were created
	var logCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM activity_logs WHERE job_type = 'system' AND (message LIKE '%scheduler started%' OR message LIKE '%scheduler stopped%')`).Scan(&logCount)
	require.NoError(t, err)
	assert.Equal(t, 2, logCount) // start + stop
}

func TestSchedulerJobsRun(t *testing.T) {
	db, cleanup := setupJobsTestDB(t)
	defer cleanup()

	logger := zerolog.Nop()
	activityLogger := activitylogger.New(db)

	// Mock client factory for tests
	tokenRepo := auth.NewSQLiteTokenRepository(db)
	credsRepo := &testSchedulerCredentialsRepo{}
	clientFactory := auth.NewClientFactory(db, credsRepo, tokenRepo)

	scheduler := New(JobDeps{
		DB:             db,
		Logger:         logger,
		ActivityLogger: activityLogger,
	}, clientFactory)

	err := scheduler.Start()
	require.NoError(t, err)

	// Wait a bit for jobs to potentially run (they are placeholders for now)
	time.Sleep(200 * time.Millisecond)

	err = scheduler.Stop()
	require.NoError(t, err)

	// This test mainly verifies the scheduler can start/stop without panicking
	// Actual job logic will be tested when analysis/executor jobs are implemented
}

func TestSchedulerGracefulShutdown(t *testing.T) {
	db, cleanup := setupJobsTestDB(t)
	defer cleanup()

	logger := zerolog.Nop()
	activityLogger := activitylogger.New(db)

	// Mock client factory for tests
	tokenRepo := auth.NewSQLiteTokenRepository(db)
	credsRepo := &testSchedulerCredentialsRepo{}
	clientFactory := auth.NewClientFactory(db, credsRepo, tokenRepo)

	scheduler := New(JobDeps{
		DB:             db,
		Logger:         logger,
		ActivityLogger: activityLogger,
	}, clientFactory)

	// Start scheduler
	err := scheduler.Start()
	require.NoError(t, err)

	// Schedule stop in background to test graceful shutdown
	go func() {
		time.Sleep(50 * time.Millisecond)
		scheduler.Stop()
	}()

	// Wait for graceful shutdown
	for i := 0; i < 10 && scheduler.IsRunning(); i++ {
		time.Sleep(20 * time.Millisecond)
	}

	assert.False(t, scheduler.IsRunning())
}

type testSchedulerCredentialsRepo struct{}

func (r *testSchedulerCredentialsRepo) GetSettings() (*auth.SettingsRecord, error) {
	return &auth.SettingsRecord{
		SpotifyClientID:     sql.NullString{String: "test-spotify-id", Valid: true},
		SpotifyClientSecret: sql.NullString{String: "test-spotify-secret", Valid: true},
		GoogleClientID:      sql.NullString{String: "test-google-id", Valid: true},
		GoogleClientSecret:  sql.NullString{String: "test-google-secret", Valid: true},
	}, nil
}
