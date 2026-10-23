// Package backups keeps point-in-time copies of chosen playlists on either service and
// restores them. A snapshot is stored only when a playlist changes; a check that finds the
// same tracks moves the latest snapshot's confirmed_at forward instead.
package backups

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/manlikeabro/spotube/internal/matching"
	"github.com/manlikeabro/spotube/internal/tracklinks"
)

const (
	SourceInitial   = "initial"
	SourceScheduled = "scheduled"
	SourceManual    = "manual"
	SourceSync      = "sync"

	DefaultIntervalHours = 24
	DefaultKeepSnapshots = 30
)

var (
	ErrNotFound = errors.New("backup not found")
	ErrExists   = errors.New("this playlist is already backed up")
	// ErrInvalid wraps errors caused by a bad request.
	ErrInvalid = errors.New("invalid request")
)

// Backup is a playlist the user chose to back up.
type Backup struct {
	ID            string `json:"id"`
	Service       string `json:"service"`
	PlaylistID    string `json:"playlist_id"`
	PlaylistName  string `json:"playlist_name"`
	Enabled       bool   `json:"enabled"`
	IntervalHours int    `json:"interval_hours"`
	KeepSnapshots int    `json:"keep_snapshots"`
	LastCheckedAt *int64 `json:"last_checked_at,omitempty"`
	LastError     string `json:"last_error,omitempty"`
	Created       int64  `json:"created"`
	Updated       int64  `json:"updated"`

	SnapshotCount int              `json:"snapshot_count"`
	Latest        *SnapshotSummary `json:"latest_snapshot,omitempty"`
}

// Track is one entry of a snapshot, in playlist order. SpotifyID and YouTubeID are the
// track's IDs on each service when known (its own ID plus the linked counterpart).
type Track struct {
	Position   int    `json:"position"`
	ID         string `json:"id"`
	Title      string `json:"title"`
	Artist     string `json:"artist,omitempty"`
	RawTitle   string `json:"raw_title,omitempty"`
	Channel    string `json:"channel,omitempty"`
	DurationMs int    `json:"duration_ms,omitempty"`
	SpotifyID  string `json:"spotify_id,omitempty"`
	YouTubeID  string `json:"youtube_id,omitempty"`
}

// IDOn returns the track's ID on service, or "" when unknown.
func (t Track) IDOn(service string) string {
	if service == "spotify" {
		return t.SpotifyID
	}
	return t.YouTubeID
}

// SnapshotSummary describes one stored playlist state without its tracks.
type SnapshotSummary struct {
	ID           string `json:"id"`
	BackupID     string `json:"backup_id"`
	PlaylistName string `json:"playlist_name"`
	TrackCount   int    `json:"track_count"`
	// Added and Removed compare with the previous snapshot; nil for the first one.
	Added       *int   `json:"added,omitempty"`
	Removed     *int   `json:"removed,omitempty"`
	Source      string `json:"source"`
	Created     int64  `json:"created"`
	ConfirmedAt int64  `json:"confirmed_at"`
}

// Snapshot is a stored playlist state with its tracks and the change since the previous one.
type Snapshot struct {
	SnapshotSummary
	Tracks []Track `json:"tracks"`
	// AddedTracks and RemovedTracks list the change against the previous snapshot.
	AddedTracks   []Track `json:"added_tracks"`
	RemovedTracks []Track `json:"removed_tracks"`
	HasPrevious   bool    `json:"has_previous"`
}

// Store reads and writes playlist_backups and playlist_snapshots.
type Store struct {
	DB  *sql.DB
	now func() time.Time
}

func (s *Store) clock() int64 {
	if s.now != nil {
		return s.now().Unix()
	}
	return time.Now().Unix()
}

const backupColumns = `b.id, b.service, b.playlist_id, COALESCE(b.playlist_name, ''), b.enabled, b.interval_hours,
	b.keep_snapshots, b.last_checked_at, COALESCE(b.last_error, ''), b.created, b.updated`

func scanBackup(row interface{ Scan(...any) error }, extra ...any) (Backup, error) {
	var b Backup
	var enabled int
	var checked sql.NullInt64
	dest := append([]any{&b.ID, &b.Service, &b.PlaylistID, &b.PlaylistName, &enabled, &b.IntervalHours,
		&b.KeepSnapshots, &checked, &b.LastError, &b.Created, &b.Updated}, extra...)
	if err := row.Scan(dest...); err != nil {
		return b, err
	}
	b.Enabled = enabled != 0
	if checked.Valid {
		b.LastCheckedAt = &checked.Int64
	}
	return b, nil
}

