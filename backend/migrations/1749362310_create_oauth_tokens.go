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
			Name: "oauth_tokens",
			Type: models.CollectionTypeBase,
			Schema: schema.NewSchema(
				&schema.SchemaField{
					Name:     "provider",
					Type:     schema.FieldTypeSelect,
					Required: true,
					Options: &schema.SelectOptions{
						Values: []string{"spotify", "google"},
					},
				},
				&schema.SchemaField{
					Name:     "access_token",
					Type:     schema.FieldTypeText,
					Required: false,
				},
				&schema.SchemaField{
					Name:     "refresh_token",
					Type:     schema.FieldTypeText,
					Required: false,
				},
				&schema.SchemaField{
					Name:     "expiry",
					Type:     schema.FieldTypeDate,
					Required: false,
				},
				&schema.SchemaField{
					Name:     "scopes",
					Type:     schema.FieldTypeText,
					Required: false,
				},
			),
		}

		// TODO: Add unique index on provider for single-user setup
		// Future: Add user relation field and compound unique index (user, provider)

		return dao.SaveCollection(collection)
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("oauth_tokens")
		if err != nil {
			return err
		}

		return dao.DeleteCollection(collection)
	})
}
