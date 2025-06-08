package mappings

import (
	"fmt"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/models"
)

// RegisterHooks registers hooks for the mappings collection
func RegisterHooks(app *pocketbase.PocketBase) {
	// BeforeCreate hook to set default values
	app.OnRecordBeforeCreateRequest("mappings").Add(func(e *core.RecordCreateEvent) error {
		// Set defaults if not provided
		if e.Record.Get("sync_name") == nil {
			e.Record.Set("sync_name", true)
		}
		if e.Record.Get("sync_tracks") == nil {
			e.Record.Set("sync_tracks", true)
		}
		if e.Record.Get("interval_minutes") == nil {
			e.Record.Set("interval_minutes", 60)
		}

		// Validate interval_minutes
		intervalMinutes := e.Record.GetFloat("interval_minutes")
		if intervalMinutes < 5 {
			return fmt.Errorf("interval_minutes must be at least 5, got %v", intervalMinutes)
		}

		// TODO: Validate playlist IDs exist (will be done after auth is complete)
		// For now, just ensure they're not empty (handled by Required field)

		return nil
	})

	// BeforeUpdate hook to validate interval_minutes
	app.OnRecordBeforeUpdateRequest("mappings").Add(func(e *core.RecordUpdateEvent) error {
		// Validate interval_minutes
		intervalMinutes := e.Record.GetFloat("interval_minutes")
		if intervalMinutes < 5 {
			return fmt.Errorf("interval_minutes must be at least 5, got %v", intervalMinutes)
		}

		return nil
	})

	// AfterCreate hook to populate cached playlist names
	app.OnRecordAfterCreateRequest("mappings").Add(func(e *core.RecordCreateEvent) error {
		// Run in background to avoid blocking the response
		go fetchAndCachePlaylistNames(app, e.Record)
		return nil
	})

	// AfterUpdate hook to refresh cached playlist names if IDs changed
	app.OnRecordAfterUpdateRequest("mappings").Add(func(e *core.RecordUpdateEvent) error {
		oldSpotifyID := e.Record.OriginalCopy().GetString("spotify_playlist_id")
		oldYouTubeID := e.Record.OriginalCopy().GetString("youtube_playlist_id")
		newSpotifyID := e.Record.GetString("spotify_playlist_id")
		newYouTubeID := e.Record.GetString("youtube_playlist_id")

		// Only fetch if IDs changed
		if oldSpotifyID != newSpotifyID || oldYouTubeID != newYouTubeID {
			go fetchAndCachePlaylistNames(app, e.Record)
		}
		return nil
	})
}

// fetchAndCachePlaylistNames fetches playlist names from Spotify and YouTube APIs
func fetchAndCachePlaylistNames(app *pocketbase.PocketBase, record *models.Record) {
	// This is a placeholder - actual implementation will use the Spotify and YouTube
	// auth packages to make authenticated API calls
	// For now, we'll just log that we would fetch the names

	// TODO: Implementation will be added after testing infrastructure is in place
	// Steps:
	// 1. Use spotifyauth package to get authenticated client
	// 2. Fetch Spotify playlist by ID
	// 3. Use googleauth package to get authenticated client
	// 4. Fetch YouTube playlist by ID
	// 5. Update the record with the fetched names

	// For now, we'll leave the names empty - they'll be populated by the sync job
}
