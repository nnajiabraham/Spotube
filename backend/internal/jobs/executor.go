package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/manlikeabro/spotube/internal/auth"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/models"
	"github.com/pocketbase/pocketbase/tools/cron"
	"github.com/zmb3/spotify/v2"
	"golang.org/x/sync/semaphore"
	"google.golang.org/api/youtube/v3"
)

const (
	BATCH_SIZE             = 50    // Maximum items to process per batch
	MAX_CONCURRENCY        = 5     // Maximum concurrent workers
	SPOTIFY_RATE_LIMIT     = 10    // Spotify requests per second (conservative)
	YOUTUBE_DAILY_QUOTA    = 10000 // YouTube quota units per day
	YOUTUBE_ADD_TRACK_COST = 50    // Cost in quota units for adding a track
)

// YouTubeQuotaTracker tracks daily YouTube API quota usage
type YouTubeQuotaTracker struct {
	mu        sync.Mutex
	used      int
	resetDate string // Date in YYYY-MM-DD format
}

// Global quota tracker instance
var youtubeQuota = &YouTubeQuotaTracker{}

// checkAndConsumeQuota checks if quota is available and consumes it if possible
func (q *YouTubeQuotaTracker) checkAndConsumeQuota(cost int) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Check if we need to reset quota (new day)
	today := time.Now().UTC().Format("2006-01-02")
	if q.resetDate != today {
		q.used = 0
		q.resetDate = today
		log.Printf("YouTube quota reset for new day: %s", today)
	}

	// Check if we have enough quota
	if q.used+cost > YOUTUBE_DAILY_QUOTA {
		log.Printf("YouTube quota exhausted: used=%d, cost=%d, limit=%d", q.used, cost, YOUTUBE_DAILY_QUOTA)
		return false
	}

	// Consume quota
	q.used += cost
	log.Printf("YouTube quota consumed: used=%d/%d (cost=%d)", q.used, YOUTUBE_DAILY_QUOTA, cost)
	return true
}

// getCurrentUsage returns current quota usage for monitoring
func (q *YouTubeQuotaTracker) getCurrentUsage() (used int, limit int, resetDate string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Reset if new day
	today := time.Now().UTC().Format("2006-01-02")
	if q.resetDate != today {
		q.used = 0
		q.resetDate = today
	}

	return q.used, YOUTUBE_DAILY_QUOTA, q.resetDate
}

// RegisterExecutor registers the sync executor job scheduler
func RegisterExecutor(app *pocketbase.PocketBase) {
	// Create a new cron instance
	c := cron.New()

	// Register a cron job that runs every minute (since 5-second intervals aren't supported in standard cron)
	c.MustAdd("sync_executor", "* * * * *", func() {
		ctx := context.Background()
		if err := ProcessQueue(app, ctx); err != nil {
			log.Printf("Executor job failed: %v", err)
		}
	})

	// Start the cron scheduler
	c.Start()
}

// ProcessQueue processes pending sync items from the queue
func ProcessQueue(app daoProvider, ctx context.Context) error {
	log.Println("Starting sync executor job...")

	// Query pending items ready for processing
	now := time.Now()
	filter := fmt.Sprintf("status = 'pending' && next_attempt_at <= '%s'",
		now.Format("2006-01-02 15:04:05.000Z"))

	items, err := app.Dao().FindRecordsByFilter(
		"sync_items",
		filter,
		"created", // order by created ASC (FIFO)
		BATCH_SIZE,
		0,
	)
	if err != nil {
		return fmt.Errorf("failed to query pending sync items: %w", err)
	}

	if len(items) == 0 {
		log.Println("No pending sync items found")
		return nil
	}

	log.Printf("Found %d pending sync items to process", len(items))

	// Create worker pool with semaphore for concurrency control
	workerPool := semaphore.NewWeighted(MAX_CONCURRENCY)
	processed := 0

	for _, item := range items {
		// Acquire semaphore slot
		if err := workerPool.Acquire(ctx, 1); err != nil {
			log.Printf("Failed to acquire worker slot: %v", err)
			continue
		}

		// Process item in goroutine
		go func(syncItem *models.Record) {
			defer workerPool.Release(1)

			if err := processSyncItem(app, syncItem); err != nil {
				log.Printf("Failed to process sync item %s: %v", syncItem.Id, err)
			}
		}(item)

		processed++
	}

	// Wait for all workers to complete
	if err := workerPool.Acquire(ctx, MAX_CONCURRENCY); err != nil {
		log.Printf("Failed to wait for workers: %v", err)
	}
	workerPool.Release(MAX_CONCURRENCY)

	log.Printf("Executor job completed. Processed %d items", processed)
	return nil
}

