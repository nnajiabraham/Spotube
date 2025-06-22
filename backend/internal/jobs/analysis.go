package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/manlikeabro/spotube/internal/activitylogger"
	"github.com/manlikeabro/spotube/internal/auth"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/daos"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tools/cron"
	"github.com/samber/lo"
	"github.com/zmb3/spotify/v2"
)

// daoProvider is an interface that matches the methods we need from pocketbase.PocketBase
// to allow for easier testing.
type daoProvider interface {
	Dao() *daos.Dao
}

// RegisterAnalysis registers the sync analysis job scheduler using PocketBase cron
func RegisterAnalysis(app *pocketbase.PocketBase) {
	// Create a new cron instance
	c := cron.New()

	// Register a cron job that runs every minute
	c.MustAdd("sync_analysis", "*/1 * * * *", func() {
		ctx := context.Background()
		if err := AnalyseMappings(app, ctx); err != nil {
			log.Printf("Analysis job failed: %v", err)
		}
	})

	// Start the cron scheduler
	c.Start()
}

// Track represents a simplified track structure for comparison
type Track struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Add more fields as needed for matching
}

// TrackList represents a list of tracks from a service
type TrackList struct {
	Tracks  []Track `json:"tracks"`
	Service string  `json:"service"`
}

// AnalyseMappings performs the main analysis logic for all mappings
func AnalyseMappings(app daoProvider, ctx context.Context) error {
	// Create activity logger
	activityLog := activitylogger.New(app)

	log.Println("Starting sync analysis job...")
	activityLog.Record("info", "Starting sync analysis job", "", "analysis")

	// Query all mapping records
	mappings, err := app.Dao().FindRecordsByFilter(
		"mappings",
		"id != ''", // Simple filter to get all records
		"-created", // order by created desc
		500,        // reasonable limit
		0,          // no offset
	)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to query mappings: %v", err)
		activityLog.Record("error", errMsg, "", "analysis")
		return fmt.Errorf("failed to query mappings: %w", err)
	}

	log.Printf("Found %d mappings to analyze", len(mappings))
	activityLog.Record("info", fmt.Sprintf("Found %d mappings to analyze", len(mappings)), "", "analysis")

	now := time.Now()
	processed := 0

	for _, mapping := range mappings {
		if shouldAnalyzeMapping(mapping, now) {
			if err := analyzeMapping(app, mapping, now); err != nil {
				errMsg := fmt.Sprintf("Failed to analyze mapping %s: %v", mapping.Id, err)
				log.Print(errMsg)
				activityLog.Record("error", errMsg, "", "analysis")
				// Continue processing other mappings even if one fails
				continue
			}
			processed++
		}
	}

	completionMsg := fmt.Sprintf("Analysis job completed. Processed %d mappings", processed)
	log.Print(completionMsg)
	activityLog.Record("info", completionMsg, "", "analysis")
	return nil
}

// shouldAnalyzeMapping determines if a mapping should be analyzed based on timing
func shouldAnalyzeMapping(mapping *models.Record, now time.Time) bool {
	// Check if next_analysis_at is set and if it's time to analyze
	nextAnalysisStr := mapping.GetString("next_analysis_at")
	if nextAnalysisStr == "" {
		// No next analysis time set, should analyze (new mapping)
		return true
	}

	// Try to parse the date - PocketBase might use different formats
	var nextAnalysisAt time.Time
	var err error

	// List of formats to try, ordered by most likely
	formats := []string{
		"2006-01-02 15:04:05.000Z", // PocketBase with milliseconds
		"2006-01-02 15:04:05Z",     // PocketBase without milliseconds
		"2006-01-02 15:04:05.999Z", // Handle different millisecond precision
		time.RFC3339,               // Standard format
		time.RFC3339Nano,           // RFC3339 with nanoseconds
	}

	for _, format := range formats {
		nextAnalysisAt, err = time.Parse(format, nextAnalysisStr)
		if err == nil {
			break // Successfully parsed
		}
	}

	if err != nil {
		log.Printf("Failed to parse next_analysis_at '%s' for mapping %s with all formats: %v",
			nextAnalysisStr, mapping.Id, err)
		return true // If we can't parse the date, analyze anyway
	}

	result := now.After(nextAnalysisAt)
	log.Printf("Mapping %s: next_analysis_at=%s, now=%s, should_analyze=%t",
		mapping.Id, nextAnalysisAt.Format(time.RFC3339), now.Format(time.RFC3339), result)

	return result
}

