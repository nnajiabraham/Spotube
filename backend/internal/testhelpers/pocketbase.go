package testhelpers

import (
	"testing"
	"time"

	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/models/schema"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/types"
	"github.com/stretchr/testify/require"
)

// SetupTestApp creates a test PocketBase app with standard collections
func SetupTestApp(t *testing.T) *tests.TestApp {
	testApp, err := tests.NewTestApp()
	require.NoError(t, err)

	CreateStandardCollections(t, testApp)
	return testApp
}

// CreateStandardCollections creates collections used across multiple tests
func CreateStandardCollections(t *testing.T, testApp *tests.TestApp) {
	CreateOAuthTokensCollection(t, testApp)
	mappingsCollection := CreateMappingsCollection(t, testApp)
	CreateSyncItemsCollection(t, testApp, mappingsCollection)
	CreateSettingsCollection(t, testApp)
}

// CreateOAuthTokensCollection creates the oauth_tokens collection
func CreateOAuthTokensCollection(t *testing.T, testApp *tests.TestApp) *models.Collection {
	oauthCollection := &models.Collection{}
	oauthCollection.Name = "oauth_tokens"
	oauthCollection.Type = models.CollectionTypeBase
	oauthCollection.Schema = schema.NewSchema(
		&schema.SchemaField{Name: "provider", Type: schema.FieldTypeText},
		&schema.SchemaField{Name: "access_token", Type: schema.FieldTypeText},
		&schema.SchemaField{Name: "refresh_token", Type: schema.FieldTypeText},
		&schema.SchemaField{Name: "expiry", Type: schema.FieldTypeDate},
		&schema.SchemaField{Name: "scopes", Type: schema.FieldTypeText},
	)
	err := testApp.Dao().SaveCollection(oauthCollection)
	require.NoError(t, err)
	return oauthCollection
}

// CreateMappingsCollection creates the mappings collection
func CreateMappingsCollection(t *testing.T, testApp *tests.TestApp) *models.Collection {
	mappingsCollection := &models.Collection{}
	mappingsCollection.Name = "mappings"
	mappingsCollection.Type = models.CollectionTypeBase
	mappingsCollection.Schema = schema.NewSchema(
		&schema.SchemaField{Name: "spotify_playlist_id", Type: schema.FieldTypeText, Required: true},
		&schema.SchemaField{Name: "spotify_playlist_name", Type: schema.FieldTypeText},
		&schema.SchemaField{Name: "youtube_playlist_id", Type: schema.FieldTypeText, Required: true},
		&schema.SchemaField{Name: "youtube_playlist_name", Type: schema.FieldTypeText},
		&schema.SchemaField{Name: "sync_name", Type: schema.FieldTypeBool},
		&schema.SchemaField{Name: "sync_tracks", Type: schema.FieldTypeBool},
		&schema.SchemaField{Name: "interval_minutes", Type: schema.FieldTypeNumber},
		&schema.SchemaField{Name: "last_analysis_at", Type: schema.FieldTypeDate},
		&schema.SchemaField{Name: "next_analysis_at", Type: schema.FieldTypeDate},
	)

	// Add unique index on (spotify_playlist_id, youtube_playlist_id)
	mappingsCollection.Indexes = types.JsonArray[string]{
		`CREATE UNIQUE INDEX idx_mappings_playlist_pair ON mappings (spotify_playlist_id, youtube_playlist_id)`,
	}

	err := testApp.Dao().SaveCollection(mappingsCollection)
	require.NoError(t, err)
	return mappingsCollection
}

