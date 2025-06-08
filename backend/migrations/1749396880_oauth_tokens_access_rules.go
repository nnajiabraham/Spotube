package migrations

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		// Find the oauth_tokens collection
		collection, err := dao.FindCollectionByNameOrId("oauth_tokens")
		if err != nil {
			return err
		}

		// Deny all client access by setting null rules (only backend access allowed)
		collection.ListRule = nil
		collection.ViewRule = nil
		collection.CreateRule = nil
		collection.UpdateRule = nil
		collection.DeleteRule = nil

		// Save the updated collection
		return dao.SaveCollection(collection)
	}, func(db dbx.Builder) error {
		// In the down migration, we'll keep the rules as nil
		// since we don't want to accidentally open up access
		return nil
	})
}
