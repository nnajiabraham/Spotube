package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/manlikeabro/spotube/internal/backups"
)

// BackupsHandler serves playlist backups: which playlists are backed up, their snapshots,
// export and restore.
type BackupsHandler struct {
	Store   *backups.Store
	Service *backups.Service
}

func NewBackupsHandler(service *backups.Service) *BackupsHandler {
	return &BackupsHandler{Store: service.Store, Service: service}
}

// RegisterBackupsRoutes mounts the handler on group (normally /api/backups).
func RegisterBackupsRoutes(group *echo.Group, h *BackupsHandler) {
	group.GET("", h.List)
	group.POST("", h.Create)
	group.GET("/snapshots/:sid", h.Snapshot)
	group.GET("/snapshots/:sid/export", h.Export)
	group.POST("/snapshots/:sid/restore", h.Restore)
	group.GET("/:id", h.Get)
	group.PATCH("/:id", h.Update)
	group.DELETE("/:id", h.Delete)
	group.POST("/:id/run", h.Run)
	group.GET("/:id/snapshots", h.Snapshots)
}

func (h *BackupsHandler) List(c echo.Context) error {
	list, err := h.Store.List(c.Request().Context())
	if err != nil {
		return err
	}
	if list == nil {
		list = []backups.Backup{}
	}
	return c.JSON(http.StatusOK, map[string]any{"items": list})
}

func (h *BackupsHandler) Get(c echo.Context) error {
	b, err := h.Store.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		return backupError(err)
	}
	return c.JSON(http.StatusOK, b)
}

type createBackupRequest struct {
	Service       string `json:"service"`
	PlaylistID    string `json:"playlist_id"`
	PlaylistName  string `json:"playlist_name"`
	IntervalHours int    `json:"interval_hours"`
	KeepSnapshots int    `json:"keep_snapshots"`
}

// Create starts backing up a playlist and takes its first snapshot. A failed first snapshot
// still creates the backup; the error is shown on it and the scheduler retries.
func (h *BackupsHandler) Create(c echo.Context) error {
	var req createBackupRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	ctx := c.Request().Context()
	b, err := h.Store.Create(ctx, req.Service, req.PlaylistID, req.PlaylistName, req.IntervalHours, req.KeepSnapshots)
	if errors.Is(err, backups.ErrExists) {
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	}
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	_, _, _ = h.Service.BackUp(ctx, *b, backups.SourceInitial)
	b, err = h.Store.Get(ctx, b.ID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, b)
}

type updateBackupRequest struct {
	Enabled       *bool `json:"enabled"`
	IntervalHours *int  `json:"interval_hours"`
	KeepSnapshots *int  `json:"keep_snapshots"`
}

func (h *BackupsHandler) Update(c echo.Context) error {
	var req updateBackupRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	b, err := h.Store.Update(c.Request().Context(), c.Param("id"), req.Enabled, req.IntervalHours, req.KeepSnapshots)
	if err != nil {
		return backupError(err)
	}
	return c.JSON(http.StatusOK, b)
}

func (h *BackupsHandler) Delete(c echo.Context) error {
	if err := h.Store.Delete(c.Request().Context(), c.Param("id")); err != nil {
		return backupError(err)
	}
	return c.NoContent(http.StatusNoContent)
}

// Run backs a playlist up now.
func (h *BackupsHandler) Run(c echo.Context) error {
	ctx := c.Request().Context()
	b, err := h.Store.Get(ctx, c.Param("id"))
	if err != nil {
		return backupError(err)
	}
	snap, created, err := h.Service.BackUp(ctx, *b, backups.SourceManual)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "backup failed: "+err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"snapshot": snap, "changed": created})
}

func (h *BackupsHandler) Snapshots(c echo.Context) error {
	ctx := c.Request().Context()
	if _, err := h.Store.Get(ctx, c.Param("id")); err != nil {
		return backupError(err)
	}
	list, err := h.Store.Snapshots(ctx, c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"items": list})
}

func (h *BackupsHandler) Snapshot(c echo.Context) error {
	snap, err := h.Store.Snapshot(c.Request().Context(), c.Param("sid"))
	if err != nil {
		return backupError(err)
	}
	return c.JSON(http.StatusOK, snap)
}

var unsafeFilename = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// Export downloads a snapshot as JSON (default) or CSV.
func (h *BackupsHandler) Export(c echo.Context) error {
	ctx := c.Request().Context()
	snap, err := h.Store.Snapshot(ctx, c.Param("sid"))
	if err != nil {
		return backupError(err)
	}
	b, err := h.Store.Get(ctx, snap.BackupID)
	if err != nil {
		return backupError(err)
	}
	export := backups.NewExport(*b, *snap)
	base := strings.Trim(unsafeFilename.ReplaceAllString(export.PlaylistName, "-"), "-")
	if base == "" {
		base = "playlist"
	}
	filename := fmt.Sprintf("%s-%s-%s", base, b.Service, time.Unix(snap.Created, 0).Format("2006-01-02-1504"))

	if c.QueryParam("format") == "csv" {
		body, err := export.CSV()
		if err != nil {
			return err
		}
		c.Response().Header().Set("Content-Disposition", `attachment; filename="`+filename+`.csv"`)
		return c.Blob(http.StatusOK, "text/csv; charset=utf-8", body)
	}
	body, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return err
	}
	c.Response().Header().Set("Content-Disposition", `attachment; filename="`+filename+`.json"`)
	return c.Blob(http.StatusOK, echo.MIMEApplicationJSONCharsetUTF8, body)
}

func (h *BackupsHandler) Restore(c echo.Context) error {
	var req backups.RestoreRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body")
	}
	result, err := h.Service.Restore(c.Request().Context(), c.Param("sid"), req)
	var quota *backups.ErrQuota
	switch {
	case errors.As(err, &quota):
		return echo.NewHTTPError(http.StatusTooManyRequests, err.Error())
	case errors.Is(err, backups.ErrNotFound):
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	case errors.Is(err, backups.ErrInvalid):
		return echo.NewHTTPError(http.StatusBadRequest, strings.TrimPrefix(err.Error(), "invalid request: "))
	case err != nil:
		return echo.NewHTTPError(http.StatusBadGateway, "restore failed: "+err.Error())
	}
	return c.JSON(http.StatusOK, result)
}

func backupError(err error) error {
	if errors.Is(err, backups.ErrNotFound) {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	return err
}