// processSyncItem processes a single sync item
func processSyncItem(app daoProvider, item *models.Record) error {
	// Mark item as running
	item.Set("status", "running")
	if err := app.Dao().SaveRecord(item); err != nil {
		return fmt.Errorf("failed to mark item as running: %w", err)
	}

	service := item.GetString("service")
	action := item.GetString("action")
	payloadStr := item.GetString("payload")

	log.Printf("Processing sync item %s: service=%s, action=%s", item.Id, service, action)

	// RFC-010 BF3: For add_track actions, perform track search first
	if action == "add_track" {
		// Check if track search has already been performed (payload contains destination_track_id)
		var existingPayload map[string]string
		if payloadStr != "" && payloadStr != "{}" {
			if err := json.Unmarshal([]byte(payloadStr), &existingPayload); err == nil {
				if _, hasTrackID := existingPayload["destination_track_id"]; hasTrackID {
					// Search already performed, proceed with execution
					log.Printf("Track search already completed for item %s", item.Id)
				}
			}
		}

		// If no destination track ID found, perform search
		if existingPayload == nil || existingPayload["destination_track_id"] == "" {
			destinationTrackID, err := performTrackSearch(app, item)
			if err != nil {
				// Track search failed - blacklist the item
				log.Printf("Track search failed for item %s: %v", item.Id, err)

				if err := createOrUpdateBlacklistEntryForSearchFailure(app, item, err); err != nil {
					log.Printf("Failed to create blacklist entry for search failure on item %s: %v", item.Id, err)
				}

				// Mark as skipped since we're blacklisting it
				item.Set("status", "skipped")
				item.Set("last_error", fmt.Sprintf("search_failed: %s", truncateError(err.Error(), 450)))
				return app.Dao().SaveRecord(item)
			}

			// Search successful - update payload with destination track ID
			updatedPayload := map[string]string{
				"destination_track_id": destinationTrackID,
			}
			payloadJSON, err := json.Marshal(updatedPayload)
			if err != nil {
				return fmt.Errorf("failed to marshal updated payload: %w", err)
			}

			payloadStr = string(payloadJSON)
			item.Set("payload", payloadStr)

			if err := app.Dao().SaveRecord(item); err != nil {
				return fmt.Errorf("failed to update item with search result: %w", err)
			}

			log.Printf("Track search successful for item %s: found destination track ID %s", item.Id, destinationTrackID)
		}
	}

	// Execute the appropriate handler
	err := executeAction(app, item, service, action, payloadStr)

	// Update item based on result
	attempts := item.GetInt("attempts") + 1
	item.Set("attempts", attempts)

	if err != nil {
		// Handle different types of errors
		if isRateLimitError(err) {
			// Rate limit - back off and retry
			log.Printf("Rate limit hit for item %s: %v", item.Id, err)
			return handleRetry(app, item, "rate_limit", err)
		} else if isFatalError(err) {
			// Fatal error - mark as error and don't retry
			log.Printf("Fatal error for item %s: %v", item.Id, err)

			// Create or update blacklist entry for unrecoverable errors
			if err := createOrUpdateBlacklistEntry(app, item, err); err != nil {
				log.Printf("Failed to create blacklist entry for item %s: %v", item.Id, err)
				// Continue with marking as error even if blacklist creation fails
			}

			item.Set("status", "skipped") // Mark as skipped instead of error since it's blacklisted
			item.Set("last_error", truncateError(err.Error(), 512))
		} else {
			// Temporary error - retry with backoff
			log.Printf("Temporary error for item %s: %v", item.Id, err)
			return handleRetry(app, item, "temporary", err)
		}
	} else {
		// Success
		log.Printf("Successfully processed sync item %s", item.Id)
		item.Set("status", "done")
		item.Set("last_error", "")
	}

	return app.Dao().SaveRecord(item)
}