// analyzeMapping performs analysis for a single mapping
func analyzeMapping(app daoProvider, mapping *models.Record, now time.Time) error {
	log.Printf("Analyzing mapping %s", mapping.Id)

	spotifyPlaylistID := mapping.GetString("spotify_playlist_id")
	youtubePlaylistID := mapping.GetString("youtube_playlist_id")
	syncName := mapping.GetBool("sync_name")
	syncTracks := mapping.GetBool("sync_tracks")

	// Fetch track lists from both services
	spotifyTracks, err := fetchSpotifyTracks(app, spotifyPlaylistID)
	if err != nil {
		return fmt.Errorf("failed to fetch Spotify tracks: %w", err)
	}

	youtubeTracks, err := fetchYouTubeTracks(app, youtubePlaylistID)
	if err != nil {
		return fmt.Errorf("failed to fetch YouTube tracks: %w", err)
	}

	// Perform bidirectional diff analysis
	if syncTracks {
		if err := analyzeTracks(app, mapping, spotifyTracks, youtubeTracks); err != nil {
			return fmt.Errorf("failed to analyze tracks: %w", err)
		}
	}

	// Perform playlist name sync if enabled
	if syncName {
		if err := analyzePlaylistNames(app, mapping, spotifyTracks, youtubeTracks); err != nil {
			return fmt.Errorf("failed to analyze playlist names: %w", err)
		}
	}

	// Update analysis timestamps
	if err := updateMappingAnalysisTime(app, mapping, now); err != nil {
		return fmt.Errorf("failed to update mapping timestamps: %w", err)
	}

	return nil
}

// analyzeTracks performs bidirectional track difference analysis
func analyzeTracks(app daoProvider, mapping *models.Record, spotifyTracks, youtubeTracks TrackList) error {
	// Convert to comparable formats
	spotifyIDs := lo.Map(spotifyTracks.Tracks, func(t Track, _ int) string { return t.ID })
	youtubeIDs := lo.Map(youtubeTracks.Tracks, func(t Track, _ int) string { return t.ID })

	// Calculate differences
	toAddOnSpotify := lo.Without(youtubeIDs, spotifyIDs...) // YouTube tracks missing from Spotify
	toAddOnYouTube := lo.Without(spotifyIDs, youtubeIDs...) // Spotify tracks missing from YouTube

	// Filter out blacklisted tracks before enqueuing
	toAddOnSpotify = filterBlacklistedTracks(app, mapping, "spotify", toAddOnSpotify)
	toAddOnYouTube = filterBlacklistedTracks(app, mapping, "youtube", toAddOnYouTube)

	// Create lookup maps for track details
	spotifyTrackMap := make(map[string]Track)
	for _, track := range spotifyTracks.Tracks {
		spotifyTrackMap[track.ID] = track
	}

	youtubeTrackMap := make(map[string]Track)
	for _, track := range youtubeTracks.Tracks {
		youtubeTrackMap[track.ID] = track
	}

	// Enqueue add_track items for Spotify (source: YouTube, destination: Spotify)
	for _, trackID := range toAddOnSpotify {
		sourceTrack, exists := youtubeTrackMap[trackID]
		if !exists {
			log.Printf("Warning: YouTube track %s not found in track map", trackID)
			continue
		}

		if err := enqueueSyncItemWithDetails(app, mapping, "spotify", "add_track",
			trackID, sourceTrack.Title, "youtube", "spotify"); err != nil {
			return err
		}
	}

	// Enqueue add_track items for YouTube (source: Spotify, destination: YouTube)
	for _, trackID := range toAddOnYouTube {
		sourceTrack, exists := spotifyTrackMap[trackID]
		if !exists {
			log.Printf("Warning: Spotify track %s not found in track map", trackID)
			continue
		}

		if err := enqueueSyncItemWithDetails(app, mapping, "youtube", "add_track",
			trackID, sourceTrack.Title, "spotify", "youtube"); err != nil {
			return err
		}
	}

	log.Printf("Mapping %s: queued %d tracks for Spotify, %d tracks for YouTube",
		mapping.Id, len(toAddOnSpotify), len(toAddOnYouTube))

	return nil
}

