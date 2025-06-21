package migrations

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("sync_items")
		if err != nil {
			return err
		}

		// Add the unique composite index to prevent duplicate sync items
		collection.Indexes = append(collection.Indexes,
			"CREATE UNIQUE INDEX IF NOT EXISTS idx_sync_items_unique_composite ON sync_items (mapping_id, service, action, payload)",
		)

		return dao.SaveCollection(collection)
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("sync_items")
		if err != nil {
			return err
		}

		// Remove the unique composite index
		newIndexes := make([]string, 0)
		for _, index := range collection.Indexes {
			// Keep all indexes except the one we're removing
			if index != "CREATE UNIQUE INDEX IF NOT EXISTS idx_sync_items_unique_composite ON sync_items (mapping_id, service, action, payload)" {
				newIndexes = append(newIndexes, index)
			}
		}
		collection.Indexes = newIndexes

		return dao.SaveCollection(collection)
	})
}