// performTrackSearch searches for a track on the destination service using the source track title
// RFC-010 BF3: Implement track search before adding tracks
func performTrackSearch(app daoProvider, item *models.Record) (string, error) {
	sourceTrackTitle := item.GetString("source_track_title")
	destinationService := item.GetString("destination_service")

	if sourceTrackTitle == "" {
		return "", fmt.Errorf("source_track_title is empty")
	}
	if destinationService == "" {
		return "", fmt.Errorf("destination_service is empty")
	}

	log.Printf("Searching for track '%s' on %s", sourceTrackTitle, destinationService)

	switch destinationService {
	case "spotify":
		return searchTrackOnSpotify(app, sourceTrackTitle)
	case "youtube":
		return searchTrackOnYouTube(app, sourceTrackTitle)
	default:
		return "", fmt.Errorf("unsupported destination service: %s", destinationService)
	}
}

// searchTrackOnSpotify searches for a track on Spotify by title
func searchTrackOnSpotify(app daoProvider, trackTitle string) (string, error) {
	ctx := context.Background()

	client, err := auth.GetSpotifyClientForJob(ctx, app)
	if err != nil {
		return "", fmt.Errorf("failed to get Spotify client: %w", err)
	}

	// Search for tracks with the given title
	results, err := client.Search(ctx, trackTitle, spotify.SearchTypeTrack)
	if err != nil {
		return "", fmt.Errorf("failed to search Spotify: %w", err)
	}

	if results.Tracks == nil || len(results.Tracks.Tracks) == 0 {
		return "", fmt.Errorf("no tracks found on Spotify for '%s'", trackTitle)
	}

	// Return the first match
	firstTrack := results.Tracks.Tracks[0]
	trackID := string(firstTrack.ID)

	log.Printf("Found Spotify track: '%s' (ID: %s) for search '%s'", firstTrack.Name, trackID, trackTitle)
	return trackID, nil
}

// searchTrackOnYouTube searches for a track on YouTube by title
func searchTrackOnYouTube(app daoProvider, trackTitle string) (string, error) {
	ctx := context.Background()

	svc, err := auth.GetYouTubeServiceForJob(ctx, app)
	if err != nil {
		return "", fmt.Errorf("failed to get YouTube service: %w", err)
	}

	// Search for videos with the given title
	call := svc.Search.List([]string{"id,snippet"}).Q(trackTitle).Type("video").MaxResults(1)
	response, err := call.Do()
	if err != nil {
		return "", fmt.Errorf("failed to search YouTube: %w", err)
	}

	if len(response.Items) == 0 {
		return "", fmt.Errorf("no videos found on YouTube for '%s'", trackTitle)
	}

	// Return the first match
	firstVideo := response.Items[0]
	videoID := firstVideo.Id.VideoId

	log.Printf("Found YouTube video: '%s' (ID: %s) for search '%s'", firstVideo.Snippet.Title, videoID, trackTitle)
	return videoID, nil
}

