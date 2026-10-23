package backups

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/manlikeabro/spotube/internal/activitylogger"
	"github.com/manlikeabro/spotube/internal/matching"
)

// Reader fetches a playlist's name and tracks in order.
type Reader interface {
	PlaylistTracks(ctx context.Context, service, playlistID string) (name string, tracks []matching.Track, err error)
}

// Writer creates playlists and adds tracks to them.
type Writer interface {
	CreatePlaylist(ctx context.Context, service, name, description string) (id string, err error)
	// AddTracks adds ids in order and returns how many were added before any error.
	AddTracks(ctx context.Context, service, playlistID string, ids []string) (added int, err error)
}

// YouTube quota units per call (see platform package).
const (
	youtubeInsertCost = 50
	youtubeReadCost   = 2 // per 50 videos: playlistItems.list + videos.list
)

// ErrQuota means a YouTube restore would use more quota than is left today.
type ErrQuota struct {
	Needed, Left int
}

func (e *ErrQuota) Error() string {
	return fmt.Sprintf("this restore needs about %d YouTube quota units but only %d are left today; try again after midnight Pacific", e.Needed, e.Left)
}

// Service takes snapshots on demand or on schedule and restores them.
type Service struct {
	Store    *Store
	Reader   Reader
	Writer   Writer
	Activity *activitylogger.Logger
	Logger   zerolog.Logger
	// QuotaLeft returns YouTube quota units left today; nil means unlimited.
	QuotaLeft func() int
}

// BackUp fetches a playlist and records it. It returns the snapshot now describing the
// playlist and whether it is new.
func (s *Service) BackUp(ctx context.Context, b Backup, source string) (*SnapshotSummary, bool, error) {
	name, tracks, err := s.Reader.PlaylistTracks(ctx, b.Service, b.PlaylistID)
	if err != nil {
		_ = s.Store.MarkError(ctx, b.ID, err)
		return nil, false, err
	}
	return s.Store.Record(ctx, b, name, tracks, source)
}

// RunDue backs up every enabled playlist whose interval has elapsed.
func (s *Service) RunDue(ctx context.Context) {
	due, err := s.Store.Due(ctx)
	if err != nil {
		s.Logger.Error().Err(err).Msg("backups: list due failed")
		return
	}
	for _, b := range due {
		if ctx.Err() != nil {
			return
		}
		snap, created, err := s.BackUp(ctx, b, SourceScheduled)
		if err != nil {
			s.Logger.Warn().Err(err).Str("backup_id", b.ID).Msg("backup failed")
			s.log("error", fmt.Sprintf("Backup of %q failed: %v", label(b), err))
			continue
		}
		s.Logger.Info().Str("backup_id", b.ID).Bool("changed", created).Int("tracks", snap.TrackCount).Msg("backup checked")
		if created {
			s.log("info", fmt.Sprintf("Backed up %q (%d tracks)", label(b), snap.TrackCount))
		}
	}
}

// Start runs RunDue now and then every interval until ctx is done.
func (s *Service) Start(ctx context.Context, interval time.Duration) {
	go func() {
		s.RunDue(ctx)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.RunDue(ctx)
			}
		}
	}()
}

const (
	RestoreNew     = "new"
	RestoreMissing = "missing"
)

// RestoreRequest says how to restore a snapshot. Service defaults to the backup's own service;
// a different service rebuilds the playlist from the snapshot's linked counterpart IDs.
type RestoreRequest struct {
	Mode    string `json:"mode"`
	Service string `json:"service,omitempty"`
	Name    string `json:"name,omitempty"`
}

// RestoreResult reports what a restore did.
type RestoreResult struct {
	Mode         string `json:"mode"`
	Service      string `json:"service"`
	PlaylistID   string `json:"playlist_id"`
	PlaylistName string `json:"playlist_name"`
	PlaylistURL  string `json:"playlist_url"`
	Added        int    `json:"added"`
	// AlreadyPresent counts snapshot tracks still in the playlist (missing mode).
	AlreadyPresent int `json:"already_present"`
	// NoCounterpart lists tracks with no known ID on the target service.
	NoCounterpart []Track `json:"no_counterpart"`
	// Error is set when the restore stopped part way.
	Error string `json:"error,omitempty"`
}

