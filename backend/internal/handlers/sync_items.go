package handlers

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"

	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/db/table"
)

type SyncItemResponse struct {
	ID            string `json:"id"`
	MappingID     string `json:"mapping_id"`
	Operation     string `json:"operation"`
	Service       string `json:"service"`
	TrackID       string `json:"track_id"`
	TrackTitle    string `json:"track_title"`
	TrackArtist   string `json:"track_artist"`
	Status        string `json:"status"`
	ErrorMessage  string `json:"error_message"`
	AttemptCount  int    `json:"attempt_count"`
	LastAttemptAt *int64 `json:"last_attempt_at"`
	Created       int64  `json:"created"`
	Updated       int64  `json:"updated"`
}

type SyncItemsHandler struct {
	DB *sql.DB
}

func NewSyncItemsHandler(db *sql.DB) *SyncItemsHandler {
	return &SyncItemsHandler{DB: db}
}

func RegisterSyncItemsRoutes(group *echo.Group, handler *SyncItemsHandler) {
	group.GET("/:id", handler.Get)
}

func (h *SyncItemsHandler) Get(c echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing id")
	}

	var items []model.SyncItems
	err := table.SyncItems.
		SELECT(table.SyncItems.AllColumns).
		WHERE(table.SyncItems.ID.EQ(sqlite.String(id))).
		LIMIT(1).
		Query(h.DB, &items)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch sync item")
	}
	if len(items) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "sync item not found")
	}

	return c.JSON(http.StatusOK, syncItemToResponse(items[0]))
}

func syncItemToResponse(item model.SyncItems) SyncItemResponse {
	return SyncItemResponse{
		ID:            lo.FromPtr(item.ID),
		MappingID:     item.MappingID,
		Operation:     item.Operation,
		Service:       item.Service,
		TrackID:       stringFromPtr(item.TrackID),
		TrackTitle:    stringFromPtr(item.TrackTitle),
		TrackArtist:   stringFromPtr(item.TrackArtist),
		Status:        item.Status,
		ErrorMessage:  stringFromPtr(item.ErrorMessage),
		AttemptCount:  int(item.AttemptCount),
		LastAttemptAt: int64FromPtr(item.LastAttemptAt),
		Created:       int64(item.Created),
		Updated:       int64(item.Updated),
	}
}
