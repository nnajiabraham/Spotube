package migrations

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/models/schema"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		collection := &models.Collection{
			Name:   "settings",
			Type:   models.CollectionTypeBase,
			System: true, // singleton
			Schema: schema.NewSchema(
				&schema.SchemaField{
					Name:     "spotify_client_id",
					Type:     schema.FieldTypeText,
					Required: false,
				},
				&schema.SchemaField{
					Name:     "spotify_client_secret",
					Type:     schema.FieldTypeText,
					Required: false,
				},
				&schema.SchemaField{
					Name:     "google_client_id",
					Type:     schema.FieldTypeText,
					Required: false,
				},
				&schema.SchemaField{
					Name:     "google_client_secret",
					Type:     schema.FieldTypeText,
					Required: false,
				},
			),
		}

		return dao.SaveCollection(collection)
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("settings")
		if err != nil {
			return err
		}

		return dao.DeleteCollection(collection)
	})
}