// createOrUpdateBlacklistEntryForSearchFailure creates blacklist entry specifically for search failures
func createOrUpdateBlacklistEntryForSearchFailure(app daoProvider, item *models.Record, searchErr error) error {
	// Get track info from the sync item detail fields
	sourceTrackID := item.GetString("source_track_id")
	destinationService := item.GetString("destination_service")

	if sourceTrackID == "" {
		return fmt.Errorf("source_track_id is empty")
	}

	// Get mapping ID from sync item
	var mappingID string
	rawMappingId := item.Get("mapping_id")
	if mappingIds, ok := rawMappingId.([]string); ok && len(mappingIds) > 0 {
		mappingID = mappingIds[0] // Get first element from array
	} else {
		mappingID = item.GetString("mapping_id") // Fallback to string
	}

	// Check if blacklist entry already exists
	filter := fmt.Sprintf("mapping_id = '%s' && service = '%s' && track_id = '%s'",
		mappingID, destinationService, sourceTrackID)

	existingRecords, err := app.Dao().FindRecordsByFilter(
		"blacklist",
		filter,
		"",
		1,
		0,
	)
	if err != nil {
		return fmt.Errorf("failed to query existing blacklist entries: %w", err)
	}

	collection, err := app.Dao().FindCollectionByNameOrId("blacklist")
	if err != nil {
		return fmt.Errorf("failed to find blacklist collection: %w", err)
	}

	now := time.Now()

	if len(existingRecords) > 0 {
		// Update existing blacklist entry
		record := existingRecords[0]
		skipCounter := record.GetInt("skip_counter") + 1
		record.Set("skip_counter", skipCounter)
		record.Set("last_skipped_at", now.Format("2006-01-02 15:04:05.000Z"))
		record.Set("reason", "search_failed")

		if err := app.Dao().SaveRecord(record); err != nil {
			return fmt.Errorf("failed to update blacklist entry: %w", err)
		}

		log.Printf("Updated blacklist entry for mapping %s, service %s, track %s (reason: search_failed, skip_counter: %d)",
			mappingID, destinationService, sourceTrackID, skipCounter)
	} else {
		// Create new blacklist entry
		record := models.NewRecord(collection)
		record.Set("mapping_id", mappingID)
		record.Set("service", destinationService)
		record.Set("track_id", sourceTrackID)
		record.Set("reason", "search_failed")
		record.Set("skip_counter", 1)
		record.Set("last_skipped_at", now.Format("2006-01-02 15:04:05.000Z"))

		if err := app.Dao().SaveRecord(record); err != nil {
			return fmt.Errorf("failed to create blacklist entry: %w", err)
		}

		log.Printf("Created blacklist entry for mapping %s, service %s, track %s (reason: search_failed)",
			mappingID, destinationService, sourceTrackID)
	}

	return nil
}

// executeAction executes the specific action based on service and action type
func executeAction(app daoProvider, item *models.Record, service, action, payloadStr string) error {
	switch service + ":" + action {
	case "spotify:add_track":
		return executeSpotifyAddTrack(app, item, payloadStr)
	case "youtube:add_track":
		return executeYouTubeAddTrack(app, item, payloadStr)
	case "spotify:rename_playlist":
		return executeSpotifyRenamePlaylist(app, item, payloadStr)
	case "youtube:rename_playlist":
		return executeYouTubeRenamePlaylist(app, item, payloadStr)
	default:
		return fmt.Errorf("unsupported action: %s:%s", service, action)
	}
}

// handleRetry implements exponential backoff retry logic
func handleRetry(app daoProvider, item *models.Record, reason string, err error) error {
	attempts := item.GetInt("attempts")

	// Calculate new backoff using exponential backoff formula: min(2^attempts * 30, 3600)
	newBackoff := int(math.Min(math.Pow(2, float64(attempts))*30, 3600))

	nextAttempt := time.Now().UTC().Add(time.Duration(newBackoff) * time.Second)

	item.Set("status", "pending")
	item.Set("attempt_backoff_secs", newBackoff)
	item.Set("next_attempt_at", nextAttempt.Format("2006-01-02 15:04:05.000Z"))
	item.Set("last_error", fmt.Sprintf("%s: %s", reason, truncateError(err.Error(), 500)))

	log.Printf("Retrying item %s in %d seconds (attempt %d)", item.Id, newBackoff, attempts)

	return app.Dao().SaveRecord(item)
}

// Error classification functions
func isRateLimitError(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "too many requests")
}

func isFatalError(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "404") ||
		strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "could not be found") ||
		strings.Contains(errStr, "forbidden") ||
		strings.Contains(errStr, "unauthorized") ||
		strings.Contains(errStr, "invalid")
}

// Utility function to truncate error messages
func truncateError(errMsg string, maxLen int) string {
	if len(errMsg) <= maxLen {
		return errMsg
	}
	return errMsg[:maxLen-3] + "..."
}

