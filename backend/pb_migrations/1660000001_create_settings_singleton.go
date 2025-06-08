package migrations

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/models"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		// Find the settings collection
		collection, err := dao.FindCollectionByNameOrId("settings")
		if err != nil {
			return err
		}

		// Check if the singleton record already exists
		_, err = dao.FindRecordById("settings", "settings")
		if err == nil {
			// Record already exists, nothing to do
			return nil
		}

		// Create the singleton settings record
		record := models.NewRecord(collection)
		record.SetId("settings")

		// Save the record with empty values (will be populated by setup wizard)
		return dao.SaveRecord(record)
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		// Try to find and delete the singleton record
		record, err := dao.FindRecordById("settings", "settings")
		if err != nil {
			// Record doesn't exist, nothing to do
			return nil
		}

		return dao.DeleteRecord(record)
	})
}
