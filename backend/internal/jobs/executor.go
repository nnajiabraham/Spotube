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

	// Register a cron job that runs every 5 seconds
	c.MustAdd("sync_executor", "*/5 * * * * *", func() {
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
			item.Set("status", "error")
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

// Placeholder action implementations - these will be implemented in the next steps
func executeSpotifyAddTrack(app daoProvider, item *models.Record, payloadStr string) error {
	// Parse payload to get track ID
	var payload map[string]string
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	trackID, ok := payload["track_id"]
	if !ok || trackID == "" {
		return fmt.Errorf("track_id not found in payload")
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
	client, err := getSpotifyClientForJob(app)
	if err != nil {
		return fmt.Errorf("failed to get Spotify client: %w", err)
	}

	// Add track to playlist
	ctx := context.Background()
	_, err = client.AddTracksToPlaylist(ctx, spotify.ID(playlistID), spotify.ID(trackID))
	if err != nil {
		return fmt.Errorf("failed to add track to Spotify playlist: %w", err)
	}

	log.Printf("Successfully added track %s to Spotify playlist %s", trackID, playlistID)
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

	// Parse payload to get track ID
	var payload map[string]string
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	trackID, ok := payload["track_id"]
	if !ok || trackID == "" {
		return fmt.Errorf("track_id not found in payload")
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
	svc, err := getYouTubeServiceForJob(ctx, app)
	if err != nil {
		return fmt.Errorf("failed to get YouTube service: %w", err)
	}

	// Create playlist item
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

	log.Printf("Successfully added track %s to YouTube playlist %s", trackID, playlistID)
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
	client, err := getSpotifyClientForJob(app)
	if err != nil {
		return fmt.Errorf("failed to get Spotify client: %w", err)
	}

	// Rename playlist
	ctx := context.Background()
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
	svc, err := getYouTubeServiceForJob(ctx, app)
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
