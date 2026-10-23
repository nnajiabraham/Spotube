package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"

	"github.com/manlikeabro/spotube/internal/apiusage"
	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/db/table"
	"github.com/manlikeabro/spotube/internal/quota"
	"github.com/manlikeabro/spotube/internal/schedule"
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
	// Schedule: every interval_minutes from schedule_start (HH:MM in schedule_tz) on
	// schedule_days (Sunday = 1 ... Saturday = 64).
	ScheduleStart *string `json:"schedule_start,omitempty"`
	ScheduleDays  *int    `json:"schedule_days,omitempty"`
	ScheduleTZ    *string `json:"schedule_tz,omitempty"`
	// CreatePlaylist ("spotify" or "youtube") creates that side's playlist instead of pairing an
	// existing one; its ID must then be empty. The name defaults to the other side's name.
	CreatePlaylist string `json:"create_playlist,omitempty"`
	// SyncNow starts the first sync straight away (default: the sync_new_mappings_now setting).
	SyncNow *bool `json:"sync_now,omitempty"`
}

// QueueCounts is how many of a mapping's queued tracks are in each state.
type QueueCounts struct {
	Pending int `json:"pending"`
	Review  int `json:"review"`
	Error   int `json:"error"`
	Done    int `json:"done"`
	Skipped int `json:"skipped"`
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
	ScheduleStart       string `json:"schedule_start"`
	ScheduleDays        int    `json:"schedule_days"`
	ScheduleTZ          string `json:"schedule_tz,omitempty"`
	NextRunAt           *int64 `json:"next_run_at"`
	LastAnalysisAt      *int64 `json:"last_analysis_at"`
	LastSyncStatus      string `json:"last_sync_status,omitempty"`
	LastSyncError       string `json:"last_sync_error,omitempty"`
	// TracksCount is the larger side; kept for older clients.
	TracksCount        int          `json:"tracks_count"`
	SpotifyTracksCount int          `json:"spotify_tracks_count"`
	YoutubeTracksCount int          `json:"youtube_tracks_count"`
	Queue              *QueueCounts `json:"queue,omitempty"`
	// Syncing is true while a sync for this mapping is running or waiting to start.
	Syncing bool  `json:"syncing"`
	Created int64 `json:"created"`
	Updated int64 `json:"updated"`
}

// MappingsListResponse represents paginated mappings response.
type MappingsListResponse struct {
	Items      []MappingResponse `json:"items"`
	Page       int               `json:"page"`
	PerPage    int               `json:"perPage"`
	TotalItems int64             `json:"totalItems"`
	TotalPages int               `json:"totalPages"`
}

// MappingSyncer runs syncs for mappings (the shared runner in internal/jobs).
type MappingSyncer interface {
	// SyncMapping analyses and applies now; it fails when the mapping is already syncing.
	SyncMapping(ctx context.Context, mapping model.Mappings) error
	// StartSync queues a sync in the background; false when one is already queued or running.
	StartSync(mapping model.Mappings) bool
	IsSyncing(mappingID string) bool
}

// PlaylistManager creates and deletes playlists.
type PlaylistManager interface {
	CreatePlaylist(ctx context.Context, service, name, description string) (string, error)
	DeletePlaylist(ctx context.Context, service, playlistID string) error
}

// youtubePlaylistCost is the quota for playlists.insert and playlists.delete.
const youtubePlaylistCost = 50

// MappingsHandler handles mapping CRUD operations.
type MappingsHandler struct {
	DB *sql.DB
	// Syncer powers Sync now and sync_now; nil disables them.
	Syncer MappingSyncer
	// Playlists powers create_playlist, mirror and deleting playlists; nil disables them.
	Playlists PlaylistManager
	// Budget refuses YouTube work that won't fit today; nil skips the check.
	Budget *quota.Budget
	// SyncNowDefault is the default of sync_now; nil means true.
	SyncNowDefault func() bool
	// DailyLimit is the YouTube quota for schedule advice; nil uses apiusage.Limit.
	DailyLimit func() int
	Now        func() time.Time
}

func NewMappingsHandler(db *sql.DB) *MappingsHandler {
	return &MappingsHandler{DB: db, Now: time.Now}
}

