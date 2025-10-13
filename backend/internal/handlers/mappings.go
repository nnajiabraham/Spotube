package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"

	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/db/table"
)

// MappingRequest represents the payload for creating/updating mappings.
type MappingRequest struct {
	SpotifyPlaylistID   string `json:"spotify_playlist_id" validate:"required"`
	YoutubePlaylistID   string `json:"youtube_playlist_id" validate:"required"`
	SpotifyPlaylistName string `json:"spotify_playlist_name"`
	YoutubePlaylistName string `json:"youtube_playlist_name"`
	SyncName            *bool  `json:"sync_name"`
	SyncTracks          *bool  `json:"sync_tracks"`
	IntervalMinutes     *int   `json:"interval_minutes"`
}

// MappingResponse represents the API response for a single mapping.
type MappingResponse struct {
	ID                  string `json:"id"`
	SpotifyPlaylistID   string `json:"spotify_playlist_id"`
	YoutubePlaylistID   string `json:"youtube_playlist_id"`
	SpotifyPlaylistName string `json:"spotify_playlist_name"`
	YoutubePlaylistName string `json:"youtube_playlist_name"`
	SyncName            bool   `json:"sync_name"`
	SyncTracks          bool   `json:"sync_tracks"`
	IntervalMinutes     int    `json:"interval_minutes"`
	LastAnalysisAt      *int64 `json:"last_analysis_at"`
	TracksCount         int    `json:"tracks_count"`
	Created             int64  `json:"created"`
	Updated             int64  `json:"updated"`
}

// MappingsListResponse represents paginated mappings response.
type MappingsListResponse struct {
	Items      []MappingResponse `json:"items"`
	Page       int               `json:"page"`
	PerPage    int               `json:"perPage"`
	TotalItems int64             `json:"totalItems"`
	TotalPages int               `json:"totalPages"`
}

// MappingsHandler handles mapping CRUD operations.
type MappingsHandler struct {
	DB *sql.DB
}

func NewMappingsHandler(db *sql.DB) *MappingsHandler {
	return &MappingsHandler{DB: db}
}

func RegisterMappingsRoutes(group *echo.Group, handler *MappingsHandler) {
	group.GET("", handler.List)
	group.POST("", handler.Create)
	group.GET("/:id", handler.Get)
	group.PATCH("/:id", handler.Update)
	group.DELETE("/:id", handler.Delete)
}

func (h *MappingsHandler) List(c echo.Context) error {
	page := parseIntParam(c.QueryParam("page"), 1)
	perPage := parseIntParam(c.QueryParam("per_page"), 20)
	if perPage > 100 {
		perPage = 100
	}

	sort := c.QueryParam("sort")
	if sort == "" {
		sort = "created"
	}

	order := c.QueryParam("order")
	if order == "" {
		order = "desc"
	}

	offset := (page - 1) * perPage

	var orderExpr sqlite.OrderByClause
	switch sort {
	case "created":
		if order == "asc" {
			orderExpr = table.Mappings.Created.ASC()
		} else {
			orderExpr = table.Mappings.Created.DESC()
		}
	case "updated":
		if order == "asc" {
			orderExpr = table.Mappings.Updated.ASC()
		} else {
			orderExpr = table.Mappings.Updated.DESC()
		}
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "invalid sort field")
	}

	var mappings []model.Mappings
	err := table.Mappings.
		SELECT(table.Mappings.AllColumns).
		ORDER_BY(orderExpr).
		LIMIT(int64(perPage)).
		OFFSET(int64(offset)).
		Query(h.DB, &mappings)

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch mappings")
	}

	var totalCount struct {
		Count int64
	}
	err = table.Mappings.
		SELECT(sqlite.COUNT(table.Mappings.ID).AS("count")).
		Query(h.DB, &totalCount)

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to count mappings")
	}

	totalPages := int((totalCount.Count + int64(perPage) - 1) / int64(perPage))

	items := lo.Map(mappings, func(m model.Mappings, _ int) MappingResponse {
		return modelToResponse(m)
	})

	response := MappingsListResponse{
		Items:      items,
		Page:       page,
		PerPage:    perPage,
		TotalItems: totalCount.Count,
		TotalPages: totalPages,
	}

	return c.JSON(http.StatusOK, response)
}

func (h *MappingsHandler) Create(c echo.Context) error {
	var req MappingRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	if err := validateMappingRequest(req); err != nil {
		return err
	}

	now := time.Now().Unix()
	id := uuid.NewString()

	mapping := model.Mappings{
		ID:                  &id,
		SpotifyPlaylistID:   req.SpotifyPlaylistID,
		YoutubePlaylistID:   req.YoutubePlaylistID,
		SpotifyPlaylistName: stringPtr(req.SpotifyPlaylistName),
		YoutubePlaylistName: stringPtr(req.YoutubePlaylistName),
		SyncName:            intFromBool(getBoolOrDefault(req.SyncName, true)),
		SyncTracks:          intFromBool(getBoolOrDefault(req.SyncTracks, true)),
		IntervalMinutes:     getIntOrDefault(req.IntervalMinutes, 60),
		LastAnalysisAt:      nil,
		TracksCount:         0,
		Created:             int32(now),
		Updated:             int32(now),
	}

	_, err := table.Mappings.
		INSERT(table.Mappings.AllColumns).
		MODEL(mapping).
		Exec(h.DB)

	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return echo.NewHTTPError(http.StatusConflict, "mapping already exists")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create mapping")
	}

	return c.JSON(http.StatusCreated, modelToResponse(mapping))
}

