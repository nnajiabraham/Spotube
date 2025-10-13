package handlers

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"

	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/db/table"
)

// BlacklistRequest represents the payload for creating blacklist entries.
type BlacklistRequest struct {
	MappingID string `json:"mapping_id" validate:"required"`
	Service   string `json:"service" validate:"required"`
	TrackID   string `json:"track_id" validate:"required"`
	Reason    string `json:"reason"`
}

// BlacklistResponse represents the API response for a blacklist entry.
type BlacklistResponse struct {
	ID            string `json:"id"`
	MappingID     string `json:"mapping_id"`
	Service       string `json:"service"`
	TrackID       string `json:"track_id"`
	Reason        string `json:"reason"`
	SkipCounter   int    `json:"skip_counter"`
	LastSkippedAt *int64 `json:"last_skipped_at"`
	Created       int64  `json:"created"`
	Updated       int64  `json:"updated"`
}

// BlacklistListResponse represents paginated blacklist response.
type BlacklistListResponse struct {
	Items      []BlacklistResponse `json:"items"`
	Page       int                 `json:"page"`
	PerPage    int                 `json:"perPage"`
	TotalItems int64               `json:"totalItems"`
	TotalPages int                 `json:"totalPages"`
}

// BlacklistHandler handles blacklist CRUD operations.
type BlacklistHandler struct {
	DB *sql.DB
}

func NewBlacklistHandler(db *sql.DB) *BlacklistHandler {
	return &BlacklistHandler{DB: db}
}

func RegisterBlacklistRoutes(group *echo.Group, handler *BlacklistHandler) {
	group.GET("", handler.List)
	group.POST("", handler.Create)
	group.DELETE("/:id", handler.Delete)
}

func (h *BlacklistHandler) List(c echo.Context) error {
	page := parseIntParam(c.QueryParam("page"), 1)
	perPage := parseIntParam(c.QueryParam("per_page"), 20)
	if perPage > 100 {
		perPage = 100
	}

	mappingID := c.QueryParam("mapping_id")

	offset := (page - 1) * perPage

	selectQuery := table.Blacklist.
		SELECT(table.Blacklist.AllColumns).
		ORDER_BY(table.Blacklist.Created.DESC()).
		LIMIT(int64(perPage)).
		OFFSET(int64(offset))

	countQuery := table.Blacklist.SELECT(sqlite.COUNT(table.Blacklist.ID).AS("count"))

	if strings.TrimSpace(mappingID) != "" {
		selectQuery = selectQuery.WHERE(table.Blacklist.MappingID.EQ(sqlite.String(mappingID)))
		countQuery = countQuery.WHERE(table.Blacklist.MappingID.EQ(sqlite.String(mappingID)))
	}

	var blacklistEntries []model.Blacklist
	err := selectQuery.Query(h.DB, &blacklistEntries)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch blacklist entries")
	}

	var totalCount struct {
		Count int64
	}
	err = countQuery.Query(h.DB, &totalCount)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to count blacklist entries")
	}

	totalPages := int((totalCount.Count + int64(perPage) - 1) / int64(perPage))

	items := lo.Map(blacklistEntries, func(b model.Blacklist, _ int) BlacklistResponse {
		return blacklistModelToResponse(b)
	})

	response := BlacklistListResponse{
		Items:      items,
		Page:       page,
		PerPage:    perPage,
		TotalItems: totalCount.Count,
		TotalPages: totalPages,
	}

	return c.JSON(http.StatusOK, response)
}

func (h *BlacklistHandler) Create(c echo.Context) error {
	var req BlacklistRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	if err := validateBlacklistRequest(req); err != nil {
		return err
	}

	now := time.Now().Unix()
	id := uuid.NewString()

	blacklistEntry := model.Blacklist{
		ID:            &id,
		MappingID:     req.MappingID,
		Service:       req.Service,
		TrackID:       req.TrackID,
		Reason:        stringPtr(req.Reason),
		SkipCounter:   0,
		LastSkippedAt: nil,
		Created:       int32(now),
		Updated:       int32(now),
	}

	_, err := table.Blacklist.
		INSERT(table.Blacklist.AllColumns).
		MODEL(blacklistEntry).
		Exec(h.DB)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return echo.NewHTTPError(http.StatusConflict, "blacklist entry already exists")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create blacklist entry")
	}

	return c.JSON(http.StatusCreated, blacklistModelToResponse(blacklistEntry))
}

func (h *BlacklistHandler) Delete(c echo.Context) error {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing id")
	}

	result, err := table.Blacklist.
		DELETE().
		WHERE(table.Blacklist.ID.EQ(sqlite.String(id))).
		Exec(h.DB)

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete blacklist entry")
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "blacklist entry not found")
	}

	return c.NoContent(http.StatusNoContent)
}

// Helper functions

func validateBlacklistRequest(req BlacklistRequest) error {
	if strings.TrimSpace(req.MappingID) == "" {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "mapping_id is required")
	}
	if strings.TrimSpace(req.Service) == "" {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "service is required")
	}
	if req.Service != "spotify" && req.Service != "youtube" {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "service must be 'spotify' or 'youtube'")
	}
	if strings.TrimSpace(req.TrackID) == "" {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "track_id is required")
	}
	return nil
}

func blacklistModelToResponse(b model.Blacklist) BlacklistResponse {
	return BlacklistResponse{
		ID:            lo.FromPtr(b.ID),
		MappingID:     b.MappingID,
		Service:       b.Service,
		TrackID:       b.TrackID,
		Reason:        stringFromPtr(b.Reason),
		SkipCounter:   int(b.SkipCounter),
		LastSkippedAt: int64FromPtr(b.LastSkippedAt),
		Created:       int64(b.Created),
		Updated:       int64(b.Updated),
	}
}