func RegisterMappingsRoutes(group *echo.Group, handler *MappingsHandler) {
	group.GET("", handler.List)
	group.POST("", handler.Create)
	group.GET("/schedule-recommendation", handler.Recommend)
	group.POST("/mirror", handler.Mirror)
	group.GET("/:id", handler.Get)
	group.PATCH("/:id", handler.Update)
	group.DELETE("/:id", handler.Delete)
	group.POST("/:id/sync", handler.SyncNow)
}

func (h *MappingsHandler) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

// SyncNow analyzes the mapping immediately (and applies part of its queue when auto-apply is
// on), then returns the mapping with its updated sync status.
func (h *MappingsHandler) SyncNow(c echo.Context) error {
	if h.Syncer == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "sync is not configured")
	}
	mapping, err := h.loadMapping(c.Param("id"))
	if err != nil {
		return err
	}
	// Finish the run even if the browser disconnects mid-request.
	if err := h.Syncer.SyncMapping(context.WithoutCancel(c.Request().Context()), mapping); err != nil {
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	}
	updated, err := h.loadMapping(c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.response(updated, nil))
}

func (h *MappingsHandler) loadMapping(id string) (model.Mappings, error) {
	if strings.TrimSpace(id) == "" {
		return model.Mappings{}, echo.NewHTTPError(http.StatusBadRequest, "missing id")
	}
	var mappings []model.Mappings
	err := table.Mappings.
		SELECT(table.Mappings.AllColumns).
		WHERE(table.Mappings.ID.EQ(sqlite.String(id))).
		Query(h.DB, &mappings)
	if err != nil {
		return model.Mappings{}, echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch mapping")
	}
	if len(mappings) == 0 {
		return model.Mappings{}, echo.NewHTTPError(http.StatusNotFound, "mapping not found")
	}
	return mappings[0], nil
}