func (h *MappingsHandler) Get(c echo.Context) error {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing id")
	}

	var mappings []model.Mappings
	err := table.Mappings.
		SELECT(table.Mappings.AllColumns).
		WHERE(table.Mappings.ID.EQ(sqlite.String(id))).
		Query(h.DB, &mappings)

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch mapping")
	}

	if len(mappings) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "mapping not found")
	}

	return c.JSON(http.StatusOK, modelToResponse(mappings[0]))
}

func (h *MappingsHandler) Update(c echo.Context) error {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing id")
	}

	var req MappingRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	if err := validateMappingUpdateRequest(req); err != nil {
		return err
	}

	now := time.Now().Unix()

	// Build update SQL manually due to Jet SET complexity
	updateSQL := "UPDATE mappings SET updated = ? "
	args := []any{now}

	if strings.TrimSpace(req.SpotifyPlaylistName) != "" {
		updateSQL += ", spotify_playlist_name = ? "
		args = append(args, req.SpotifyPlaylistName)
	}
	if strings.TrimSpace(req.YoutubePlaylistName) != "" {
		updateSQL += ", youtube_playlist_name = ? "
		args = append(args, req.YoutubePlaylistName)
	}
	if req.SyncName != nil {
		updateSQL += ", sync_name = ? "
		args = append(args, intFromBool(*req.SyncName))
	}
	if req.SyncTracks != nil {
		updateSQL += ", sync_tracks = ? "
		args = append(args, intFromBool(*req.SyncTracks))
	}
	if req.IntervalMinutes != nil {
		if *req.IntervalMinutes < 5 || *req.IntervalMinutes > 720 {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "interval must be 5-720 minutes")
		}
		updateSQL += ", interval_minutes = ? "
		args = append(args, *req.IntervalMinutes)
	}

	updateSQL += " WHERE id = ?"
	args = append(args, id)

	result, err := h.DB.Exec(updateSQL, args...)

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update mapping")
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "mapping not found")
	}

	// Return updated mapping
	var updatedMappings []model.Mappings
	err = table.Mappings.
		SELECT(table.Mappings.AllColumns).
		WHERE(table.Mappings.ID.EQ(sqlite.String(id))).
		Query(h.DB, &updatedMappings)

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch updated mapping")
	}

	if len(updatedMappings) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "mapping not found after update")
	}

	return c.JSON(http.StatusOK, modelToResponse(updatedMappings[0]))
}

func (h *MappingsHandler) Delete(c echo.Context) error {
	id := c.Param("id")
	if strings.TrimSpace(id) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing id")
	}

	result, err := table.Mappings.
		DELETE().
		WHERE(table.Mappings.ID.EQ(sqlite.String(id))).
		Exec(h.DB)

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete mapping")
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "mapping not found")
	}

	return c.NoContent(http.StatusNoContent)
}

// Helper functions

func parseIntParam(param string, defaultVal int) int {
	if param == "" {
		return defaultVal
	}
	if val, err := strconv.Atoi(param); err == nil && val > 0 {
		return val
	}
	return defaultVal
}

func validateMappingRequest(req MappingRequest) error {
	if strings.TrimSpace(req.SpotifyPlaylistID) == "" {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "spotify_playlist_id is required")
	}
	if strings.TrimSpace(req.YoutubePlaylistID) == "" {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "youtube_playlist_id is required")
	}
	if req.IntervalMinutes != nil && (*req.IntervalMinutes < 5 || *req.IntervalMinutes > 720) {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "interval must be 5-720 minutes")
	}
	return nil
}

func validateMappingUpdateRequest(req MappingRequest) error {
	// For updates, playlist IDs are not required (they can't be changed)
	if req.IntervalMinutes != nil && (*req.IntervalMinutes < 5 || *req.IntervalMinutes > 720) {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "interval must be 5-720 minutes")
	}
	return nil
}

func getBoolOrDefault(ptr *bool, defaultVal bool) bool {
	if ptr == nil {
		return defaultVal
	}
	return *ptr
}

func getIntOrDefault(ptr *int, defaultVal int) int32 {
	if ptr == nil {
		return int32(defaultVal)
	}
	return int32(*ptr)
}

func intFromBool(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

func boolFromInt(i int32) bool {
	return i != 0
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func stringFromPtr(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func int64FromPtr(ptr *int32) *int64 {
	if ptr == nil {
		return nil
	}
	val := int64(*ptr)
	return &val
}

func modelToResponse(m model.Mappings) MappingResponse {
	return MappingResponse{
		ID:                  lo.FromPtr(m.ID),
		SpotifyPlaylistID:   m.SpotifyPlaylistID,
		YoutubePlaylistID:   m.YoutubePlaylistID,
		SpotifyPlaylistName: stringFromPtr(m.SpotifyPlaylistName),
		YoutubePlaylistName: stringFromPtr(m.YoutubePlaylistName),
		SyncName:            boolFromInt(m.SyncName),
		SyncTracks:          boolFromInt(m.SyncTracks),
		IntervalMinutes:     int(m.IntervalMinutes),
		LastAnalysisAt:      int64FromPtr(m.LastAnalysisAt),
		TracksCount:         int(m.TracksCount),
		Created:             int64(m.Created),
		Updated:             int64(m.Updated),
	}
}
