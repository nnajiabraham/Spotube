package migrations

import (
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/models/schema"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		// RFC-010 BF3: Add track detail fields to sync_items collection
		// Preserve all existing records, add nullable columns using PocketBase helpers

		collection, err := dao.FindCollectionByNameOrId("sync_items")
		if err != nil {
			return err
		}

		// Add source_track_id field (nullable)
		collection.Schema.AddField(&schema.SchemaField{
			Name:     "source_track_id",
			Type:     schema.FieldTypeText,
			Required: false, // Nullable to preserve existing records
		})

		// Add source_track_title field (nullable)
		collection.Schema.AddField(&schema.SchemaField{
			Name:     "source_track_title",
			Type:     schema.FieldTypeText,
			Required: false, // Nullable to preserve existing records
		})

		// Add source_service field (nullable)
		collection.Schema.AddField(&schema.SchemaField{
			Name:     "source_service",
			Type:     schema.FieldTypeSelect,
			Required: false, // Nullable to preserve existing records
			Options: &schema.SelectOptions{
				Values:    []string{"spotify", "youtube"},
				MaxSelect: 1,
			},
		})

		// Add destination_service field (nullable)
		collection.Schema.AddField(&schema.SchemaField{
			Name:     "destination_service",
			Type:     schema.FieldTypeSelect,
			Required: false, // Nullable to preserve existing records
			Options: &schema.SelectOptions{
				Values:    []string{"spotify", "youtube"},
				MaxSelect: 1,
			},
		})

		// Save the collection with new fields
		if err := dao.SaveCollection(collection); err != nil {
			return err
		}

		// Set sensible defaults for existing records using raw SQL
		// (DAO record operations would be too slow for bulk updates)
		_, err = db.NewQuery(`
			UPDATE sync_items 
			SET 
				source_track_id = COALESCE(source_track_id, 'unknown'),
				source_track_title = COALESCE(source_track_title, 'Unknown Track'),
				source_service = COALESCE(source_service, 
					CASE 
						WHEN service = 'youtube' THEN 'spotify'  -- If adding to YouTube, likely from Spotify
						WHEN service = 'spotify' THEN 'youtube' -- If adding to Spotify, likely from YouTube
						ELSE 'spotify' -- Default fallback
					END
				),
				destination_service = COALESCE(destination_service, service)
			WHERE source_track_id IS NULL 
			   OR source_track_title IS NULL 
			   OR source_service IS NULL 
			   OR destination_service IS NULL
		`).Execute()

		return err
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		// Rollback: Remove the added fields using PocketBase helpers
		collection, err := dao.FindCollectionByNameOrId("sync_items")
		if err != nil {
			return err
		}

		// Remove fields by name
		fieldNames := []string{"source_track_id", "source_track_title", "source_service", "destination_service"}
		for _, fieldName := range fieldNames {
			if field := collection.Schema.GetFieldByName(fieldName); field != nil {
				collection.Schema.RemoveField(field.Id)
			}
		}

		return dao.SaveCollection(collection)
	})
}
