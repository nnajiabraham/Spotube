package handlers

import (
	"database/sql"
	"net/http"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"

	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/db/table"
)

// DashboardStatsResponse represents the dashboard statistics response.
type DashboardStatsResponse struct {
	Mappings     MappingsStats     `json:"mappings"`
	Queue        QueueStats        `json:"queue"`
	RecentRuns   []RecentRun       `json:"recent_runs"`
	YouTubeQuota YouTubeQuotaStats `json:"youtube_quota"`
}

// MappingsStats represents mapping statistics.
type MappingsStats struct {
	Total int64 `json:"total"`
}

// QueueStats represents sync queue statistics.
type QueueStats struct {
	Pending int64 `json:"pending"`
	Running int64 `json:"running"`
	Done    int64 `json:"done"`
	Error   int64 `json:"error"`
	Skipped int64 `json:"skipped"`
}

// RecentRun represents a recent activity log entry.
type RecentRun struct {
	Timestamp int64   `json:"timestamp"`
	JobType   string  `json:"job_type"`
	Level     string  `json:"level"`
	Message   string  `json:"message"`
	MappingID *string `json:"mapping_id,omitempty"`
}

// YouTubeQuotaStats represents YouTube API quota usage.
type YouTubeQuotaStats struct {
	Used  int64 `json:"used"`
	Limit int64 `json:"limit"`
}

// DashboardHandler handles dashboard statistics endpoint.
type DashboardHandler struct {
	DB *sql.DB
}

func NewDashboardHandler(db *sql.DB) *DashboardHandler {
	return &DashboardHandler{DB: db}
}

func RegisterDashboardRoutes(group *echo.Group, handler *DashboardHandler) {
	group.GET("/stats", handler.GetStats)
}

func (h *DashboardHandler) GetStats(c echo.Context) error {
	// Get mappings total
	var mappingsTotal struct {
		Count int64
	}
	err := table.Mappings.
		SELECT(sqlite.COUNT(table.Mappings.ID).AS("count")).
		Query(h.DB, &mappingsTotal)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch mappings count")
	}

	// Get queue stats grouped by status
	queueStats, err := h.getQueueStats()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch queue stats")
	}

	// Get recent activity logs
	recentRuns, err := h.getRecentRuns()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch recent runs")
	}

	// YouTube quota placeholder (will be implemented in Phase 7)
	youtubeQuota := YouTubeQuotaStats{
		Used:  0,
		Limit: 10000,
	}

	response := DashboardStatsResponse{
		Mappings: MappingsStats{
			Total: mappingsTotal.Count,
		},
		Queue:        queueStats,
		RecentRuns:   recentRuns,
		YouTubeQuota: youtubeQuota,
	}

	return c.JSON(http.StatusOK, response)
}

func (h *DashboardHandler) getQueueStats() (QueueStats, error) {
	stats := QueueStats{}

	// Query each status separately to avoid GROUP BY complexity
	statuses := []string{"pending", "running", "done", "error", "skipped"}

	for _, status := range statuses {
		var count struct {
			Count int64
		}

		err := table.SyncItems.
			SELECT(sqlite.COUNT(table.SyncItems.ID).AS("count")).
			WHERE(table.SyncItems.Status.EQ(sqlite.String(status))).
			Query(h.DB, &count)

		if err != nil {
			return QueueStats{}, err
		}

		switch status {
		case "pending":
			stats.Pending = count.Count
		case "running":
			stats.Running = count.Count
		case "done":
			stats.Done = count.Count
		case "error":
			stats.Error = count.Count
		case "skipped":
			stats.Skipped = count.Count
		}
	}

	return stats, nil
}

func (h *DashboardHandler) getRecentRuns() ([]RecentRun, error) {
	var activityLogs []model.ActivityLogs
	err := table.ActivityLogs.
		SELECT(table.ActivityLogs.AllColumns).
		ORDER_BY(table.ActivityLogs.Created.DESC()).
		LIMIT(10).
		Query(h.DB, &activityLogs)

	if err != nil {
		return nil, err
	}

	recentRuns := lo.Map(activityLogs, func(log model.ActivityLogs, _ int) RecentRun {
		return RecentRun{
			Timestamp: int64(log.Created),
			JobType:   log.JobType,
			Level:     log.Level,
			Message:   log.Message,
			MappingID: log.MappingID,
		}
	})

	return recentRuns, nil
}