// Create adds a playlist to back up.
func (s *Store) Create(ctx context.Context, service, playlistID, name string, intervalHours, keep int) (*Backup, error) {
	if service != "spotify" && service != "youtube" {
		return nil, errors.New("service must be spotify or youtube")
	}
	playlistID = strings.TrimSpace(playlistID)
	if playlistID == "" {
		return nil, errors.New("playlist_id is required")
	}
	if intervalHours <= 0 {
		intervalHours = DefaultIntervalHours
	}
	if keep <= 0 {
		keep = DefaultKeepSnapshots
	}
	existing, err := s.FindByPlaylist(ctx, service, playlistID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, ErrExists
	}
	now := s.clock()
	id := uuid.NewString()
	_, err = s.DB.ExecContext(ctx,
		`INSERT INTO playlist_backups (id, service, playlist_id, playlist_name, enabled, interval_hours, keep_snapshots, created, updated)
		 VALUES (?, ?, ?, ?, 1, ?, ?, ?, ?)`,
		id, service, playlistID, nullable(name), intervalHours, keep, now, now)
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// Get returns a backup with its snapshot count and latest snapshot.
func (s *Store) Get(ctx context.Context, id string) (*Backup, error) {
	list, err := s.list(ctx, `WHERE b.id = ?`, id)
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, ErrNotFound
	}
	return &list[0], nil
}

// FindByPlaylist returns the backup of a playlist, or nil.
func (s *Store) FindByPlaylist(ctx context.Context, service, playlistID string) (*Backup, error) {
	list, err := s.list(ctx, `WHERE b.service = ? AND b.playlist_id = ?`, service, playlistID)
	if err != nil || len(list) == 0 {
		return nil, err
	}
	return &list[0], nil
}

// List returns every backup, Spotify first, then by name.
func (s *Store) List(ctx context.Context) ([]Backup, error) {
	return s.list(ctx, ``)
}

func (s *Store) list(ctx context.Context, where string, args ...any) ([]Backup, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+backupColumns+`,
		(SELECT COUNT(*) FROM playlist_snapshots p WHERE p.backup_id = b.id)
		FROM playlist_backups b `+where+`
		ORDER BY b.service, LOWER(COALESCE(b.playlist_name, b.playlist_id))`, args...)
	if err != nil {
		return nil, err
	}
	var out []Backup
	for rows.Next() {
		var count int
		b, err := scanBackup(rows, &count)
		if err != nil {
			rows.Close()
			return nil, err
		}
		b.SnapshotCount = count
		out = append(out, b)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		latest, err := s.latestSummary(ctx, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Latest = latest
	}
	return out, nil
}

