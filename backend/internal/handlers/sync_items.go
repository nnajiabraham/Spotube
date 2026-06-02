package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"

	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/db/table"
	"github.com/manlikeabro/spotube/internal/jobs"
)

// SyncItemResponse represents a sync item detail payload.
type SyncItemResponse struct {
	ID                      string `json:"id"`
	MappingID               string `json:"mapping_id"`
	Operation               string `json:"operation"`
	Service                 string `json:"service"`
	TrackID                 string `json:"track_id"`
	TrackTitle              string `json:"track_title"`
	TrackArtist             string `json:"track_artist"`
	Status                  string `json:"status"`
	ErrorMessage            string `json:"error_message"`
	AttemptCount            int    `json:"attempt_count"`
	LastAttemptAt           *int64 `json:"last_attempt_at"`
	Created                 int64  `json:"created"`
	Updated                 int64  `json:"updated"`
	SourceService           string `json:"source_service"`
	DestinationService      string `json:"destination_service"`
	SourcePlaylistName      string `json:"source_playlist_name"`
	DestinationPlaylistName string `json:"destination_playlist_name"`
}

// SyncItemsListResponse represents paginated sync items.
type SyncItemsListResponse struct {
	Items      []SyncItemResponse `json:"items"`
	Page       int                `json:"page"`
	PerPage    int                `json:"perPage"`
	TotalItems int64              `json:"totalItems"`
	TotalPages int                `json:"totalPages"`
}

// SyncItemExecuteResponse is returned from POST execute.
type SyncItemExecuteResponse struct {
	SyncItemResponse
	ExecutionLog string `json:"execution_log,omitempty"`
}

// SyncItemExecutor runs a single sync item.
type SyncItemExecutor interface {
	ExecuteOne(ctx context.Context, itemID string) (model.SyncItems, error)
}

// SyncItemsHandler handles sync item endpoints.
type SyncItemsHandler struct {
	DB       *sql.DB
	Executor SyncItemExecutor
}

func NewSyncItemsHandler(db *sql.DB, executor SyncItemExecutor) *SyncItemsHandler {
	return &SyncItemsHandler{DB: db, Executor: executor}
}

func RegisterSyncItemsRoutes(group *echo.Group, handler *SyncItemsHandler) {
	group.GET("", handler.List)
	group.GET("/:id", handler.Get)
	group.POST("/:id/execute", handler.Execute)
}