// filterBlacklistedTracks removes blacklisted tracks from the given track list
func filterBlacklistedTracks(app daoProvider, mapping *models.Record, service string, trackIDs []string) []string {
	if len(trackIDs) == 0 {
		return trackIDs
	}

	// Build filter to check for blacklisted tracks
	// Check both mapping-specific blacklist (mapping_id = current mapping)
	// and global blacklist (mapping_id is empty/null)
	filter := fmt.Sprintf(
		"service = '%s' && (mapping_id = '%s' || mapping_id = '')",
		service, mapping.Id,
	)

	blacklistRecords, err := app.Dao().FindRecordsByFilter(
		"blacklist",
		filter,
		"",   // no specific order needed
		1000, // reasonable limit for blacklist entries
		0,    // no offset
	)
	if err != nil {
		// Log error but don't fail the analysis - continue without filtering
		log.Printf("Failed to query blacklist for mapping %s service %s: %v", mapping.Id, service, err)
		return trackIDs
	}

	if len(blacklistRecords) == 0 {
		// No blacklist entries found, return all tracks
		return trackIDs
	}

	// Extract blacklisted track IDs
	blacklistedTrackIDs := make(map[string]bool)
	for _, record := range blacklistRecords {
		trackID := record.GetString("track_id")
		if trackID != "" {
			blacklistedTrackIDs[trackID] = true
		}
	}

	// Filter out blacklisted tracks
	var filteredTracks []string
	var filteredCount int
	for _, trackID := range trackIDs {
		if !blacklistedTrackIDs[trackID] {
			filteredTracks = append(filteredTracks, trackID)
		} else {
			filteredCount++
		}
	}

	if filteredCount > 0 {
		log.Printf("Mapping %s: filtered %d blacklisted tracks for service %s",
			mapping.Id, filteredCount, service)
	}

	return filteredTracks
}

// analyzePlaylistNames checks for playlist name differences and enqueues rename actions
func analyzePlaylistNames(app daoProvider, mapping *models.Record, spotifyTracks, youtubeTracks TrackList) error {
	// For now, we'll use cached names from the mapping record
	// In a full implementation, we'd fetch current names from the APIs
	spotifyName := mapping.GetString("spotify_playlist_name")
	youtubeName := mapping.GetString("youtube_playlist_name")

	if spotifyName != "" && youtubeName != "" && spotifyName != youtubeName {
		// Choose canonical name (YouTube by default as specified in RFC)
		canonicalName := youtubeName
		if youtubeName == "" {
			canonicalName = spotifyName
		}

		// Enqueue rename for Spotify if it differs
		if spotifyName != canonicalName {
			payload := map[string]string{"new_name": canonicalName}
			if err := enqueueSyncItem(app, mapping, "spotify", "rename_playlist", payload); err != nil {
				return err
			}
		}

		// Enqueue rename for YouTube if it differs
		if youtubeName != canonicalName {
			payload := map[string]string{"new_name": canonicalName}
			if err := enqueueSyncItem(app, mapping, "youtube", "rename_playlist", payload); err != nil {
				return err
			}
		}
	}

	return nil
}

