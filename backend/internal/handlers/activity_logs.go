package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"

	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/db/table"
)

// ActivityLogResponse represents the API response for an activity log entry.
type ActivityLogResponse struct {
	ID          string          `json:"id"`
	Level       string          `json:"level"`
	Message     string          `json:"message"`
	MappingID   *string         `json:"mapping_id"`
	JobType     string          `json:"job_type"`
	Created     int64           `json:"created"`
	SyncItemID  *string         `json:"sync_item_id,omitempty"`
	DetailsJSON json.RawMessage `json:"details_json,omitempty"`
}

// ActivityLogsListResponse represents paginated activity logs response.
type ActivityLogsListResponse struct {
	Items      []ActivityLogResponse `json:"items"`
	Page       int                   `json:"page"`
	PerPage    int                   `json:"perPage"`
	TotalItems int64                 `json:"totalItems"`
	TotalPages int                   `json:"totalPages"`
}

// ActivityLogsHandler handles activity logs read operations.
type ActivityLogsHandler struct {
	DB *sql.DB
}

func NewActivityLogsHandler(db *sql.DB) *ActivityLogsHandler {
	return &ActivityLogsHandler{DB: db}
}

func RegisterActivityLogsRoutes(group *echo.Group, handler *ActivityLogsHandler) {
	group.GET("", handler.List)
}

func (h *ActivityLogsHandler) List(c echo.Context) error {
	page := parseIntParam(c.QueryParam("page"), 1)
	perPage := parseIntParam(c.QueryParam("per_page"), 20)
	if perPage > 100 {
		perPage = 100
	}

	// Filter parameters
	jobType := c.QueryParam("job_type")
	level := c.QueryParam("level")
	mappingID := c.QueryParam("mapping_id")
	syncItemID := strings.TrimSpace(c.QueryParam("sync_item_id"))

	offset := (page - 1) * perPage

	selectQuery := table.ActivityLogs.
		SELECT(table.ActivityLogs.AllColumns).
		ORDER_BY(table.ActivityLogs.Created.DESC()).
		LIMIT(int64(perPage)).
		OFFSET(int64(offset))

	countQuery := table.ActivityLogs.SELECT(sqlite.COUNT(table.ActivityLogs.ID).AS("count"))

	// Apply filters
	if strings.TrimSpace(jobType) != "" {
		selectQuery = selectQuery.WHERE(table.ActivityLogs.JobType.EQ(sqlite.String(jobType)))
		countQuery = countQuery.WHERE(table.ActivityLogs.JobType.EQ(sqlite.String(jobType)))
	}

	if strings.TrimSpace(level) != "" {
		selectQuery = selectQuery.WHERE(table.ActivityLogs.Level.EQ(sqlite.String(level)))
		countQuery = countQuery.WHERE(table.ActivityLogs.Level.EQ(sqlite.String(level)))
	}

	if strings.TrimSpace(mappingID) != "" {
		selectQuery = selectQuery.WHERE(table.ActivityLogs.MappingID.EQ(sqlite.String(mappingID)))
		countQuery = countQuery.WHERE(table.ActivityLogs.MappingID.EQ(sqlite.String(mappingID)))
	}

	if syncItemID != "" {
		selectQuery = selectQuery.WHERE(table.ActivityLogs.SyncItemID.EQ(sqlite.String(syncItemID)))
		countQuery = countQuery.WHERE(table.ActivityLogs.SyncItemID.EQ(sqlite.String(syncItemID)))
	}

	var activityLogs []model.ActivityLogs
	err := selectQuery.Query(h.DB, &activityLogs)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch activity logs")
	}

	var totalCount struct {
		Count int64
	}
	err = countQuery.Query(h.DB, &totalCount)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to count activity logs")
	}

	totalPages := int((totalCount.Count + int64(perPage) - 1) / int64(perPage))

	items := lo.Map(activityLogs, func(a model.ActivityLogs, _ int) ActivityLogResponse {
		return activityLogModelToResponse(a)
	})

	response := ActivityLogsListResponse{
		Items:      items,
		Page:       page,
		PerPage:    perPage,
		TotalItems: totalCount.Count,
		TotalPages: totalPages,
	}

	return c.JSON(http.StatusOK, response)
}

func activityLogModelToResponse(a model.ActivityLogs) ActivityLogResponse {
	resp := ActivityLogResponse{
		ID:         lo.FromPtr(a.ID),
		Level:      a.Level,
		Message:    a.Message,
		MappingID:  a.MappingID,
		JobType:    a.JobType,
		Created:    int64(a.Created),
		SyncItemID: a.SyncItemID,
	}
	if a.DetailsJSON != nil && strings.TrimSpace(*a.DetailsJSON) != "" {
		resp.DetailsJSON = json.RawMessage(*a.DetailsJSON)
	}
	return resp
}