func (h *SyncItemsHandler) List(c echo.Context) error {
	page := parseIntParam(c.QueryParam("page"), 1)
	perPage := parseIntParam(c.QueryParam("per_page"), 20)
	if perPage > 100 {
		perPage = 100
	}

	sort := c.QueryParam("sort")
	if sort == "" {
		sort = "created"
	}
	if sort != "created" {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid sort field")
	}

	order := c.QueryParam("order")
	if order == "" {
		order = "desc"
	}

	statusFilter := strings.TrimSpace(c.QueryParam("status"))
	serviceFilter := strings.TrimSpace(c.QueryParam("service"))
	operationFilter := strings.TrimSpace(c.QueryParam("operation"))
	mappingFilter := strings.TrimSpace(c.QueryParam("mapping_id"))

	offset := (page - 1) * perPage

	selectQuery := table.SyncItems.SELECT(table.SyncItems.AllColumns)
	countQuery := table.SyncItems.SELECT(sqlite.COUNT(table.SyncItems.ID).AS("count"))

	var conditions []sqlite.BoolExpression
	if statusFilter != "" {
		conditions = append(conditions, table.SyncItems.Status.EQ(sqlite.String(statusFilter)))
	}
	if serviceFilter != "" {
		conditions = append(conditions, table.SyncItems.Service.EQ(sqlite.String(serviceFilter)))
	}
	if operationFilter != "" {
		conditions = append(conditions, table.SyncItems.Operation.EQ(sqlite.String(operationFilter)))
	}
	if mappingFilter != "" {
		conditions = append(conditions, table.SyncItems.MappingID.EQ(sqlite.String(mappingFilter)))
	}
	if len(conditions) > 0 {
		where := conditions[0]
		for i := 1; i < len(conditions); i++ {
			where = where.AND(conditions[i])
		}
		selectQuery = selectQuery.WHERE(where)
		countQuery = countQuery.WHERE(where)
	}

	if order == "asc" {
		selectQuery = selectQuery.ORDER_BY(table.SyncItems.Created.ASC())
	} else {
		selectQuery = selectQuery.ORDER_BY(table.SyncItems.Created.DESC())
	}

	selectQuery = selectQuery.LIMIT(int64(perPage)).OFFSET(int64(offset))

	var items []model.SyncItems
	if err := selectQuery.Query(h.DB, &items); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch sync items")
	}

	var totalCount struct {
		Count int64
	}
	if err := countQuery.Query(h.DB, &totalCount); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to count sync items")
	}

	mappingNames, err := h.loadMappingNames(lo.Map(items, func(i model.SyncItems, _ int) string {
		return i.MappingID
	}))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load mappings")
	}

	totalPages := int((totalCount.Count + int64(perPage) - 1) / int64(perPage))
	responseItems := lo.Map(items, func(item model.SyncItems, _ int) SyncItemResponse {
		return enrichSyncItemResponse(item, mappingNames[item.MappingID])
	})

	return c.JSON(http.StatusOK, SyncItemsListResponse{
		Items:      responseItems,
		Page:       page,
		PerPage:    perPage,
		TotalItems: totalCount.Count,
		TotalPages: totalPages,
	})
}

func (h *SyncItemsHandler) Get(c echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing id")
	}

	item, mapping, err := h.loadItemWithMapping(id)
	if err != nil {
		if errors.Is(err, errSyncItemNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "sync item not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch sync item")
	}

	return c.JSON(http.StatusOK, enrichSyncItemResponse(item, mapping))
}

func (h *SyncItemsHandler) Execute(c echo.Context) error {
	if h.Executor == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "executor not configured")
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing id")
	}

	item, execErr := h.Executor.ExecuteOne(c.Request().Context(), id)
	mapping, mapErr := h.loadMappingByID(item.MappingID)
	if mapErr != nil && !errors.Is(execErr, jobs.ErrSyncItemNotFound) {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load mapping")
	}

	response := SyncItemExecuteResponse{
		SyncItemResponse: enrichSyncItemResponse(item, mapping),
	}

	switch {
	case errors.Is(execErr, jobs.ErrSyncItemNotFound):
		return echo.NewHTTPError(http.StatusNotFound, "sync item not found")
	case errors.Is(execErr, jobs.ErrSyncItemNotExecutable):
		return echo.NewHTTPError(http.StatusConflict, "sync item is not executable")
	case errors.Is(execErr, jobs.ErrSyncItemBlacklisted):
		return c.JSON(http.StatusConflict, response)
	case execErr != nil:
		response.ExecutionLog = execErr.Error()
		return c.JSON(http.StatusOK, response)
	default:
		return c.JSON(http.StatusOK, response)
	}
}

var errSyncItemNotFound = errors.New("sync item not found")

func (h *SyncItemsHandler) loadItemWithMapping(id string) (model.SyncItems, mappingNames, error) {
	var items []model.SyncItems
	err := table.SyncItems.
		SELECT(table.SyncItems.AllColumns).
		WHERE(table.SyncItems.ID.EQ(sqlite.String(id))).
		LIMIT(1).
		Query(h.DB, &items)
	if err != nil {
		return model.SyncItems{}, mappingNames{}, err
	}
	if len(items) == 0 {
		return model.SyncItems{}, mappingNames{}, errSyncItemNotFound
	}

	names, err := h.loadMappingNames([]string{items[0].MappingID})
	if err != nil {
		return model.SyncItems{}, mappingNames{}, err
	}
	return items[0], names[items[0].MappingID], nil
}