// enqueueSyncItem creates a new sync_items record
// RFC-010 BF2: Check for existing pending or running sync items with same parameters
// This prevents duplicate items from being enqueued
func enqueueSyncItem(app daoProvider, mapping *models.Record, service, action string, payload map[string]string) error {
	// Create a unique payload by adding timestamp to avoid UNIQUE constraint violations
	// while still allowing duplicate detection via application logic
	uniquePayload := make(map[string]string)
	for k, v := range payload {
		uniquePayload[k] = v
	}
	// Add timestamp to make payload unique for database constraint
	uniquePayload["_timestamp"] = fmt.Sprintf("%d", time.Now().UnixNano())

	// Convert payload to JSON for comparison (without timestamp for duplicate detection)
	originalPayloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	originalPayloadStr := string(originalPayloadJSON)

	// Convert unique payload to JSON for storage
	uniquePayloadJSON, err := json.Marshal(uniquePayload)
	if err != nil {
		return fmt.Errorf("failed to marshal unique payload: %w", err)
	}
	uniquePayloadStr := string(uniquePayloadJSON)

	// Get all sync_items and check for duplicates manually (more reliable than complex filters)
	allSyncItems, err := app.Dao().FindRecordsByFilter("sync_items", "id != ''", "", 100, 0)
	if err != nil {
		return fmt.Errorf("failed to check existing sync items: %w", err)
	}

	// Check for duplicates among pending and running items using original payload (without timestamp)
	for _, item := range allSyncItems {
		itemStatus := item.GetString("status")

		// Only check pending and running items (allow duplicates of completed items)
		if itemStatus != "pending" && itemStatus != "running" {
			continue
		}

		// Handle mapping_id as either string or relation array
		var itemMappingId string
		rawMappingId := item.Get("mapping_id")
		if mappingIds, ok := rawMappingId.([]string); ok && len(mappingIds) > 0 {
			itemMappingId = mappingIds[0] // Get first element from relation array
		} else {
			itemMappingId = item.GetString("mapping_id") // Fallback to string
		}

		// Parse the stored payload to remove timestamp for comparison
		itemPayloadStr := item.GetString("payload")
		var itemPayload map[string]string
		if json.Unmarshal([]byte(itemPayloadStr), &itemPayload) == nil {
			// Remove timestamp for comparison
			delete(itemPayload, "_timestamp")
			itemPayloadForComparison, _ := json.Marshal(itemPayload)

			// Check for exact match on all parameters (using original payload without timestamp)
			if itemMappingId == mapping.Id &&
				item.GetString("service") == service &&
				item.GetString("action") == action &&
				string(itemPayloadForComparison) == originalPayloadStr {
				log.Printf("Skipping duplicate sync item: mapping_id=%s, service=%s, action=%s, payload=%s (existing item: %s)",
					mapping.Id, service, action, originalPayloadStr, item.Id)
				return nil
			}
		}
	}

	// No duplicate found, proceed with creating new sync item
	collection, err := app.Dao().FindCollectionByNameOrId("sync_items")
	if err != nil {
		return fmt.Errorf("failed to find sync_items collection: %w", err)
	}

	record := models.NewRecord(collection)
	record.Set("mapping_id", mapping.Id)
	record.Set("service", service)
	record.Set("action", action)
	record.Set("status", "pending")
	record.Set("attempts", 0)

	// Set executor fields with defaults
	now := time.Now()
	record.Set("next_attempt_at", now.Format("2006-01-02 15:04:05.000Z"))
	record.Set("attempt_backoff_secs", 30)
	record.Set("payload", uniquePayloadStr) // Use unique payload for storage

	log.Printf("Creating sync item: mapping_id=%s, service=%s, action=%s, payload=%s",
		mapping.Id, service, action, originalPayloadStr)

	if err := app.Dao().SaveRecord(record); err != nil {
		return fmt.Errorf("failed to save sync item: %w", err)
	}

	log.Printf("Successfully created sync item with ID: %s", record.Id)
	return nil
}

// enqueueSyncItemWithDetails creates a new sync_items record with track details for BF3
// RFC-010 BF3: Populate track detail fields for proper search and logging
func enqueueSyncItemWithDetails(app daoProvider, mapping *models.Record, destinationService, action, sourceTrackID, sourceTrackTitle, sourceService, destService string) error {
	// For BF3: Create a unique payload that includes track details for duplicate detection
	// This avoids UNIQUE constraint failures when multiple tracks have empty payloads
	payload := map[string]string{
		"source_track_id": sourceTrackID,
		"action_key":      fmt.Sprintf("%s_%s_%s", sourceService, destService, sourceTrackID),
	}

	// Convert payload to JSON for duplicate checking
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	payloadStr := string(payloadJSON)

	// Check for duplicates using the same logic as before, but now with track details
	allSyncItems, err := app.Dao().FindRecordsByFilter("sync_items", "id != ''", "", 100, 0)
	if err != nil {
		return fmt.Errorf("failed to check existing sync items: %w", err)
	}

	// Check for duplicates among pending and running items
	for _, item := range allSyncItems {
		itemStatus := item.GetString("status")

		// Only check pending and running items (allow duplicates of completed items)
		if itemStatus != "pending" && itemStatus != "running" {
			continue
		}

		// Handle mapping_id as either string or relation array
		var itemMappingId string
		rawMappingId := item.Get("mapping_id")
		if mappingIds, ok := rawMappingId.([]string); ok && len(mappingIds) > 0 {
			itemMappingId = mappingIds[0] // Get first element from relation array
		} else {
			itemMappingId = item.GetString("mapping_id") // Fallback to string
		}

		// Check for duplicate based on track details rather than just payload
		if itemMappingId == mapping.Id &&
			item.GetString("service") == destinationService &&
			item.GetString("action") == action &&
			item.GetString("source_track_id") == sourceTrackID &&
			item.GetString("source_service") == sourceService &&
			item.GetString("destination_service") == destService {
			log.Printf("Skipping duplicate sync item: mapping_id=%s, service=%s, action=%s, source_track_id=%s (existing item: %s)",
				mapping.Id, destinationService, action, sourceTrackID, item.Id)
			return nil
		}
	}

	// No duplicate found, proceed with creating new sync item
	collection, err := app.Dao().FindCollectionByNameOrId("sync_items")
	if err != nil {
		return fmt.Errorf("failed to find sync_items collection: %w", err)
	}

	record := models.NewRecord(collection)
	record.Set("mapping_id", mapping.Id)
	record.Set("service", destinationService)
	record.Set("action", action)
	record.Set("status", "pending")
	record.Set("attempts", 0)

	// RFC-010 BF3: Set the new track detail fields
	record.Set("source_track_id", sourceTrackID)
	record.Set("source_track_title", sourceTrackTitle)
	record.Set("source_service", sourceService)
	record.Set("destination_service", destService)

	// Set executor fields with defaults
	now := time.Now()
	record.Set("next_attempt_at", now.Format("2006-01-02 15:04:05.000Z"))
	record.Set("attempt_backoff_secs", 30)
	record.Set("payload", payloadStr) // Unique payload for constraint, will be updated by executor after search

	log.Printf("Creating sync item with details: mapping_id=%s, service=%s, action=%s, source_track='%s' (%s), source_service=%s, destination_service=%s",
		mapping.Id, destinationService, action, sourceTrackTitle, sourceTrackID, sourceService, destService)

	if err := app.Dao().SaveRecord(record); err != nil {
		return fmt.Errorf("failed to save sync item: %w", err)
	}

	log.Printf("Successfully created sync item with ID: %s", record.Id)
	return nil
}