// Restore rebuilds a snapshot as a new playlist, or adds its missing tracks back to the
// original playlist. It never removes tracks.
func (s *Service) Restore(ctx context.Context, snapshotID string, req RestoreRequest) (*RestoreResult, error) {
	snap, err := s.Store.Snapshot(ctx, snapshotID)
	if err != nil {
		return nil, err
	}
	b, err := s.Store.Get(ctx, snap.BackupID)
	if err != nil {
		return nil, err
	}
	target := req.Service
	if target == "" {
		target = b.Service
	}
	if target != "spotify" && target != "youtube" {
		return nil, fmt.Errorf("%w: service must be spotify or youtube", ErrInvalid)
	}
	if req.Mode == "" {
		req.Mode = RestoreNew
	}
	if req.Mode != RestoreNew && req.Mode != RestoreMissing {
		return nil, fmt.Errorf("%w: mode must be new or missing", ErrInvalid)
	}
	if req.Mode == RestoreMissing && target != b.Service {
		return nil, fmt.Errorf("%w: missing tracks can only be restored into the original playlist", ErrInvalid)
	}

	result := &RestoreResult{Mode: req.Mode, Service: target, NoCounterpart: []Track{}}
	ids := make([]string, 0, len(snap.Tracks))
	for _, t := range snap.Tracks {
		id := t.ID
		if target != b.Service {
			id = t.IDOn(target)
		}
		if id == "" {
			result.NoCounterpart = append(result.NoCounterpart, t)
			continue
		}
		ids = append(ids, id)
	}

	if req.Mode == RestoreMissing {
		result.PlaylistID, result.PlaylistName = b.PlaylistID, b.PlaylistName
		if err := s.checkQuota(target, youtubeReadCost*(len(snap.Tracks)/50+1)); err != nil {
			return nil, err
		}
		_, current, err := s.Reader.PlaylistTracks(ctx, target, b.PlaylistID)
		if err != nil {
			return nil, err
		}
		present := map[string]bool{}
		for _, t := range current {
			present[t.ID] = true
		}
		var missing []string
		for _, id := range ids {
			if present[id] {
				result.AlreadyPresent++
				continue
			}
			present[id] = true // add each missing track once
			missing = append(missing, id)
		}
		ids = missing
		if err := s.checkQuota(target, youtubeInsertCost*len(ids)); err != nil {
			return nil, err
		}
	} else {
		if err := s.checkQuota(target, youtubeInsertCost*(len(ids)+1)); err != nil {
			return nil, err
		}
		name := req.Name
		if name == "" {
			base := snap.PlaylistName
			if base == "" {
				base = b.PlaylistName
			}
			name = fmt.Sprintf("%s (restored %s)", base, time.Unix(snap.Created, 0).Format("2006-01-02"))
		}
		description := fmt.Sprintf("Restored by Spotube from a backup taken %s.", time.Unix(snap.Created, 0).Format("2 Jan 2006 15:04"))
		id, err := s.Writer.CreatePlaylist(ctx, target, name, description)
		if err != nil {
			return nil, err
		}
		result.PlaylistID, result.PlaylistName = id, name
	}
	result.PlaylistURL = PlaylistURL(target, result.PlaylistID)

	if len(ids) > 0 {
		added, err := s.Writer.AddTracks(ctx, target, result.PlaylistID, ids)
		result.Added = added
		if err != nil {
			result.Error = err.Error()
		}
	}

	msg := fmt.Sprintf("Restored backup of %q from %s: added %d tracks to %q",
		label(*b), time.Unix(snap.Created, 0).Format("2 Jan 15:04"), result.Added, result.PlaylistName)
	if result.Error != "" {
		s.log("error", msg+" before an error: "+result.Error)
	} else {
		s.log("info", msg)
	}
	return result, nil
}

// EstimateRestoreCost returns the YouTube quota units a restore would use (0 for Spotify).
func EstimateRestoreCost(service, mode string, tracks int) int {
	if service != "youtube" {
		return 0
	}
	if mode == RestoreMissing {
		return youtubeReadCost*(tracks/50+1) + youtubeInsertCost*tracks
	}
	return youtubeInsertCost * (tracks + 1)
}

func (s *Service) checkQuota(service string, needed int) error {
	if service != "youtube" || s.QuotaLeft == nil || needed == 0 {
		return nil
	}
	if left := s.QuotaLeft(); needed > left {
		return &ErrQuota{Needed: needed, Left: left}
	}
	return nil
}

func (s *Service) log(level, msg string) {
	if s.Activity == nil {
		return
	}
	_ = s.Activity.Record(level, msg, "", "system")
}

// PlaylistURL links to a playlist on its service.
func PlaylistURL(service, id string) string {
	if service == "spotify" {
		return "https://open.spotify.com/playlist/" + id
	}
	return "https://www.youtube.com/playlist?list=" + id
}

func label(b Backup) string {
	if b.PlaylistName != "" {
		return b.PlaylistName
	}
	return b.PlaylistID
}