func (h *MappingsHandler) List(c echo.Context) error {
	page := parseIntParam(c.QueryParam("page"), 1)
	perPage := parseIntParam(c.QueryParam("per_page"), 20)
	if perPage > 200 {
		perPage = 200
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

	queues, err := h.queueCounts(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to count queued tracks")
	}

	totalPages := int((totalCount.Count + int64(perPage) - 1) / int64(perPage))

	items := lo.Map(mappings, func(m model.Mappings, _ int) MappingResponse {
		q := queues[lo.FromPtr(m.ID)]
		return h.response(m, &q)
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

// queueCounts returns each mapping's queued tracks by status.
func (h *MappingsHandler) queueCounts(ctx context.Context) (map[string]QueueCounts, error) {
	rows, err := h.DB.QueryContext(ctx, `SELECT mapping_id, status, COUNT(*) FROM sync_items GROUP BY mapping_id, status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]QueueCounts{}
	for rows.Next() {
		var id, status string
		var n int
		if err := rows.Scan(&id, &status, &n); err != nil {
			return nil, err
		}
		q := out[id]
		switch status {
		case "pending", "running":
			q.Pending += n
		case "review":
			q.Review += n
		case "error":
			q.Error += n
		case "done":
			q.Done += n
		case "skipped":
			q.Skipped += n
		}
		out[id] = q
	}
	return out, rows.Err()
}

// scheduleFromRequest fills a schedule from the request over base.
func scheduleFromRequest(req MappingRequest, base schedule.Schedule) schedule.Schedule {
	if req.IntervalMinutes != nil {
		base.IntervalMinutes = *req.IntervalMinutes
	}
	if req.ScheduleStart != nil {
		base.Start = strings.TrimSpace(*req.ScheduleStart)
	}
	if req.ScheduleDays != nil {
		base.Days = *req.ScheduleDays
	}
	if req.ScheduleTZ != nil {
		base.TZ = strings.TrimSpace(*req.ScheduleTZ)
	}
	return base
}

func (h *MappingsHandler) Create(c echo.Context) error {
	var req MappingRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	if err := validateMappingRequest(req); err != nil {
		return err
	}
	sched := scheduleFromRequest(req, schedule.Default(60))
	if err := sched.Validate(); err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
	}
	if err := h.checkNotPaired(req); err != nil {
		return err
	}
	ctx := c.Request().Context()
	if req.CreatePlaylist != "" {
		if err := h.createMissingPlaylist(ctx, &req); err != nil {
			return err
		}
	}

	mapping, err := h.insertMapping(req, sched)
	if err != nil {
		h.cleanUpCreated(ctx, req)
		return err
	}
	syncNow := h.SyncNowDefault == nil || h.SyncNowDefault()
	if req.SyncNow != nil {
		syncNow = *req.SyncNow
	}
	if syncNow && h.Syncer != nil && (mapping.SyncTracks != 0 || mapping.SyncName != 0) {
		h.Syncer.StartSync(mapping)
	}
	return c.JSON(http.StatusCreated, h.response(mapping, &QueueCounts{}))
}

// insertMapping stores a new mapping; its first scheduled run is the schedule's next slot.
func (h *MappingsHandler) insertMapping(req MappingRequest, sched schedule.Schedule) (model.Mappings, error) {
	now := h.now()
	id := uuid.NewString()
	next := int32(sched.Next(now).Unix())
	mapping := model.Mappings{
		ID:                  &id,
		SpotifyPlaylistID:   req.SpotifyPlaylistID,
		YoutubePlaylistID:   req.YoutubePlaylistID,
		SpotifyPlaylistName: stringPtr(req.SpotifyPlaylistName),
		YoutubePlaylistName: stringPtr(req.YoutubePlaylistName),
		SyncName:            intFromBool(getBoolOrDefault(req.SyncName, true)),
		SyncTracks:          intFromBool(getBoolOrDefault(req.SyncTracks, true)),
		IntervalMinutes:     int32(sched.IntervalMinutes),
		ScheduleStart:       sched.Start,
		ScheduleDays:        int32(sched.Days),
		ScheduleTz:          stringPtr(sched.TZ),
		NextRunAt:           &next,
		Created:             int32(now.Unix()),
		Updated:             int32(now.Unix()),
	}
	_, err := table.Mappings.INSERT(table.Mappings.AllColumns).MODEL(mapping).Exec(h.DB)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return model.Mappings{}, echo.NewHTTPError(http.StatusConflict, "one of these playlists is already paired")
		}
		return model.Mappings{}, echo.NewHTTPError(http.StatusInternalServerError, "failed to create mapping")
	}
	return mapping, nil
}

// cleanUpCreated deletes a playlist Spotube just created when the mapping could not be saved,
// so a failed pairing leaves nothing behind.
func (h *MappingsHandler) cleanUpCreated(ctx context.Context, req MappingRequest) {
	if req.CreatePlaylist == "" || h.Playlists == nil {
		return
	}
	id := req.SpotifyPlaylistID
	if req.CreatePlaylist == "youtube" {
		id = req.YoutubePlaylistID
	}
	if id != "" {
		_ = h.Playlists.DeletePlaylist(context.WithoutCancel(ctx), req.CreatePlaylist, id)
	}
}

func (h *MappingsHandler) Get(c echo.Context) error {
	mapping, err := h.loadMapping(c.Param("id"))
	if err != nil {
		return err
	}
	queues, err := h.queueCounts(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to count queued tracks")
	}
	q := queues[lo.FromPtr(mapping.ID)]
	return c.JSON(http.StatusOK, h.response(mapping, &q))
}

func (h *MappingsHandler) Update(c echo.Context) error {
	id := c.Param("id")
	existing, err := h.loadMapping(id)
	if err != nil {
		return err
	}

	var req MappingRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	now := h.now().Unix()

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
	if req.IntervalMinutes != nil || req.ScheduleStart != nil || req.ScheduleDays != nil || req.ScheduleTZ != nil {
		current := schedule.Schedule{
			IntervalMinutes: int(existing.IntervalMinutes),
			Start:           existing.ScheduleStart,
			Days:            int(existing.ScheduleDays),
			TZ:              lo.FromPtr(existing.ScheduleTz),
		}
		sched := scheduleFromRequest(req, current)
		if err := sched.Validate(); err != nil {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
		}
		updateSQL += ", interval_minutes = ?, schedule_start = ?, schedule_days = ?, schedule_tz = ?, next_run_at = ? "
		args = append(args, sched.IntervalMinutes, sched.Start, sched.Days, stringPtr(sched.TZ), sched.Next(h.now()).Unix())
	}

	updateSQL += " WHERE id = ?"
	args = append(args, id)

	if _, err := h.DB.Exec(updateSQL, args...); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update mapping")
	}
	updated, err := h.loadMapping(id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.response(updated, nil))
}

// PlaylistDeletion is the outcome of deleting one side's playlist with its mapping.
type PlaylistDeletion struct {
	Service string `json:"service"`
	Deleted bool   `json:"deleted"`
	Error   string `json:"error,omitempty"`
}

// DeleteResponse is returned by DELETE /:id.
type DeleteResponse struct {
	Deleted   bool               `json:"deleted"`
	Playlists []PlaylistDeletion `json:"playlists"`
}

// Delete removes a mapping and, when asked (?delete_spotify_playlist=true,
// ?delete_youtube_playlist=true), its playlists. The mapping goes first; a playlist that
// can't be deleted is reported, not retried.
func (h *MappingsHandler) Delete(c echo.Context) error {
	mapping, err := h.loadMapping(c.Param("id"))
	if err != nil {
		return err
	}
	deleteSpotify := c.QueryParam("delete_spotify_playlist") == "true"
	deleteYouTube := c.QueryParam("delete_youtube_playlist") == "true"
	if (deleteSpotify || deleteYouTube) && h.Playlists == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "deleting playlists is not configured")
	}

	if _, err := table.Mappings.DELETE().WHERE(table.Mappings.ID.EQ(sqlite.String(lo.FromPtr(mapping.ID)))).Exec(h.DB); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete mapping")
	}
	_, _ = h.DB.Exec(`DELETE FROM playlist_cache WHERE (service = 'spotify' AND playlist_id = ?) OR (service = 'youtube' AND playlist_id = ?)`,
		mapping.SpotifyPlaylistID, mapping.YoutubePlaylistID)

	resp := DeleteResponse{Deleted: true, Playlists: []PlaylistDeletion{}}
	ctx := context.WithoutCancel(c.Request().Context())
	remove := func(service, playlistID string) {
		d := PlaylistDeletion{Service: service}
		if service == "youtube" && h.Budget != nil {
			if err := h.Budget.Allows(quota.Interactive, youtubePlaylistCost); err != nil {
				d.Error = err.Error()
				resp.Playlists = append(resp.Playlists, d)
				return
			}
		}
		if err := h.Playlists.DeletePlaylist(ctx, service, playlistID); err != nil {
			d.Error = err.Error()
		} else {
			d.Deleted = true
		}
		resp.Playlists = append(resp.Playlists, d)
	}
	if deleteSpotify {
		remove("spotify", mapping.SpotifyPlaylistID)
	}
	if deleteYouTube {
		remove("youtube", mapping.YoutubePlaylistID)
	}
	return c.JSON(http.StatusOK, resp)
}

// Recommend runs GET /schedule-recommendation?spotify_tracks=&youtube_videos=&tz=: a schedule
// for a new mapping that keeps reads affordable and spreads it from the others.
func (h *MappingsHandler) Recommend(c echo.Context) error {
	spotifyTracks, _ := strconv.Atoi(c.QueryParam("spotify_tracks"))
	youtubeVideos, _ := strconv.Atoi(c.QueryParam("youtube_videos"))
	tz := strings.TrimSpace(c.QueryParam("tz"))
	if tz != "" {
		if _, err := time.LoadLocation(tz); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "unknown time zone")
		}
	}
	existing, err := h.existingSchedules()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load schedules")
	}
	return c.JSON(http.StatusOK, schedule.Recommend(max(spotifyTracks, 0), max(youtubeVideos, 0), existing, h.dailyLimit(), tz, h.now()))
}

func (h *MappingsHandler) dailyLimit() int {
	if h.DailyLimit != nil {
		return h.DailyLimit()
	}
	return apiusage.Limit()
}

func (h *MappingsHandler) existingSchedules() ([]schedule.Existing, error) {
	var mappings []model.Mappings
	if err := table.Mappings.SELECT(table.Mappings.AllColumns).Query(h.DB, &mappings); err != nil {
		return nil, err
	}
	out := make([]schedule.Existing, 0, len(mappings))
	for _, m := range mappings {
		if m.SyncTracks == 0 && m.SyncName == 0 {
			continue
		}
		out = append(out, schedule.Existing{
			Schedule: schedule.Schedule{
				IntervalMinutes: int(m.IntervalMinutes),
				Start:           m.ScheduleStart,
				Days:            int(m.ScheduleDays),
				TZ:              lo.FromPtr(m.ScheduleTz),
			},
			YouTubeVideos: int(max(m.YoutubeTracksCount, m.SpotifyTracksCount)),
		})
	}
	return out, nil
}

// MirrorPlaylist is one playlist to mirror onto the other service.
type MirrorPlaylist struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Tracks int    `json:"tracks"`
}

// MirrorRequest creates the counterpart of each playlist on the other service, each as its
// own 1:1 mapping with a recommended schedule.
type MirrorRequest struct {
	// Source is the service the playlists exist on.
	Source     string           `json:"source"`
	Playlists  []MirrorPlaylist `json:"playlists"`
	SyncName   *bool            `json:"sync_name"`
	SyncTracks *bool            `json:"sync_tracks"`
	SyncNow    *bool            `json:"sync_now"`
	ScheduleTZ string           `json:"schedule_tz"`
}

// MirrorResult is the outcome for one playlist. Status: created, already_paired,
// waiting_quota or failed.
type MirrorResult struct {
	PlaylistID string           `json:"playlist_id"`
	Name       string           `json:"name"`
	Status     string           `json:"status"`
	Error      string           `json:"error,omitempty"`
	Mapping    *MappingResponse `json:"mapping,omitempty"`
}

// maxMirror caps playlists per mirror request.
const maxMirror = 100

// Mirror runs POST /mirror. Each playlist is handled on its own; partial success is normal.
func (h *MappingsHandler) Mirror(c echo.Context) error {
	var req MirrorRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	if req.Source != "spotify" && req.Source != "youtube" {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "source must be spotify or youtube")
	}
	if len(req.Playlists) == 0 || len(req.Playlists) > maxMirror {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "pick 1-100 playlists")
	}
	if req.ScheduleTZ != "" {
		if _, err := time.LoadLocation(req.ScheduleTZ); err != nil {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "unknown time zone")
		}
	}
	if h.Playlists == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "creating playlists is not configured")
	}
	target := "youtube"
	if req.Source == "youtube" {
		target = "spotify"
	}
	syncNow := h.SyncNowDefault == nil || h.SyncNowDefault()
	if req.SyncNow != nil {
		syncNow = *req.SyncNow
	}

	ctx := c.Request().Context()
	results := make([]MirrorResult, 0, len(req.Playlists))
	quotaShort := false
	for _, p := range req.Playlists {
		res := MirrorResult{PlaylistID: p.ID, Name: strings.TrimSpace(p.Name)}
		mreq := MappingRequest{SyncName: req.SyncName, SyncTracks: req.SyncTracks, CreatePlaylist: target}
		if req.Source == "spotify" {
			mreq.SpotifyPlaylistID, mreq.SpotifyPlaylistName = p.ID, res.Name
		} else {
			mreq.YoutubePlaylistID, mreq.YoutubePlaylistName = p.ID, res.Name
		}
		switch {
		case strings.TrimSpace(p.ID) == "" || res.Name == "":
			res.Status, res.Error = "failed", "playlist id and name are required"
		case quotaShort:
			res.Status, res.Error = "waiting_quota", "not enough YouTube quota left today; try again after the reset"
		default:
			if err := h.checkNotPaired(mreq); err != nil {
				res.Status, res.Error = "already_paired", "already paired"
				break
			}
			if err := h.createMissingPlaylist(ctx, &mreq); err != nil {
				var he *echo.HTTPError
				if errors.As(err, &he) && he.Code == http.StatusTooManyRequests {
					quotaShort = true
					res.Status = "waiting_quota"
				} else {
					res.Status = "failed"
				}
				res.Error = errorMessage(err)
				break
			}
			existing, err := h.existingSchedules()
			if err != nil {
				h.cleanUpCreated(ctx, mreq)
				res.Status, res.Error = "failed", "failed to load schedules"
				break
			}
			// Size the schedule by the source playlist; the copy grows the other side to match.
			rec := schedule.Recommend(p.Tracks, p.Tracks, existing, h.dailyLimit(), req.ScheduleTZ, h.now())
			mapping, err := h.insertMapping(mreq, rec.Schedule)
			if err != nil {
				h.cleanUpCreated(ctx, mreq)
				res.Status, res.Error = "failed", errorMessage(err)
				break
			}
			if syncNow && h.Syncer != nil {
				h.Syncer.StartSync(mapping)
			}
			resp := h.response(mapping, &QueueCounts{})
			res.Status, res.Mapping = "created", &resp
		}
		results = append(results, res)
	}
	return c.JSON(http.StatusOK, map[string]any{"items": results})
}

func errorMessage(err error) string {
	var he *echo.HTTPError
	if errors.As(err, &he) {
		if s, ok := he.Message.(string); ok {
			return s
		}
	}
	return err.Error()
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

// checkNotPaired rejects a playlist that is already in a mapping: each playlist syncs with one
// partner, so a second mapping would sync it twice. (Unique indexes enforce this too.)
func (h *MappingsHandler) checkNotPaired(req MappingRequest) error {
	var n int
	err := h.DB.QueryRow(
		`SELECT COUNT(*) FROM mappings WHERE (? != '' AND spotify_playlist_id = ?) OR (? != '' AND youtube_playlist_id = ?)`,
		req.SpotifyPlaylistID, req.SpotifyPlaylistID, req.YoutubePlaylistID, req.YoutubePlaylistID,
	).Scan(&n)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to check existing mappings")
	}
	if n > 0 {
		return echo.NewHTTPError(http.StatusConflict, "one of these playlists is already paired")
	}
	return nil
}

// createMissingPlaylist creates the playlist named by req.CreatePlaylist and fills in its ID
// and name. YouTube creation is refused when today's quota can't cover it.
func (h *MappingsHandler) createMissingPlaylist(ctx context.Context, req *MappingRequest) error {
	if h.Playlists == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "creating playlists is not configured")
	}
	service := req.CreatePlaylist
	name, from := &req.YoutubePlaylistName, req.SpotifyPlaylistName
	if service == "spotify" {
		name, from = &req.SpotifyPlaylistName, req.YoutubePlaylistName
	}
	if strings.TrimSpace(*name) == "" {
		*name = from
	}
	*name = strings.TrimSpace(*name)
	if *name == "" {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "a name is required for the new playlist")
	}
	if service == "youtube" && h.Budget != nil {
		if err := h.Budget.Allows(quota.Interactive, youtubePlaylistCost); err != nil {
			return echo.NewHTTPError(http.StatusTooManyRequests, "not enough YouTube quota left today to create a playlist")
		}
	}
	id, err := h.Playlists.CreatePlaylist(ctx, service, *name, "Synced by Spotube.")
	if err != nil {
		if errors.Is(err, quota.ErrBudget) {
			return echo.NewHTTPError(http.StatusTooManyRequests, "not enough YouTube quota left today to create a playlist")
		}
		return echo.NewHTTPError(http.StatusBadGateway, "couldn't create the playlist: "+err.Error())
	}
	if service == "spotify" {
		req.SpotifyPlaylistID = id
	} else {
		req.YoutubePlaylistID = id
	}
	return nil
}

func validateMappingRequest(req MappingRequest) error {
	switch req.CreatePlaylist {
	case "":
	case "spotify", "youtube":
		id := req.SpotifyPlaylistID
		if req.CreatePlaylist == "youtube" {
			id = req.YoutubePlaylistID
		}
		if strings.TrimSpace(id) != "" {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, req.CreatePlaylist+"_playlist_id must be empty when creating that playlist")
		}
	default:
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "create_playlist must be spotify or youtube")
	}
	if strings.TrimSpace(req.SpotifyPlaylistID) == "" && req.CreatePlaylist != "spotify" {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "spotify_playlist_id is required")
	}
	if strings.TrimSpace(req.YoutubePlaylistID) == "" && req.CreatePlaylist != "youtube" {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, "youtube_playlist_id is required")
	}
	return nil
}

func getBoolOrDefault(ptr *bool, defaultVal bool) bool {
	if ptr == nil {
		return defaultVal
	}
	return *ptr
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

func (h *MappingsHandler) response(m model.Mappings, queue *QueueCounts) MappingResponse {
	r := modelToResponse(m)
	r.Queue = queue
	if h.Syncer != nil {
		r.Syncing = h.Syncer.IsSyncing(r.ID)
	}
	return r
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
		ScheduleStart:       m.ScheduleStart,
		ScheduleDays:        int(m.ScheduleDays),
		ScheduleTZ:          stringFromPtr(m.ScheduleTz),
		NextRunAt:           int64FromPtr(m.NextRunAt),
		LastAnalysisAt:      int64FromPtr(m.LastAnalysisAt),
		LastSyncStatus:      stringFromPtr(m.LastSyncStatus),
		LastSyncError:       stringFromPtr(m.LastSyncError),
		TracksCount:         int(m.TracksCount),
		SpotifyTracksCount:  int(m.SpotifyTracksCount),
		YoutubeTracksCount:  int(m.YoutubeTracksCount),
		Created:             int64(m.Created),
		Updated:             int64(m.Updated),
	}
}
