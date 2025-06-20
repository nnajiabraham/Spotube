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
			Name: "blacklist",
			Type: models.CollectionTypeBase,
			Schema: schema.NewSchema(
				&schema.SchemaField{
					Name:     "mapping_id",
					Type:     schema.FieldTypeRelation,
					Required: false, // nullable (nil = global blacklist)
					Options: &schema.RelationOptions{
						CollectionId:  "mappings",
						CascadeDelete: true,
						MinSelect:     nil,
						MaxSelect:     intPtr(1),
					},
				},
				&schema.SchemaField{
					Name:     "service",
					Type:     schema.FieldTypeSelect,
					Required: true,
					Options: &schema.SelectOptions{
						Values: []string{"spotify", "youtube"},
					},
				},
				&schema.SchemaField{
					Name:     "track_id",
					Type:     schema.FieldTypeText,
					Required: true,
				},
				&schema.SchemaField{
					Name:     "reason",
					Type:     schema.FieldTypeText,
					Required: true,
				},
				&schema.SchemaField{
					Name:     "skip_counter",
					Type:     schema.FieldTypeNumber,
					Required: true,
					Options: &schema.NumberOptions{
						Min: float64Ptr(1),
					},
				},
				&schema.SchemaField{
					Name:     "last_skipped_at",
					Type:     schema.FieldTypeDate,
					Required: true,
				},
			),
			// Authenticated users only
			ListRule:   stringPtr("@request.auth.id != \"\""),
			ViewRule:   stringPtr("@request.auth.id != \"\""),
			CreateRule: stringPtr("@request.auth.id != \"\""),
			UpdateRule: stringPtr("@request.auth.id != \"\""),
			DeleteRule: stringPtr("@request.auth.id != \"\""),
		}

		// Create collection first
		if err := dao.SaveCollection(collection); err != nil {
			return err
		}

		// Add unique composite index on (mapping_id, service, track_id)
		if _, err := db.NewQuery(`
			CREATE UNIQUE INDEX idx_blacklist_composite 
			ON blacklist (mapping_id, service, track_id)
		`).Execute(); err != nil {
			return err
		}

		// Add index on service field
		if _, err := db.NewQuery(`
			CREATE INDEX idx_blacklist_service 
			ON blacklist (service)
		`).Execute(); err != nil {
			return err
		}

		// Add index on track_id field
		_, err := db.NewQuery(`
			CREATE INDEX idx_blacklist_track_id 
			ON blacklist (track_id)
		`).Execute()

		return err
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("blacklist")
		if err != nil {
			return err
		}

		return dao.DeleteCollection(collection)
	})
}

// Helper function for intPtr (not defined in other migrations)
func intPtr(i int) *int {
	return &i
}