type mappingNames struct {
	SpotifyPlaylistID   string
	YoutubePlaylistID   string
	SpotifyPlaylistName string
	YoutubePlaylistName string
}

func (h *SyncItemsHandler) loadMappingByID(mappingID string) (mappingNames, error) {
	names, err := h.loadMappingNames([]string{mappingID})
	if err != nil {
		return mappingNames{}, err
	}
	return names[mappingID], nil
}

func (h *SyncItemsHandler) loadMappingNames(mappingIDs []string) (map[string]mappingNames, error) {
	uniqueIDs := lo.Uniq(lo.Filter(mappingIDs, func(id string, _ int) bool {
		return strings.TrimSpace(id) != ""
	}))
	result := make(map[string]mappingNames, len(uniqueIDs))
	if len(uniqueIDs) == 0 {
		return result, nil
	}

	ids := make([]sqlite.Expression, len(uniqueIDs))
	for i, id := range uniqueIDs {
		ids[i] = sqlite.String(id)
	}

	var mappings []model.Mappings
	err := table.Mappings.
		SELECT(
			table.Mappings.ID,
			table.Mappings.SpotifyPlaylistID,
			table.Mappings.YoutubePlaylistID,
			table.Mappings.SpotifyPlaylistName,
			table.Mappings.YoutubePlaylistName,
		).
		WHERE(table.Mappings.ID.IN(ids...)).
		Query(h.DB, &mappings)
	if err != nil {
		return nil, err
	}

	for _, m := range mappings {
		result[lo.FromPtr(m.ID)] = mappingNames{
			SpotifyPlaylistID:   m.SpotifyPlaylistID,
			YoutubePlaylistID:   m.YoutubePlaylistID,
			SpotifyPlaylistName: stringFromPtr(m.SpotifyPlaylistName),
			YoutubePlaylistName: stringFromPtr(m.YoutubePlaylistName),
		}
	}
	return result, nil
}

func syncItemToResponse(item model.SyncItems) SyncItemResponse {
	return enrichSyncItemResponse(item, mappingNames{})
}

func enrichSyncItemResponse(item model.SyncItems, names mappingNames) SyncItemResponse {
	sourceService, destinationService := deriveSourceDestination(item.Service)
	sourcePlaylist := playlistDisplayLabel(names.SpotifyPlaylistName, names.SpotifyPlaylistID)
	destinationPlaylist := playlistDisplayLabel(names.YoutubePlaylistName, names.YoutubePlaylistID)
	if destinationService == "spotify" {
		sourcePlaylist = playlistDisplayLabel(names.YoutubePlaylistName, names.YoutubePlaylistID)
		destinationPlaylist = playlistDisplayLabel(names.SpotifyPlaylistName, names.SpotifyPlaylistID)
	}

	return SyncItemResponse{
		ID:                      lo.FromPtr(item.ID),
		MappingID:               item.MappingID,
		Operation:               item.Operation,
		Service:                 item.Service,
		TrackID:                 stringFromPtr(item.TrackID),
		TrackTitle:              stringFromPtr(item.TrackTitle),
		TrackArtist:             stringFromPtr(item.TrackArtist),
		Status:                  item.Status,
		ErrorMessage:            stringFromPtr(item.ErrorMessage),
		AttemptCount:            int(item.AttemptCount),
		LastAttemptAt:           int64FromPtr(item.LastAttemptAt),
		Created:                 int64(item.Created),
		Updated:                 int64(item.Updated),
		SourceService:           sourceService,
		DestinationService:      destinationService,
		SourcePlaylistName:      sourcePlaylist,
		DestinationPlaylistName: destinationPlaylist,
	}
}

func playlistDisplayLabel(name, id string) string {
	if strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return strings.TrimSpace(id)
}

func deriveSourceDestination(destinationService string) (source string, destination string) {
	destination = destinationService
	if destinationService == "spotify" {
		source = "youtube"
	} else {
		source = "spotify"
	}
	return source, destination
}
