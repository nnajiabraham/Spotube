package migrations

import (
	"encoding/json"

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
			nextAttemptAtField := &schema.SchemaField{}
			if err := json.Unmarshal([]byte(`{
				"system": false,
				"id": "next_attempt_at",
				"name": "next_attempt_at",
				"type": "date",
				"required": true,
				"presentable": false,
				"unique": false,
				"options": {
					"min": "",
					"max": ""
				}
			}`), nextAttemptAtField); err != nil {
				return err
			}
			collection.Schema.AddField(nextAttemptAtField)
		}

		// Add attempt_backoff_secs field
		if collection.Schema.GetFieldByName("attempt_backoff_secs") == nil {
			attemptBackoffSecsField := &schema.SchemaField{}
			if err := json.Unmarshal([]byte(`{
				"system": false,
				"id": "attempt_backoff_secs",
				"name": "attempt_backoff_secs",
				"type": "number",
				"required": true,
				"presentable": false,
				"unique": false,
				"options": {
					"min": 30,
					"max": 3600
				}
			}`), attemptBackoffSecsField); err != nil {
				return err
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
