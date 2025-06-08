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
			Name: "mappings",
			Type: models.CollectionTypeBase,
			Schema: schema.NewSchema(
				&schema.SchemaField{
					Name:     "spotify_playlist_id",
					Type:     schema.FieldTypeText,
					Required: true,
				},
				&schema.SchemaField{
					Name:     "youtube_playlist_id",
					Type:     schema.FieldTypeText,
					Required: true,
				},
				&schema.SchemaField{
					Name:     "spotify_playlist_name",
					Type:     schema.FieldTypeText,
					Required: false,
				},
				&schema.SchemaField{
					Name:     "youtube_playlist_name",
					Type:     schema.FieldTypeText,
					Required: false,
				},
				&schema.SchemaField{
					Name:     "sync_name",
					Type:     schema.FieldTypeBool,
					Required: false,
					Options:  &schema.BoolOptions{
						// Default will be set to true in BeforeCreate hook
					},
				},
				&schema.SchemaField{
					Name:     "sync_tracks",
					Type:     schema.FieldTypeBool,
					Required: false,
					Options:  &schema.BoolOptions{
						// Default will be set to true in BeforeCreate hook
					},
				},
				&schema.SchemaField{
					Name:     "interval_minutes",
					Type:     schema.FieldTypeNumber,
					Required: false,
					Options: &schema.NumberOptions{
						Min: float64Ptr(5),
					},
				},
			),
			ListRule:   stringPtr("@request.auth.id != \"\""),
			ViewRule:   stringPtr("@request.auth.id != \"\""),
			CreateRule: stringPtr("@request.auth.id != \"\""),
			UpdateRule: stringPtr("@request.auth.id != \"\""),
			DeleteRule: stringPtr("@request.auth.id != \"\""),
		}

		// Create collection
		if err := dao.SaveCollection(collection); err != nil {
			return err
		}

		// Add unique index on (spotify_playlist_id, youtube_playlist_id)
		_, err := db.NewQuery(`
			CREATE UNIQUE INDEX idx_mappings_unique_pair 
			ON mappings (spotify_playlist_id, youtube_playlist_id)
		`).Execute()

		return err
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("mappings")
		if err != nil {
			return err
		}

		return dao.DeleteCollection(collection)
	})
}

// Helper functions for pointers
func stringPtr(s string) *string {
	return &s
}

func float64Ptr(f float64) *float64 {
	return &f
}
