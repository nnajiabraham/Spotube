package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-jet/jet/v2/qrm"
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
	TrackTitle    string `json:"track_title,omitempty"`
	TrackArtist   string `json:"track_artist,omitempty"`
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
		resp := blacklistModelToResponse(b)
		resp.TrackTitle, resp.TrackArtist = h.lookupTrackLabel(b)
		return resp
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

	// Take the track out of the queue now instead of waiting for the executor to reach it.
	if _, err := h.DB.Exec(
		`UPDATE sync_items SET status = 'skipped', error_message = 'track blacklisted', updated = ?
		 WHERE mapping_id = ? AND service = ? AND track_id = ? AND operation = 'add' AND status IN ('pending', 'error')`,
		now, req.MappingID, req.Service, req.TrackID,
	); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to skip queued track")
	}

	return c.JSON(http.StatusCreated, blacklistModelToResponse(blacklistEntry))
}

func (h *BlacklistHandler) Delete(c echo.Context) error {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing id")
	}

	var entry model.Blacklist
	err := table.Blacklist.
		SELECT(table.Blacklist.AllColumns).
		WHERE(table.Blacklist.ID.EQ(sqlite.String(id))).
		Query(h.DB, &entry)
	if err != nil {
		if errors.Is(err, qrm.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "blacklist entry not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load blacklist entry")
	}

	if _, err := table.Blacklist.
		DELETE().
		WHERE(table.Blacklist.ID.EQ(sqlite.String(id))).
		Exec(h.DB); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete blacklist entry")
	}

	// The skipped queue row would stop analysis from queuing the track again, so put it back.
	if _, err := h.DB.Exec(
		`UPDATE sync_items SET status = 'pending', error_message = NULL, attempt_count = 0, updated = ?
		 WHERE mapping_id = ? AND service = ? AND track_id = ? AND operation = 'add' AND status = 'skipped'`,
		time.Now().Unix(), entry.MappingID, entry.Service, entry.TrackID,
	); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to requeue track")
	}

	return c.NoContent(http.StatusNoContent)
}

// lookupTrackLabel finds the title/artist of a blacklisted track from its queue row.
func (h *BlacklistHandler) lookupTrackLabel(b model.Blacklist) (string, string) {
	var title, artist sql.NullString
	_ = h.DB.QueryRow(
		`SELECT track_title, track_artist FROM sync_items
		 WHERE mapping_id = ? AND service = ? AND track_id = ? AND operation = 'add' LIMIT 1`,
		b.MappingID, b.Service, b.TrackID,
	).Scan(&title, &artist)
	return title.String, artist.String
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
