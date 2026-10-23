package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/manlikeabro/spotube/internal/jobs"
	"github.com/manlikeabro/spotube/internal/platform"
	"github.com/manlikeabro/spotube/internal/tracklinks"
)

// Resolve actions for a sync item the engine could not finish on its own.
const (
	// ResolveMatch links the source track to target_id and adds it right away.
	ResolveMatch = "match"
	// ResolveUnavailable records that the song is not on the destination service.
	ResolveUnavailable = "unavailable"
	// ResolveRetry runs a fresh automatic search.
	ResolveRetry = "retry"
)

// ResolveRequest is the body of POST /sync_items/records/:id/resolve.
type ResolveRequest struct {
	Action string `json:"action"`
	// TargetID is a track ID, URL or URI on the destination service (match only).
	TargetID string `json:"target_id"`
}

// resolvableStatuses are the states a user may decide on; done and running are final or busy.
var resolvableStatuses = map[string]bool{"review": true, "error": true, "pending": true, "skipped": true}

// Resolve applies the user's decision for one add item. Decisions are remembered as track
// links, so every other mapping holding the same song is settled too.
func (h *SyncItemsHandler) Resolve(c echo.Context) error {
	id := strings.TrimSpace(c.Param("id"))
	var req ResolveRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	item, _, err := h.loadItemWithMapping(id)
	if errors.Is(err, errSyncItemNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "sync item not found")
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch sync item")
	}
	if item.Operation != "add" {
		return echo.NewHTTPError(http.StatusBadRequest, "only add items can be resolved")
	}
	if !resolvableStatuses[item.Status] {
		return echo.NewHTTPError(http.StatusConflict, "sync item is "+item.Status)
	}

	ctx := c.Request().Context()
	links := &tracklinks.Store{DB: h.DB}
	source := jobs.SourceTrack(item)
	dest := item.Service

	switch req.Action {
	case ResolveMatch:
		target, ok := parseTrackRef(dest, req.TargetID)
		if !ok {
			return echo.NewHTTPError(http.StatusBadRequest, "target_id is not a valid "+serviceName(dest)+" track")
		}
		spotifyID, youtubeID := source.ID, target
		if dest == "spotify" {
			spotifyID, youtubeID = target, source.ID
		}
		if _, err := links.Set(ctx, spotifyID, youtubeID, source.Title, source.Artist); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to save match")
		}
		// Items for the same song in other mappings can now be added without a search.
		if err := h.requeueSong(ctx, dest, source.ID, "review"); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to requeue item")
		}
		if err := h.requeue(ctx, id); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to requeue item")
		}
		return h.executeAndRespond(c, id)

	case ResolveUnavailable:
		spotifyID, youtubeID := source.ID, ""
		if dest == "spotify" {
			spotifyID, youtubeID = "", source.ID
		}
		if _, err := links.Set(ctx, spotifyID, youtubeID, source.Title, source.Artist); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to save decision")
		}
		now := time.Now().Unix()
		if _, err := h.DB.ExecContext(ctx,
			`UPDATE sync_items SET status = 'skipped', error_message = ?, updated = ?
			 WHERE operation = 'add' AND service = ? AND track_id = ? AND status IN ('pending', 'error', 'review')`,
			"not on "+serviceName(dest), now, dest, source.ID); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to skip items")
		}
		// The item itself may have been skipped already for another reason.
		if _, err := h.DB.ExecContext(ctx,
			`UPDATE sync_items SET status = 'skipped', error_message = ?, updated = ? WHERE id = ?`,
			"not on "+serviceName(dest), now, id); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to skip item")
		}
		updated, names, err := h.loadItemWithMapping(id)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to reload sync item")
		}
		return c.JSON(http.StatusOK, enrichSyncItemResponse(updated, names))

	case ResolveRetry:
		if err := h.requeue(ctx, id); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to requeue item")
		}
		return h.executeAndRespond(c, id)
	}
	return echo.NewHTTPError(http.StatusBadRequest, "action must be match, unavailable or retry")
}

// requeue puts one item back to pending with a fresh attempt budget.
func (h *SyncItemsHandler) requeue(ctx context.Context, id string) error {
	_, err := h.DB.ExecContext(ctx,
		`UPDATE sync_items SET status = 'pending', error_message = NULL, attempt_count = 0, updated = ? WHERE id = ?`,
		time.Now().Unix(), id)
	return err
}

// requeueSong moves every item copying sourceID to service from status back to pending.
func (h *SyncItemsHandler) requeueSong(ctx context.Context, service, sourceID, status string) error {
	_, err := h.DB.ExecContext(ctx,
		`UPDATE sync_items SET status = 'pending', error_message = NULL, attempt_count = 0, updated = ?
		 WHERE operation = 'add' AND service = ? AND track_id = ? AND status = ?`,
		time.Now().Unix(), service, sourceID, status)
	return err
}

func (h *SyncItemsHandler) executeAndRespond(c echo.Context, id string) error {
	if h.Executor == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "executor not configured")
	}
	item, execErr := h.Executor.ExecuteOne(c.Request().Context(), id)
	if errors.Is(execErr, jobs.ErrSyncItemNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, "sync item not found")
	}
	names, err := h.loadMappingByID(item.MappingID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load mapping")
	}
	response := SyncItemExecuteResponse{SyncItemResponse: enrichSyncItemResponse(item, names)}
	if execErr != nil {
		response.ExecutionLog = execErr.Error()
	}
	return c.JSON(http.StatusOK, response)
}

// attachLastRuns adds each item's most recent executor run (search query, candidates and
// scores), so the review queue can show alternatives without a new search.
func (h *SyncItemsHandler) attachLastRuns(ctx context.Context, items []SyncItemResponse) error {
	if len(items) == 0 {
		return nil
	}
	index := make(map[string]int, len(items))
	args := make([]any, 0, len(items))
	placeholders := make([]string, 0, len(items))
	for i, item := range items {
		index[item.ID] = i
		args = append(args, item.ID)
		placeholders = append(placeholders, "?")
	}
	rows, err := h.DB.QueryContext(ctx,
		`SELECT sync_item_id, json_extract(details_json, '$.data') FROM activity_logs
		 WHERE job_type = 'executor' AND details_json IS NOT NULL
		   AND sync_item_id IN (`+strings.Join(placeholders, ",")+`)
		 ORDER BY created DESC, id DESC`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var itemID string
		var data *string
		if err := rows.Scan(&itemID, &data); err != nil {
			return err
		}
		i := index[itemID]
		if data == nil || items[i].LastRun != nil {
			continue
		}
		items[i].LastRun = json.RawMessage(*data)
	}
	return rows.Err()
}

func parseTrackRef(service, ref string) (string, bool) {
	if service == "spotify" {
		return platform.ParseSpotifyTrackRef(ref)
	}
	return platform.ParseYouTubeVideoRef(ref)
}

func serviceName(service string) string {
	if service == "youtube" {
		return "YouTube"
	}
	return "Spotify"
}
