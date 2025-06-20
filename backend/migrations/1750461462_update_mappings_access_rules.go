package migrations

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("mappings")
		if err != nil {
			return err
		}

		collection.ListRule = stringPtr("")
		collection.ViewRule = stringPtr("")
		collection.CreateRule = stringPtr("")
		collection.UpdateRule = stringPtr("")
		collection.DeleteRule = stringPtr("")

		return dao.SaveCollection(collection)
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("mappings")
		if err != nil {
			return err
		}

		collection.ListRule = stringPtr("@request.auth.id != \"\"")
		collection.ViewRule = stringPtr("@request.auth.id != \"\"")
		collection.CreateRule = stringPtr("@request.auth.id != \"\"")
		collection.UpdateRule = stringPtr("@request.auth.id != \"\"")
		collection.DeleteRule = stringPtr("@request.auth.id != \"\"")

		return dao.SaveCollection(collection)
	})
}
