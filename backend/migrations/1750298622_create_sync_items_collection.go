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
			Name: "sync_items",
			Type: models.CollectionTypeBase,
			Schema: schema.NewSchema(
				&schema.SchemaField{
					Name:     "mapping_id",
					Type:     schema.FieldTypeRelation,
					Required: true,
					Options: &schema.RelationOptions{
						CollectionId:  "mappings",
						CascadeDelete: true,
						MaxSelect:     func() *int { i := 1; return &i }(),
					},
				},
				&schema.SchemaField{
					Name:     "service",
					Type:     schema.FieldTypeSelect,
					Required: true,
					Options: &schema.SelectOptions{
						MaxSelect: 1,
						Values:    []string{"spotify", "youtube"},
					},
				},
				&schema.SchemaField{
					Name:     "action",
					Type:     schema.FieldTypeSelect,
					Required: true,
					Options: &schema.SelectOptions{
						MaxSelect: 1,
						Values:    []string{"add_track", "remove_track", "rename_playlist"},
					},
				},
				&schema.SchemaField{
					Name:     "payload",
					Type:     schema.FieldTypeJson,
					Required: false,
				},
				&schema.SchemaField{
					Name:     "status",
					Type:     schema.FieldTypeSelect,
					Required: true,
					Options: &schema.SelectOptions{
						MaxSelect: 1,
						Values:    []string{"pending", "running", "done", "error", "skipped"},
					},
				},
				&schema.SchemaField{
					Name:     "attempts",
					Type:     schema.FieldTypeNumber,
					Required: true,
					Options: &schema.NumberOptions{
						Min: func() *float64 { f := 0.0; return &f }(),
					},
				},
				&schema.SchemaField{
					Name:     "last_error",
					Type:     schema.FieldTypeText,
					Required: false,
				},
			),
			Indexes: []string{
				"CREATE INDEX idx_sync_items_mapping_id ON sync_items (mapping_id)",
				"CREATE INDEX idx_sync_items_status ON sync_items (status)",
				"CREATE INDEX idx_sync_items_service ON sync_items (service)",
			},
		}

		return dao.SaveCollection(collection)
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("sync_items")
		if err != nil {
			return err
		}

		return dao.DeleteCollection(collection)
	})
}
