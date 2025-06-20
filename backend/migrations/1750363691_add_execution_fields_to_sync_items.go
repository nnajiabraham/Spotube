package migrations

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/models/schema"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("sync_items")
		if err != nil {
			return err
		}

		// Add next_attempt_at field
		if collection.Schema.GetFieldByName("next_attempt_at") == nil {
			nextAttemptAtField := &schema.SchemaField{
				Name:     "next_attempt_at",
				Type:     schema.FieldTypeDate,
				Required: true,
			}
			collection.Schema.AddField(nextAttemptAtField)
		}

		// Add attempt_backoff_secs field
		if collection.Schema.GetFieldByName("attempt_backoff_secs") == nil {
			attemptBackoffSecsField := &schema.SchemaField{
				Name:     "attempt_backoff_secs",
				Type:     schema.FieldTypeNumber,
				Required: true,
				Options: &schema.NumberOptions{
					Min: func() *float64 { f := 30.0; return &f }(),
					Max: func() *float64 { f := 3600.0; return &f }(),
				},
			}
			collection.Schema.AddField(attemptBackoffSecsField)
		}

		return dao.SaveCollection(collection)
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("sync_items")
		if err != nil {
			return err
		}

		// Remove attempt_backoff_secs field
		if field := collection.Schema.GetFieldByName("attempt_backoff_secs"); field != nil {
			collection.Schema.RemoveField(field.Id)
		}

		// Remove next_attempt_at field
		if field := collection.Schema.GetFieldByName("next_attempt_at"); field != nil {
			collection.Schema.RemoveField(field.Id)
		}

		return dao.SaveCollection(collection)
	})
}