// createOrUpdateBlacklistEntry creates or updates a blacklist entry for failed sync items
func createOrUpdateBlacklistEntry(app daoProvider, item *models.Record, execErr error) error {
	// Only create blacklist entries for add_track actions
	action := item.GetString("action")
	if action != "add_track" {
		return nil // Don't blacklist rename operations
	}

	// RFC-010 BF3: Use track detail fields instead of parsing payload
	sourceTrackID := item.GetString("source_track_id")
	destinationService := item.GetString("destination_service")

	if sourceTrackID == "" {
		// Fallback to old method for backward compatibility
		payloadStr := item.GetString("payload")
		var payload map[string]string
		if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
			return fmt.Errorf("failed to parse payload and source_track_id is empty: %w", err)
		}

		// Try both old and new payload formats
		if trackID, ok := payload["track_id"]; ok && trackID != "" {
			sourceTrackID = trackID
		} else if trackID, ok := payload["destination_track_id"]; ok && trackID != "" {
			sourceTrackID = trackID
		} else {
			return fmt.Errorf("no track ID found in payload or source_track_id field")
		}
	}

	if destinationService == "" {
		// Fallback to service field for backward compatibility
		destinationService = item.GetString("service")
	}

	// Get mapping ID from sync item
	var mappingID string
	rawMappingId := item.Get("mapping_id")
	if mappingIds, ok := rawMappingId.([]string); ok && len(mappingIds) > 0 {
		mappingID = mappingIds[0] // Get first element from array
	} else {
		mappingID = item.GetString("mapping_id") // Fallback to string
	}

	// Determine reason based on error type
	reason := categorizeError(execErr)

	// Check if blacklist entry already exists
	filter := fmt.Sprintf("mapping_id = '%s' && service = '%s' && track_id = '%s'",
		mappingID, destinationService, sourceTrackID)

	existingRecords, err := app.Dao().FindRecordsByFilter(
		"blacklist",
		filter,
		"",
		1,
		0,
	)
	if err != nil {
		return fmt.Errorf("failed to query existing blacklist entries: %w", err)
	}

	collection, err := app.Dao().FindCollectionByNameOrId("blacklist")
	if err != nil {
		return fmt.Errorf("failed to find blacklist collection: %w", err)
	}

	now := time.Now()

	if len(existingRecords) > 0 {
		// Update existing blacklist entry
		record := existingRecords[0]
		skipCounter := record.GetInt("skip_counter") + 1
		record.Set("skip_counter", skipCounter)
		record.Set("last_skipped_at", now.Format("2006-01-02 15:04:05.000Z"))
		record.Set("reason", reason) // Update reason in case it changed

		if err := app.Dao().SaveRecord(record); err != nil {
			return fmt.Errorf("failed to update blacklist entry: %w", err)
		}

		log.Printf("Updated blacklist entry for mapping %s, service %s, track %s (skip_counter: %d)",
			mappingID, destinationService, sourceTrackID, skipCounter)
	} else {
		// Create new blacklist entry
		record := models.NewRecord(collection)
		record.Set("mapping_id", mappingID)
		record.Set("service", destinationService)
		record.Set("track_id", sourceTrackID)
		record.Set("reason", reason)
		record.Set("skip_counter", 1)
		record.Set("last_skipped_at", now.Format("2006-01-02 15:04:05.000Z"))

		if err := app.Dao().SaveRecord(record); err != nil {
			return fmt.Errorf("failed to create blacklist entry: %w", err)
		}

		log.Printf("Created blacklist entry for mapping %s, service %s, track %s (reason: %s)",
			mappingID, destinationService, sourceTrackID, reason)
	}

	return nil
}

// categorizeError determines the blacklist reason based on the error
func categorizeError(err error) string {
	errStr := strings.ToLower(err.Error())

	if strings.Contains(errStr, "404") || strings.Contains(errStr, "not found") {
		return "not_found"
	}
	if strings.Contains(errStr, "403") || strings.Contains(errStr, "forbidden") {
		return "forbidden"
	}
	if strings.Contains(errStr, "401") || strings.Contains(errStr, "unauthorized") {
		return "unauthorized"
	}
	if strings.Contains(errStr, "invalid") {
		return "invalid"
	}

	// Default reason for other fatal errors
	return "error"
}