// CreateSyncItemsCollection creates the sync_items collection
func CreateSyncItemsCollection(t *testing.T, testApp *tests.TestApp, mappingsCollection *models.Collection) *models.Collection {
	syncItemsCollection := &models.Collection{}
	syncItemsCollection.Name = "sync_items"
	syncItemsCollection.Type = models.CollectionTypeBase
	syncItemsCollection.Schema = schema.NewSchema(
		&schema.SchemaField{
			Name:     "mapping_id",
			Type:     schema.FieldTypeRelation,
			Required: true,
			Options: &schema.RelationOptions{
				CollectionId:  mappingsCollection.Id,
				CascadeDelete: true,
				MinSelect:     nil,
				MaxSelect:     nil,
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
			Name:     "action",
			Type:     schema.FieldTypeSelect,
			Required: true,
			Options: &schema.SelectOptions{
				Values: []string{"add_track", "remove_track", "rename_playlist"},
			},
		},
		&schema.SchemaField{Name: "payload", Type: schema.FieldTypeJson},
		&schema.SchemaField{
			Name:     "status",
			Type:     schema.FieldTypeSelect,
			Required: true,
			Options: &schema.SelectOptions{
				Values: []string{"pending", "running", "done", "error", "skipped"},
			},
		},
		&schema.SchemaField{Name: "attempts", Type: schema.FieldTypeNumber, Required: true},
		&schema.SchemaField{Name: "last_error", Type: schema.FieldTypeText},
		&schema.SchemaField{Name: "next_attempt_at", Type: schema.FieldTypeDate, Required: true},
		&schema.SchemaField{
			Name:     "attempt_backoff_secs",
			Type:     schema.FieldTypeNumber,
			Required: true,
			Options: &schema.NumberOptions{
				Min: float64Ptr(30),
				Max: float64Ptr(3600),
			},
		},
	)
	err := testApp.Dao().SaveCollection(syncItemsCollection)
	require.NoError(t, err)
	return syncItemsCollection
}

// Helper function for number field options
func float64Ptr(v float64) *float64 {
	return &v
}

// SetupOAuthTokens creates test OAuth tokens for both services
func SetupOAuthTokens(t *testing.T, testApp *tests.TestApp) {
	// Create Spotify token
	collection, err := testApp.Dao().FindCollectionByNameOrId("oauth_tokens")
	require.NoError(t, err)
	spotifyTokenRecord := models.NewRecord(collection)
	spotifyTokenRecord.Set("provider", "spotify")
	spotifyTokenRecord.Set("access_token", "fake_spotify_token")
	spotifyTokenRecord.Set("refresh_token", "fake_spotify_refresh")
	spotifyTokenRecord.Set("expiry", time.Now().Add(1*time.Hour).Format(time.RFC3339))
	err = testApp.Dao().SaveRecord(spotifyTokenRecord)
	require.NoError(t, err)

	// Create Google token
	googleTokenRecord := models.NewRecord(collection)
	googleTokenRecord.Set("provider", "google")
	googleTokenRecord.Set("access_token", "fake_google_token")
	googleTokenRecord.Set("refresh_token", "fake_google_refresh")
	googleTokenRecord.Set("expiry", time.Now().Add(1*time.Hour).Format(time.RFC3339))
	err = testApp.Dao().SaveRecord(googleTokenRecord)
	require.NoError(t, err)
}

// CreateTestMapping creates a mapping record with given properties
func CreateTestMapping(testApp *tests.TestApp, properties map[string]interface{}) *models.Record {
	collection, err := testApp.Dao().FindCollectionByNameOrId("mappings")
	if err != nil {
		return nil
	}

	mappingRecord := models.NewRecord(collection)

	// Set provided properties
	for key, value := range properties {
		mappingRecord.Set(key, value)
	}

	// Set defaults if not provided
	if mappingRecord.GetString("spotify_playlist_id") == "" {
		mappingRecord.Set("spotify_playlist_id", "default_spotify_playlist")
	}
	if mappingRecord.GetString("youtube_playlist_id") == "" {
		mappingRecord.Set("youtube_playlist_id", "default_youtube_playlist")
	}
	if mappingRecord.GetInt("interval_minutes") == 0 {
		mappingRecord.Set("interval_minutes", 60)
	}

	err = testApp.Dao().SaveRecord(mappingRecord)
	if err != nil {
		return nil
	}

	return mappingRecord
}

// CreateSettingsCollection creates the settings collection for setup wizard
func CreateSettingsCollection(t *testing.T, testApp *tests.TestApp) *models.Collection {
	settingsCollection := &models.Collection{}
	settingsCollection.Name = "settings"
	settingsCollection.Type = models.CollectionTypeBase
	settingsCollection.Schema = schema.NewSchema(
		&schema.SchemaField{Name: "spotify_client_id", Type: schema.FieldTypeText},
		&schema.SchemaField{Name: "spotify_client_secret", Type: schema.FieldTypeText},
		&schema.SchemaField{Name: "google_client_id", Type: schema.FieldTypeText},
		&schema.SchemaField{Name: "google_client_secret", Type: schema.FieldTypeText},
	)
	err := testApp.Dao().SaveCollection(settingsCollection)
	require.NoError(t, err)
	return settingsCollection
}
