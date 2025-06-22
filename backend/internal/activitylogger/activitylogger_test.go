package activitylogger

import (
	"context"
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/models/schema"
	"github.com/pocketbase/pocketbase/tests"
)

func setupActivityLogsCollection(testApp *tests.TestApp) error {
	collection := &models.Collection{
		Name: "activity_logs",
		Type: models.CollectionTypeBase,
		Schema: schema.NewSchema(
			&schema.SchemaField{
				Name:     "level",
				Type:     schema.FieldTypeSelect,
				Required: true,
				Options: &schema.SelectOptions{
					MaxSelect: 1,
					Values:    []string{"info", "warn", "error"},
				},
			},
			&schema.SchemaField{
				Name:     "message",
				Type:     schema.FieldTypeText,
				Required: true,
				Options: &schema.TextOptions{
					Max: intPtr(1024),
				},
			},
			&schema.SchemaField{
				Name:     "sync_item_id",
				Type:     schema.FieldTypeRelation,
				Required: false,
				Options: &schema.RelationOptions{
					CollectionId:  "sync_items",
					CascadeDelete: false,
					MaxSelect:     intPtr(1),
				},
			},
			&schema.SchemaField{
				Name:     "job_type",
				Type:     schema.FieldTypeSelect,
				Required: true,
				Options: &schema.SelectOptions{
					MaxSelect: 1,
					Values:    []string{"analysis", "execution", "system"},
				},
			},
		),
		ListRule:   stringPtr(""),
		ViewRule:   stringPtr(""),
		CreateRule: stringPtr(""),
		UpdateRule: stringPtr(""),
		DeleteRule: stringPtr(""),
	}

	return testApp.Dao().SaveCollection(collection)
}

func TestActivityLogger_Record(t *testing.T) {
	testApp, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("Failed to create test app: %v", err)
	}
	defer testApp.Cleanup()

	// Setup activity_logs collection
	err = setupActivityLogsCollection(testApp)
	if err != nil {
		t.Fatalf("Failed to setup activity_logs collection: %v", err)
	}

	logger := New(testApp)

	tests := []struct {
		name       string
		level      string
		message    string
		syncItemID string
		jobType    string
		wantErr    bool
	}{
		{
			name:       "info level without sync item",
			level:      "info",
			message:    "Test info message",
			syncItemID: "",
			jobType:    "analysis",
			wantErr:    false,
		},
		{
			name:       "warn level with sync item",
			level:      "warn",
			message:    "Test warning message",
			syncItemID: "test-sync-item-123",
			jobType:    "execution",
			wantErr:    false,
		},
		{
			name:       "error level",
			level:      "error",
			message:    "Test error message",
			syncItemID: "",
			jobType:    "system",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Record the activity log
			err := logger.Record(tt.level, tt.message, tt.syncItemID, tt.jobType)
			if (err != nil) != tt.wantErr {
				t.Errorf("Logger.Record() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the record was created in the database
				records, err := testApp.Dao().FindRecordsByFilter("activity_logs", "message = '"+tt.message+"'", "", 1, 0)
				if err != nil {
					t.Fatalf("Failed to query activity_logs: %v", err)
				}

				if len(records) != 1 {
					t.Fatalf("Expected 1 activity log record, got %d", len(records))
				}

				record := records[0]
				if record.GetString("level") != tt.level {
					t.Errorf("Expected level %s, got %s", tt.level, record.GetString("level"))
				}

				if record.GetString("message") != tt.message {
					t.Errorf("Expected message %s, got %s", tt.message, record.GetString("message"))
				}

				if record.GetString("job_type") != tt.jobType {
					t.Errorf("Expected job_type %s, got %s", tt.jobType, record.GetString("job_type"))
				}

				// Check sync_item_id field
				actualSyncItemID := record.GetString("sync_item_id")
				if tt.syncItemID == "" {
					if actualSyncItemID != "" {
						t.Errorf("Expected empty sync_item_id, got %s", actualSyncItemID)
					}
				} else {
					if actualSyncItemID != tt.syncItemID {
						t.Errorf("Expected sync_item_id %s, got %s", tt.syncItemID, actualSyncItemID)
					}
				}
			}
		})
	}
}

func TestActivityLogger_RecordWithContext(t *testing.T) {
	testApp, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("Failed to create test app: %v", err)
	}
	defer testApp.Cleanup()

	// Setup activity_logs collection
	err = setupActivityLogsCollection(testApp)
	if err != nil {
		t.Fatalf("Failed to setup activity_logs collection: %v", err)
	}

	logger := New(testApp)

	t.Run("successful record with context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		err := logger.RecordWithContext(ctx, "info", "Context test message", "", "system")
		if err != nil {
			t.Errorf("RecordWithContext() error = %v", err)
		}

		// Verify the record was created
		records, err := testApp.Dao().FindRecordsByFilter("activity_logs", "message = 'Context test message'", "", 1, 0)
		if err != nil {
			t.Fatalf("Failed to query activity_logs: %v", err)
		}

		if len(records) != 1 {
			t.Fatalf("Expected 1 activity log record, got %d", len(records))
		}
	})

	t.Run("context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		// Wait for context to timeout
		time.Sleep(2 * time.Millisecond)

		err := logger.RecordWithContext(ctx, "info", "Timeout test message", "", "system")
		if err == nil {
			t.Error("Expected timeout error, got nil")
		}
	})
}

func TestActivityLogger_saveToDatabase(t *testing.T) {
	testApp, err := tests.NewTestApp()
	if err != nil {
		t.Fatalf("Failed to create test app: %v", err)
	}
	defer testApp.Cleanup()

	// Setup activity_logs collection
	err = setupActivityLogsCollection(testApp)
	if err != nil {
		t.Fatalf("Failed to setup activity_logs collection: %v", err)
	}

	logger := New(testApp)

	t.Run("save with all fields", func(t *testing.T) {
		err := logger.saveToDatabase("info", "Direct save test", "sync-123", "analysis")
		if err != nil {
			t.Errorf("saveToDatabase() error = %v", err)
		}

		// Verify the record
		records, err := testApp.Dao().FindRecordsByFilter("activity_logs", "message = 'Direct save test'", "", 1, 0)
		if err != nil {
			t.Fatalf("Failed to query activity_logs: %v", err)
		}

		if len(records) != 1 {
			t.Fatalf("Expected 1 activity log record, got %d", len(records))
		}

		record := records[0]
		if record.GetString("level") != "info" {
			t.Errorf("Expected level info, got %s", record.GetString("level"))
		}
		if record.GetString("sync_item_id") != "sync-123" {
			t.Errorf("Expected sync_item_id sync-123, got %s", record.GetString("sync_item_id"))
		}
		if record.GetString("job_type") != "analysis" {
			t.Errorf("Expected job_type analysis, got %s", record.GetString("job_type"))
		}
	})
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func stringPtr(s string) *string {
	return &s
}