// Update changes a backup's settings. Nil fields are left as they are.
func (s *Store) Update(ctx context.Context, id string, enabled *bool, intervalHours, keep *int) (*Backup, error) {
	b, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if enabled != nil {
		b.Enabled = *enabled
	}
	if intervalHours != nil && *intervalHours > 0 {
		b.IntervalHours = *intervalHours
	}
	if keep != nil && *keep > 0 {
		b.KeepSnapshots = *keep
	}
	_, err = s.DB.ExecContext(ctx,
		`UPDATE playlist_backups SET enabled = ?, interval_hours = ?, keep_snapshots = ?, updated = ? WHERE id = ?`,
		boolInt(b.Enabled), b.IntervalHours, b.KeepSnapshots, s.clock(), id)
	if err != nil {
		return nil, err
	}
	if err := s.prune(ctx, id, b.KeepSnapshots); err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// Delete removes a backup and all of its snapshots.
func (s *Store) Delete(ctx context.Context, id string) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM playlist_backups WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Due returns enabled backups whose interval has elapsed since their last check.
func (s *Store) Due(ctx context.Context) ([]Backup, error) {
	all, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	now := s.clock()
	var due []Backup
	for _, b := range all {
		if !b.Enabled {
			continue
		}
		if b.LastCheckedAt == nil || now-*b.LastCheckedAt >= int64(b.IntervalHours)*3600 {
			due = append(due, b)
		}
	}
	return due, nil
}

// MarkError records a failed check. The backup stays due, so the next tick retries it.
func (s *Store) MarkError(ctx context.Context, id string, cause error) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE playlist_backups SET last_error = ?, updated = ? WHERE id = ?`,
		truncate(cause.Error(), 500), s.clock(), id)
	return err
}

// Record stores the playlist's current tracks for backup b. It returns the snapshot that now
// describes the playlist and whether a new one was created (false: unchanged since the latest).
func (s *Store) Record(ctx context.Context, b Backup, name string, tracks []matching.Track, source string) (*SnapshotSummary, bool, error) {
	links, err := (&tracklinks.Store{DB: s.DB}).LoadIndex(ctx)
	if err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(name) == "" {
		name = b.PlaylistName
	}
	entries := toEntries(b.Service, tracks, links)
	hash := contentHash(name, entries)
	now := s.clock()

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()

	var latestID, latestHash, latestJSON string
	err = tx.QueryRowContext(ctx,
		`SELECT id, content_hash, tracks_json FROM playlist_snapshots WHERE backup_id = ? ORDER BY created DESC, rowid DESC LIMIT 1`,
		b.ID).Scan(&latestID, &latestHash, &latestJSON)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, false, err
	}

	created := false
	if latestID != "" && latestHash == hash {
		if _, err := tx.ExecContext(ctx, `UPDATE playlist_snapshots SET confirmed_at = ? WHERE id = ?`, now, latestID); err != nil {
			return nil, false, err
		}
	} else {
		raw, err := json.Marshal(entries)
		if err != nil {
			return nil, false, err
		}
		var added, removed any
		if latestID != "" {
			var previous []Track
			if err := json.Unmarshal([]byte(latestJSON), &previous); err != nil {
				return nil, false, err
			}
			a, r := diff(previous, entries)
			added, removed = len(a), len(r)
		}
		latestID = uuid.NewString()
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO playlist_snapshots (id, backup_id, playlist_name, track_count, added_count, removed_count, content_hash, source, created, confirmed_at, tracks_json)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			latestID, b.ID, nullable(name), len(entries), added, removed, hash, source, now, now, string(raw)); err != nil {
			return nil, false, err
		}
		created = true
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE playlist_backups SET playlist_name = COALESCE(?, playlist_name), last_checked_at = ?, last_error = NULL, updated = ? WHERE id = ?`,
		nullable(name), now, now, b.ID); err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	if created {
		if err := s.prune(ctx, b.ID, b.KeepSnapshots); err != nil {
			return nil, false, err
		}
	}
	summary, err := s.summary(ctx, latestID)
	return summary, created, err
}

// Observe records tracks that the sync engine already fetched, when the playlist is backed
// up. It costs no API calls.
func (s *Store) Observe(ctx context.Context, service, playlistID, name string, tracks []matching.Track) (bool, error) {
	b, err := s.FindByPlaylist(ctx, service, playlistID)
	if err != nil || b == nil || !b.Enabled {
		return false, err
	}
	_, created, err := s.Record(ctx, *b, name, tracks, SourceSync)
	return created, err
}

// prune keeps the newest keep snapshots of a backup.
func (s *Store) prune(ctx context.Context, backupID string, keep int) error {
	if keep <= 0 {
		return nil
	}
	_, err := s.DB.ExecContext(ctx,
		`DELETE FROM playlist_snapshots WHERE backup_id = ? AND id NOT IN (
			SELECT id FROM playlist_snapshots WHERE backup_id = ? ORDER BY created DESC, rowid DESC LIMIT ?)`,
		backupID, backupID, keep)
	return err
}

const summaryColumns = `id, backup_id, COALESCE(playlist_name, ''), track_count, added_count, removed_count, source, created, confirmed_at`

func scanSummary(row interface{ Scan(...any) error }, extra ...any) (SnapshotSummary, error) {
	var s SnapshotSummary
	var added, removed sql.NullInt64
	dest := append([]any{&s.ID, &s.BackupID, &s.PlaylistName, &s.TrackCount, &added, &removed, &s.Source, &s.Created, &s.ConfirmedAt}, extra...)
	if err := row.Scan(dest...); err != nil {
		return s, err
	}
	if added.Valid {
		n := int(added.Int64)
		s.Added = &n
	}
	if removed.Valid {
		n := int(removed.Int64)
		s.Removed = &n
	}
	return s, nil
}

