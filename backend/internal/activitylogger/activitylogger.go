package activitylogger

import (
	"context"
	"log"
	"time"

	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
)

// daoProvider is an interface that matches the methods we need from pocketbase.PocketBase
// to allow for easier testing, following the same pattern as the jobs package.
type daoProvider interface {
	Dao() *daos.Dao
}

// Logger provides activity logging functionality
type Logger struct {
	app daoProvider
}

// New creates a new activity logger instance
func New(app daoProvider) *Logger {
	return &Logger{app: app}
}

// Record logs an activity event to both console (via standard log) and the activity_logs collection
func (l *Logger) Record(level, message, syncItemID, jobType string) error {
	// Log to console using standard log package with structured format
	logMessage := "[" + level + "] [" + jobType + "]"
	if syncItemID != "" {
		logMessage += " [sync_item:" + syncItemID + "]"
	}
	logMessage += " " + message

	switch level {
	case "warn", "error":
		log.Printf("ACTIVITY %s", logMessage)
	default:
		log.Printf("ACTIVITY %s", logMessage)
	}

	// Save to activity_logs collection
	return l.saveToDatabase(level, message, syncItemID, jobType)
}

// saveToDatabase persists the activity log to the PocketBase collection
func (l *Logger) saveToDatabase(level, message, syncItemID, jobType string) error {
	collection, err := l.app.Dao().FindCollectionByNameOrId("activity_logs")
	if err != nil {
		log.Printf("Failed to find activity_logs collection: %v", err)
		return err
	}

	record := models.NewRecord(collection)
	record.Set("level", level)
	record.Set("message", message)
	record.Set("job_type", jobType)

	// Only set sync_item_id if provided and not empty
	if syncItemID != "" {
		record.Set("sync_item_id", syncItemID)
	}

	if err := l.app.Dao().SaveRecord(record); err != nil {
		log.Printf("Failed to save activity log record: %v", err)
		return err
	}

	return nil
}

// RecordWithContext logs an activity event with context (for timeout handling)
func (l *Logger) RecordWithContext(ctx context.Context, level, message, syncItemID, jobType string) error {
	// Create a channel to handle the operation
	done := make(chan error, 1)

	go func() {
		done <- l.Record(level, message, syncItemID, jobType)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		log.Printf("ACTIVITY [warn] [%s] [sync_item:%s] Activity logging timed out", jobType, syncItemID)
		return ctx.Err()
	case <-time.After(5 * time.Second):
		log.Printf("ACTIVITY [warn] [%s] [sync_item:%s] Activity logging timed out after 5 seconds", jobType, syncItemID)
		return nil // Don't fail the main operation due to logging timeout
	}
}
