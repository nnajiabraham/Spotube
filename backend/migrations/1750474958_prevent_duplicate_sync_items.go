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

		// RFC-010 BF2: Update sync_items collection to prevent duplicates
		// Step 1: Change payload field from JSON to TEXT for better indexing and querying
		collection, err := dao.FindCollectionByNameOrId("sync_items")
		if err != nil {
			return err
		}

		// Update the existing payload field from JSON to TEXT
		payloadField := collection.Schema.GetFieldByName("payload")
		if payloadField != nil {
			payloadField.Type = schema.FieldTypeText
			payloadField.Options = nil // Text fields don't need explicit options for basic usage
		}

		if err := dao.SaveCollection(collection); err != nil {
			return err
		}

		// Step 2: Remove any existing duplicates to allow index creation
		// Delete duplicate sync_items, keeping only the most recent one for each combination
		// This query keeps the item with the highest ID (most recent) for each unique combination
		_, err = db.NewQuery(`
			DELETE FROM sync_items 
			WHERE id NOT IN (
				SELECT MAX(id) 
				FROM sync_items 
				GROUP BY mapping_id, service, action, payload
			)
		`).Execute()
		if err != nil {
			return err
		}

		// Step 3: Create unique composite index on (mapping_id, service, action, payload)
		// This prevents the database from ever allowing duplicate sync items
		_, err = db.NewQuery(`
			CREATE UNIQUE INDEX idx_sync_items_unique_composite 
			ON sync_items (mapping_id, service, action, payload)
		`).Execute()

		return err
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		// Rollback: Remove the unique index and revert payload field to JSON
		_, err := db.NewQuery(`
			DROP INDEX IF EXISTS idx_sync_items_unique_composite
		`).Execute()
		if err != nil {
			return err
		}

		// Revert payload field back to JSON type
		collection, err := dao.FindCollectionByNameOrId("sync_items")
		if err != nil {
			return err
		}

		payloadField := collection.Schema.GetFieldByName("payload")
		if payloadField != nil {
			payloadField.Type = schema.FieldTypeJson
			payloadField.Options = nil // JSON fields don't need explicit options for basic usage
		}

		return dao.SaveCollection(collection)
	})
}
