package migrations

import (
	"encoding/json"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/daos"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/models"
)

func init() {
	m.Register(func(db dbx.Builder) error {
		dao := daos.New(db)

		collection := &models.Collection{}
		if err := json.Unmarshal([]byte(`{
			"id": "syncitemsid",
			"created": "2023-01-01 00:00:00.000Z",
			"updated": "2023-01-01 00:00:00.000Z",
			"name": "sync_items",
			"type": "base",
			"system": false,
			"schema": [
				{
					"system": false,
					"id": "mapping_id",
					"name": "mapping_id",
					"type": "relation",
					"required": true,
					"unique": false,
					"options": {
						"collectionId": "mappings",
						"cascadeDelete": true,
						"minSelect": null,
						"maxSelect": 1,
						"displayFields": []
					}
				},
				{
					"system": false,
					"id": "service",
					"name": "service",
					"type": "select",
					"required": true,
					"unique": false,
					"options": {
						"maxSelect": 1,
						"values": ["spotify", "youtube"]
					}
				},
				{
					"system": false,
					"id": "action",
					"name": "action",
					"type": "select",
					"required": true,
					"unique": false,
					"options": {
						"maxSelect": 1,
						"values": ["add_track", "remove_track", "rename_playlist"]
					}
				},
				{
					"system": false,
					"id": "payload",
					"name": "payload",
					"type": "json",
					"required": false,
					"unique": false,
					"options": {}
				},
				{
					"system": false,
					"id": "status",
					"name": "status",
					"type": "select",
					"required": true,
					"unique": false,
					"options": {
						"maxSelect": 1,
						"values": ["pending", "running", "done", "error", "skipped"]
					}
				},
				{
					"system": false,
					"id": "attempts",
					"name": "attempts",
					"type": "number",
					"required": true,
					"unique": false,
					"options": {
						"min": 0,
						"max": null
					}
				},
				{
					"system": false,
					"id": "last_error",
					"name": "last_error",
					"type": "text",
					"required": false,
					"unique": false,
					"options": {
						"min": null,
						"max": null,
						"pattern": ""
					}
				}
			],
			"indexes": [
				"CREATE INDEX idx_sync_items_mapping_id ON sync_items (mapping_id)",
				"CREATE INDEX idx_sync_items_status ON sync_items (status)",
				"CREATE INDEX idx_sync_items_service ON sync_items (service)"
			],
			"listRule": null,
			"viewRule": null,
			"createRule": null,
			"updateRule": null,
			"deleteRule": null,
			"options": {}
		}`), &collection); err != nil {
			return err
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