// Placeholder action implementations - these will be implemented in the next steps
func executeSpotifyAddTrack(app daoProvider, item *models.Record, payloadStr string) error {
	// Parse payload to get destination track ID (populated by search)
	var payload map[string]string
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	trackID, ok := payload["destination_track_id"]
	if !ok || trackID == "" {
		return fmt.Errorf("destination_track_id not found in payload - track search may have failed")
	}

	// Get playlist ID from mapping - handle PocketBase relation field properly
	var mappingID string
	rawMappingId := item.Get("mapping_id")
	if mappingIds, ok := rawMappingId.([]string); ok && len(mappingIds) > 0 {
		mappingID = mappingIds[0] // Get first element from array
	} else {
		mappingID = item.GetString("mapping_id") // Fallback to string
	}

	mapping, err := app.Dao().FindRecordById("mappings", mappingID)
	if err != nil {
		return fmt.Errorf("failed to find mapping: %w", err)
	}

	playlistID := mapping.GetString("spotify_playlist_id")
	if playlistID == "" {
		return fmt.Errorf("spotify_playlist_id not found in mapping")
	}

	// Get authenticated Spotify client
	ctx := context.Background()
	client, err := auth.GetSpotifyClientForJob(ctx, app)
	if err != nil {
		return fmt.Errorf("failed to get Spotify client: %w", err)
	}

	// Add track to playlist using the searched destination track ID
	_, err = client.AddTracksToPlaylist(ctx, spotify.ID(playlistID), spotify.ID(trackID))
	if err != nil {
		return fmt.Errorf("failed to add track to Spotify playlist: %w", err)
	}

	sourceTrackTitle := item.GetString("source_track_title")
	log.Printf("Successfully added track '%s' (ID: %s) to Spotify playlist %s", sourceTrackTitle, trackID, playlistID)
	return nil
}

func executeYouTubeAddTrack(app daoProvider, item *models.Record, payloadStr string) error {
	// Check YouTube quota before executing
	if !youtubeQuota.checkAndConsumeQuota(YOUTUBE_ADD_TRACK_COST) {
		// Quota exhausted - mark as skipped
		item.Set("status", "skipped")
		item.Set("last_error", "quota")
		if err := app.Dao().SaveRecord(item); err != nil {
			return fmt.Errorf("failed to mark item as skipped: %w", err)
		}
		return nil // Return nil because skipping is not an error
	}

	// Parse payload to get destination track ID (populated by search)
	var payload map[string]string
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	trackID, ok := payload["destination_track_id"]
	if !ok || trackID == "" {
		return fmt.Errorf("destination_track_id not found in payload - track search may have failed")
	}

	// Get playlist ID from mapping - handle PocketBase relation field properly
	var mappingID string
	rawMappingId := item.Get("mapping_id")
	if mappingIds, ok := rawMappingId.([]string); ok && len(mappingIds) > 0 {
		mappingID = mappingIds[0] // Get first element from array
	} else {
		mappingID = item.GetString("mapping_id") // Fallback to string
	}

	mapping, err := app.Dao().FindRecordById("mappings", mappingID)
	if err != nil {
		return fmt.Errorf("failed to find mapping: %w", err)
	}

	playlistID := mapping.GetString("youtube_playlist_id")
	if playlistID == "" {
		return fmt.Errorf("youtube_playlist_id not found in mapping")
	}

	// Get authenticated YouTube service
	ctx := context.Background()
	svc, err := auth.GetYouTubeServiceForJob(ctx, app)
	if err != nil {
		return fmt.Errorf("failed to get YouTube service: %w", err)
	}

	// Create playlist item using the searched destination track ID (video ID)
	playlistItem := &youtube.PlaylistItem{
		Snippet: &youtube.PlaylistItemSnippet{
			PlaylistId: playlistID,
			ResourceId: &youtube.ResourceId{
				Kind:    "youtube#video",
				VideoId: trackID,
			},
		},
	}

	// Add track to playlist
	_, err = svc.PlaylistItems.Insert([]string{"snippet"}, playlistItem).Do()
	if err != nil {
		return fmt.Errorf("failed to add track to YouTube playlist: %w", err)
	}

	sourceTrackTitle := item.GetString("source_track_title")
	log.Printf("Successfully added track '%s' (ID: %s) to YouTube playlist %s", sourceTrackTitle, trackID, playlistID)
	return nil
}

