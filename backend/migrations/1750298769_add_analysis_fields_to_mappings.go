package migrations

import (
	"encoding/json"

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
			lastAnalysisAtField := &schema.SchemaField{}
			if err := json.Unmarshal([]byte(`{
				"system": false,
				"id": "last_analysis_at",
				"name": "last_analysis_at",
				"type": "date",
				"required": false,
				"presentable": false,
				"unique": false,
				"options": {
					"min": "",
					"max": ""
				}
			}`), lastAnalysisAtField); err != nil {
				return err
			}
			collection.Schema.AddField(lastAnalysisAtField)
		}

		// Add next_analysis_at field
		if collection.Schema.GetFieldByName("next_analysis_at") == nil {
			nextAnalysisAtField := &schema.SchemaField{}
			if err := json.Unmarshal([]byte(`{
				"system": false,
				"id": "next_analysis_at",
				"name": "next_analysis_at",
				"type": "date",
				"required": false,
				"presentable": false,
				"unique": false,
				"options": {
					"min": "",
					"max": ""
				}
			}`), nextAnalysisAtField); err != nil {
				return err
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