func (s *Store) summary(ctx context.Context, id string) (*SnapshotSummary, error) {
	sum, err := scanSummary(s.DB.QueryRowContext(ctx, `SELECT `+summaryColumns+` FROM playlist_snapshots WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &sum, nil
}

func (s *Store) latestSummary(ctx context.Context, backupID string) (*SnapshotSummary, error) {
	sum, err := scanSummary(s.DB.QueryRowContext(ctx,
		`SELECT `+summaryColumns+` FROM playlist_snapshots WHERE backup_id = ? ORDER BY created DESC, rowid DESC LIMIT 1`, backupID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sum, nil
}

// Snapshots lists a backup's snapshots, newest first.
func (s *Store) Snapshots(ctx context.Context, backupID string) ([]SnapshotSummary, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT `+summaryColumns+` FROM playlist_snapshots WHERE backup_id = ? ORDER BY created DESC, rowid DESC`, backupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SnapshotSummary{}
	for rows.Next() {
		sum, err := scanSummary(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, sum)
	}
	return out, rows.Err()
}

// Snapshot returns one snapshot with its tracks and the change since the previous snapshot.
func (s *Store) Snapshot(ctx context.Context, id string) (*Snapshot, error) {
	var raw string
	sum, err := scanSummary(s.DB.QueryRowContext(ctx,
		`SELECT `+summaryColumns+`, tracks_json FROM playlist_snapshots WHERE id = ?`, id), &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	snap := &Snapshot{SnapshotSummary: sum, AddedTracks: []Track{}, RemovedTracks: []Track{}}
	if err := json.Unmarshal([]byte(raw), &snap.Tracks); err != nil {
		return nil, err
	}
	var prevRaw string
	err = s.DB.QueryRowContext(ctx,
		`SELECT tracks_json FROM playlist_snapshots WHERE backup_id = ? AND (created < ? OR (created = ? AND rowid < (SELECT rowid FROM playlist_snapshots WHERE id = ?)))
		 ORDER BY created DESC, rowid DESC LIMIT 1`,
		sum.BackupID, sum.Created, sum.Created, id).Scan(&prevRaw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if prevRaw != "" {
		var previous []Track
		if err := json.Unmarshal([]byte(prevRaw), &previous); err != nil {
			return nil, err
		}
		snap.HasPrevious = true
		snap.AddedTracks, snap.RemovedTracks = diff(previous, snap.Tracks)
	}
	return snap, nil
}

// toEntries converts fetched tracks to snapshot entries, adding each track's linked
// counterpart on the other service.
func toEntries(service string, tracks []matching.Track, links *tracklinks.Index) []Track {
	out := make([]Track, 0, len(tracks))
	for i, t := range tracks {
		e := Track{Position: i, ID: t.ID, Title: t.Title, Artist: t.Artist, DurationMs: t.DurationMs}
		if service == "youtube" {
			e.RawTitle, e.Channel, e.YouTubeID = t.RawTitle, t.Channel, t.ID
		} else {
			e.SpotifyID = t.ID
		}
		if link, ok := links.For(service, t.ID); ok {
			if service == "youtube" {
				e.SpotifyID = link.SpotifyID
			} else {
				e.YouTubeID = link.YouTubeID
			}
		}
		out = append(out, e)
	}
	return out
}

// contentHash identifies a playlist state: its name and ordered track IDs.
func contentHash(name string, tracks []Track) string {
	h := sha256.New()
	h.Write([]byte(name))
	for _, t := range tracks {
		h.Write([]byte{0})
		h.Write([]byte(t.ID))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// diff returns tracks in next but not previous (added) and in previous but not next (removed),
// counting repeated IDs.
func diff(previous, next []Track) (added, removed []Track) {
	counts := map[string]int{}
	for _, t := range previous {
		counts[t.ID]++
	}
	added = []Track{}
	for _, t := range next {
		if counts[t.ID] > 0 {
			counts[t.ID]--
			continue
		}
		added = append(added, t)
	}
	counts = map[string]int{}
	for _, t := range next {
		counts[t.ID]++
	}
	removed = []Track{}
	for _, t := range previous {
		if counts[t.ID] > 0 {
			counts[t.ID]--
			continue
		}
		removed = append(removed, t)
	}
	return added, removed
}

func nullable(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