func executeSpotifyRenamePlaylist(app daoProvider, item *models.Record, payloadStr string) error {
	// Parse payload to get new name
	var payload map[string]string
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	newName, ok := payload["new_name"]
	if !ok || newName == "" {
		return fmt.Errorf("new_name not found in payload")
	}

	// Get playlist ID from mapping - handle PocketBase relation field properly
	var mappingID string
	rawMappingId := item.Get("mapping_id")
	if mappingIds, ok := rawMappingId.([]string); ok && len(mappingIds) > 0 {
		mappingID = mappingIds[0] // Get first element from array
	} else {
		mappingID = item.GetString("mapping_id") // Fallback to string
	}

	mapping, err := app.Dao().FindRecordById("mappings", mappingID)
	if err != nil {
		return fmt.Errorf("failed to find mapping: %w", err)
	}

	playlistID := mapping.GetString("spotify_playlist_id")
	if playlistID == "" {
		return fmt.Errorf("spotify_playlist_id not found in mapping")
	}

	// Get authenticated Spotify client
	ctx := context.Background()
	client, err := auth.GetSpotifyClientForJob(ctx, app)
	if err != nil {
		return fmt.Errorf("failed to get Spotify client: %w", err)
	}

	// Rename playlist
	err = client.ChangePlaylistName(ctx, spotify.ID(playlistID), newName)
	if err != nil {
		return fmt.Errorf("failed to rename Spotify playlist: %w", err)
	}

	log.Printf("Successfully renamed Spotify playlist %s to '%s'", playlistID, newName)
	return nil
}

func executeYouTubeRenamePlaylist(app daoProvider, item *models.Record, payloadStr string) error {
	// Playlist rename has minimal quota cost (assume 1 unit)
	if !youtubeQuota.checkAndConsumeQuota(1) {
		// Quota exhausted - mark as skipped
		item.Set("status", "skipped")
		item.Set("last_error", "quota")
		if err := app.Dao().SaveRecord(item); err != nil {
			return fmt.Errorf("failed to mark item as skipped: %w", err)
		}
		return nil // Return nil because skipping is not an error
	}

	// Parse payload to get new name
	var payload map[string]string
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	newName, ok := payload["new_name"]
	if !ok || newName == "" {
		return fmt.Errorf("new_name not found in payload")
	}

	// Get playlist ID from mapping - handle PocketBase relation field properly
	var mappingID string
	rawMappingId := item.Get("mapping_id")
	if mappingIds, ok := rawMappingId.([]string); ok && len(mappingIds) > 0 {
		mappingID = mappingIds[0] // Get first element from array
	} else {
		mappingID = item.GetString("mapping_id") // Fallback to string
	}

	mapping, err := app.Dao().FindRecordById("mappings", mappingID)
	if err != nil {
		return fmt.Errorf("failed to find mapping: %w", err)
	}

	playlistID := mapping.GetString("youtube_playlist_id")
	if playlistID == "" {
		return fmt.Errorf("youtube_playlist_id not found in mapping")
	}

	// Get authenticated YouTube service
	ctx := context.Background()
	svc, err := auth.GetYouTubeServiceForJob(ctx, app)
	if err != nil {
		return fmt.Errorf("failed to get YouTube service: %w", err)
	}

	// Update playlist with new name
	playlist := &youtube.Playlist{
		Id: playlistID,
		Snippet: &youtube.PlaylistSnippet{
			Title: newName,
		},
	}

	_, err = svc.Playlists.Update([]string{"snippet"}, playlist).Do()
	if err != nil {
		return fmt.Errorf("failed to rename YouTube playlist: %w", err)
	}

	log.Printf("Successfully renamed YouTube playlist %s to '%s'", playlistID, newName)
	return nil
}