// updateMappingAnalysisTime updates the analysis timestamps on the mapping
func updateMappingAnalysisTime(app daoProvider, mapping *models.Record, now time.Time) error {
	intervalMinutes := mapping.GetInt("interval_minutes")
	if intervalMinutes == 0 {
		intervalMinutes = 60 // default to 1 hour
	}

	nextAnalysis := now.Add(time.Duration(intervalMinutes) * time.Minute)

	mapping.Set("last_analysis_at", now.Format("2006-01-02 15:04:05.000Z"))
	mapping.Set("next_analysis_at", nextAnalysis.Format("2006-01-02 15:04:05.000Z"))

	if err := app.Dao().SaveRecord(mapping); err != nil {
		return fmt.Errorf("failed to update mapping analysis time: %w", err)
	}

	return nil
}

// Placeholder functions for fetching tracks - these will call existing OAuth helpers
// For now, they return empty lists to allow the job to compile and run

func fetchSpotifyTracks(app daoProvider, playlistID string) (TrackList, error) {
	// Create a dummy context
	ctx := context.Background()

	// Use the unified auth factory
	client, err := auth.GetSpotifyClientForJob(ctx, app)
	if err != nil {
		return TrackList{Service: "spotify"}, fmt.Errorf("failed to get Spotify client: %w", err)
	}

	// Fetch playlist tracks using Spotify API
	tracks, err := client.GetPlaylistTracks(ctx, spotify.ID(playlistID))
	if err != nil {
		return TrackList{Service: "spotify"}, fmt.Errorf("failed to fetch Spotify playlist tracks: %w", err)
	}

	// Convert to our Track format
	var trackList []Track
	for _, item := range tracks.Tracks {
		// Handle the track directly
		track := Track{
			ID:    string(item.Track.ID),
			Title: item.Track.Name,
		}
		trackList = append(trackList, track)
	}

	log.Printf("Fetched %d tracks from Spotify playlist %s", len(trackList), playlistID)
	return TrackList{
		Tracks:  trackList,
		Service: "spotify",
	}, nil
}

func fetchYouTubeTracks(app daoProvider, playlistID string) (TrackList, error) {
	// Create context
	ctx := context.Background()

	// Use the unified auth factory
	svc, err := auth.GetYouTubeServiceForJob(ctx, app)
	if err != nil {
		return TrackList{Service: "youtube"}, fmt.Errorf("failed to get YouTube service: %w", err)
	}

	// Fetch playlist items using YouTube API
	call := svc.PlaylistItems.List([]string{"snippet"}).PlaylistId(playlistID).MaxResults(50)
	resp, err := call.Do()
	if err != nil {
		return TrackList{Service: "youtube"}, fmt.Errorf("failed to fetch YouTube playlist items: %w", err)
	}

	// Convert to our Track format
	var trackList []Track
	for _, item := range resp.Items {
		track := Track{
			ID:    item.Id,
			Title: item.Snippet.Title,
		}
		trackList = append(trackList, track)
	}

	log.Printf("Fetched %d tracks from YouTube playlist %s", len(trackList), playlistID)
	return TrackList{
		Tracks:  trackList,
		Service: "youtube",
	}, nil
}
