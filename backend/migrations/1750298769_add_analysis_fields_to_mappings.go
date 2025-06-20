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

		collection, err := dao.FindCollectionByNameOrId("mappings")
		if err != nil {
			return err
		}

		// Add last_analysis_at field
		if collection.Schema.GetFieldByName("last_analysis_at") == nil {
			lastAnalysisAtField := &schema.SchemaField{
				Name:     "last_analysis_at",
				Type:     schema.FieldTypeDate,
				Required: false,
			}
			collection.Schema.AddField(lastAnalysisAtField)
		}

		// Add next_analysis_at field
		if collection.Schema.GetFieldByName("next_analysis_at") == nil {
			nextAnalysisAtField := &schema.SchemaField{
				Name:     "next_analysis_at",
				Type:     schema.FieldTypeDate,
				Required: false,
			}
			collection.Schema.AddField(nextAnalysisAtField)
		}

		return dao.SaveCollection(collection)
	}, func(db dbx.Builder) error {
		dao := daos.New(db)

		collection, err := dao.FindCollectionByNameOrId("mappings")
		if err != nil {
			return err
		}

		// Remove next_analysis_at field
		if field := collection.Schema.GetFieldByName("next_analysis_at"); field != nil {
			collection.Schema.RemoveField(field.Id)
		}

		// Remove last_analysis_at field
		if field := collection.Schema.GetFieldByName("last_analysis_at"); field != nil {
			collection.Schema.RemoveField(field.Id)
		}

		return dao.SaveCollection(collection)
	})
}
