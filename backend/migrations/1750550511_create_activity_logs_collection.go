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
			Name: "activity_logs",
			Type: models.CollectionTypeBase,
			Schema: schema.NewSchema(
				&schema.SchemaField{
					Name:     "level",
					Type:     schema.FieldTypeSelect,
					Required: true,
					Options: &schema.SelectOptions{
						MaxSelect: 1,
						Values:    []string{"info", "warn", "error"},
					},
				},
				&schema.SchemaField{
					Name:     "message",
					Type:     schema.FieldTypeText,
					Required: true,
					Options: &schema.TextOptions{
						Max: intPtr(1024),
					},
				},
				&schema.SchemaField{
					Name:     "sync_item_id",
					Type:     schema.FieldTypeRelation,
					Required: false, // optional - links to specific sync item
					Options: &schema.RelationOptions{
						CollectionId:  "sync_items",
						CascadeDelete: false, // don't delete logs when sync items are deleted
						MaxSelect:     intPtr(1),
					},
				},
				&schema.SchemaField{
					Name:     "job_type",
					Type:     schema.FieldTypeSelect,
					Required: true,
					Options: &schema.SelectOptions{
						MaxSelect: 1,
						Values:    []string{"analysis", "execution", "system"},
					},
				},
			),
			// Make accessible to all users (auth and unauth) for all operations
			ListRule:   stringPtr(""), // public read access
			ViewRule:   stringPtr(""), // public read access
			CreateRule: stringPtr(""), // public create access
			UpdateRule: stringPtr(""), // public update access
			DeleteRule: stringPtr(""), // public delete access
			// Add indexes for efficient querying
			Indexes: []string{
				"CREATE INDEX idx_activity_logs_level ON activity_logs (level)",
				"CREATE INDEX idx_activity_logs_job_type ON activity_logs (job_type)",
				"CREATE INDEX idx_activity_logs_created ON activity_logs (created DESC)",
			},
		}

		return dao.SaveCollection(collection)
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("activity_logs")
		if err != nil {
			return err
		}

		return dao.DeleteCollection(collection)
	})
}
