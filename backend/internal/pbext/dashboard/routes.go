package dashboard

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/daos"

	"github.com/manlikeabro/spotube/internal/jobs"
)

// StatsResponse represents the response for /api/dashboard/stats
type StatsResponse struct {
	Mappings     MappingsStats     `json:"mappings"`
	Queue        QueueStats        `json:"queue"`
	RecentRuns   []RecentRunStats  `json:"recent_runs"`
	YouTubeQuota YouTubeQuotaStats `json:"youtube_quota"`
}

// MappingsStats represents mapping statistics
type MappingsStats struct {
	Total int `json:"total"`
}

// QueueStats represents sync queue statistics
type QueueStats struct {
	Pending int `json:"pending"`
	Running int `json:"running"`
	Errors  int `json:"errors"`
	Skipped int `json:"skipped"`
	Done    int `json:"done"`
}

// RecentRunStats represents recent job run statistics
type RecentRunStats struct {
	Timestamp string `json:"timestamp"`
	JobType   string `json:"job_type"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// YouTubeQuotaStats represents YouTube quota statistics
type YouTubeQuotaStats struct {
	Used  int `json:"used"`
	Limit int `json:"limit"`
}

// Register registers the dashboard routes with the PocketBase app
func Register(app *pocketbase.PocketBase) {
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.GET("/api/dashboard/stats", statsHandler(app))
		return nil
	})
}

// statsHandler returns dashboard statistics (unauthenticated)
func statsHandler(app *pocketbase.PocketBase) echo.HandlerFunc {
	return func(c echo.Context) error {
		dao := daos.New(app.Dao().DB())

		// Get mappings statistics
		mappingsStats, err := getMappingsStats(dao)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to get mappings stats: %v", err))
		}

		// Get queue statistics
		queueStats, err := getQueueStats(dao)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to get queue stats: %v", err))
		}

		// Get recent runs from activity logs
		recentRuns, err := getRecentRuns(dao)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to get recent runs: %v", err))
		}

		// Get YouTube quota statistics
		used, limit := jobs.GetYouTubeQuotaUsage()
		youtubeQuota := YouTubeQuotaStats{
			Used:  used,
			Limit: limit,
		}

		response := StatsResponse{
			Mappings:     mappingsStats,
			Queue:        queueStats,
			RecentRuns:   recentRuns,
			YouTubeQuota: youtubeQuota,
		}

		return c.JSON(http.StatusOK, response)
	}
}

// getMappingsStats retrieves mapping statistics
func getMappingsStats(dao *daos.Dao) (MappingsStats, error) {
	// Count total mappings using FindRecordsByFilter
	mappingRecords, err := dao.FindRecordsByFilter("mappings", "id != ''", "", 1000, 0)
	if err != nil {
		return MappingsStats{}, fmt.Errorf("failed to query mappings: %w", err)
	}

	return MappingsStats{
		Total: len(mappingRecords),
	}, nil
}

// getQueueStats retrieves sync queue statistics
func getQueueStats(dao *daos.Dao) (QueueStats, error) {
	stats := QueueStats{}

	// Count sync items by status using FindRecordsByFilter
	statuses := []string{"pending", "running", "error", "skipped", "done"}
	for _, status := range statuses {
		filter := fmt.Sprintf("status = '%s'", status)
		records, err := dao.FindRecordsByFilter("sync_items", filter, "", 10000, 0)
		if err != nil {
			return stats, fmt.Errorf("failed to count sync_items with status %s: %w", status, err)
		}

		count := len(records)
		switch status {
		case "pending":
			stats.Pending = count
		case "running":
			stats.Running = count
		case "error":
			stats.Errors = count
		case "skipped":
			stats.Skipped = count
		case "done":
			stats.Done = count
		}
	}

	return stats, nil
}

// getRecentRuns retrieves recent job runs from activity logs
func getRecentRuns(dao *daos.Dao) ([]RecentRunStats, error) {
	// Query recent activity logs for job completion events
	// Look for analysis and execution job completion messages
	filter := "(job_type = 'analysis' || job_type = 'execution') && (message ~ 'completed' || message ~ 'failed')"

	activityLogs, err := dao.FindRecordsByFilter("activity_logs", filter, "-created", 10, 0)
	if err != nil {
		// If activity_logs table doesn't exist yet, return empty array
		return []RecentRunStats{}, nil
	}

	var recentRuns []RecentRunStats
	for _, record := range activityLogs {
		jobType := record.GetString("job_type")
		level := record.GetString("level")
		message := record.GetString("message")
		created := record.GetDateTime("created")

		// Determine status from level
		status := "success"
		if level == "error" {
			status = "error"
		} else if level == "warn" {
			status = "warning"
		}

		recentRuns = append(recentRuns, RecentRunStats{
			Timestamp: created.Time().Format(time.RFC3339),
			JobType:   jobType,
			Status:    status,
			Message:   message,
		})
	}

	return recentRuns, nil
}
