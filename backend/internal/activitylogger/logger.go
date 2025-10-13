package activitylogger

import (
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/db/table"
)

// Logger provides a simple interface for recording activity to the database.
type Logger struct {
	db *sql.DB
}

// New creates a new activity logger instance.
func New(db *sql.DB) *Logger {
	return &Logger{db: db}
}

// Record inserts an activity log entry into the database.
func (l *Logger) Record(level, message, mappingID, jobType string) error {
	id := uuid.NewString()
	now := time.Now().Unix()

	activityLog := model.ActivityLogs{
		ID:        &id,
		Level:     level,
		Message:   message,
		MappingID: stringToNullablePtr(mappingID),
		JobType:   jobType,
		Created:   int32(now),
	}

	_, err := table.ActivityLogs.
		INSERT(table.ActivityLogs.AllColumns).
		MODEL(activityLog).
		Exec(l.db)

	return err
}

// RecordInfo is a convenience method for info-level logs.
func (l *Logger) RecordInfo(message, mappingID, jobType string) error {
	return l.Record("info", message, mappingID, jobType)
}

// RecordWarn is a convenience method for warn-level logs.
func (l *Logger) RecordWarn(message, mappingID, jobType string) error {
	return l.Record("warn", message, mappingID, jobType)
}

// RecordError is a convenience method for error-level logs.
func (l *Logger) RecordError(message, mappingID, jobType string) error {
	return l.Record("error", message, mappingID, jobType)
}

func stringToNullablePtr(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
