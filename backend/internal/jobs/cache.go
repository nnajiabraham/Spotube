package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/manlikeabro/spotube/internal/matching"
)

// PlaylistCache holds the latest full read of each synced playlist, shared by analysis and
// every apply pass, so the queue never re-lists a destination per track. Tracks Spotube adds
// are appended as it goes. Entries are written to playlist_cache, so a restart (or a probe
// that finds nothing changed) does not pay for another YouTube read.
type PlaylistCache struct {
	db      *sql.DB
	mu      sync.Mutex
	entries map[string]*cacheEntry
}

type cacheEntry struct {
	tracks []matching.Track
	readAt time.Time
	dirty  bool
}

// NewPlaylistCache returns a cache persisted to db (memory only when db is nil).
func NewPlaylistCache(db *sql.DB) *PlaylistCache {
	return &PlaylistCache{db: db, entries: map[string]*cacheEntry{}}
}

func cacheKey(service, playlistID string) string { return service + "|" + playlistID }

// Get returns a copy of the cached tracks and when they were read.
func (c *PlaylistCache) Get(ctx context.Context, service, playlistID string) ([]matching.Track, time.Time, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[cacheKey(service, playlistID)]
	if !ok && c.db != nil {
		var raw string
		var readAt int64
		err := c.db.QueryRowContext(ctx,
			`SELECT tracks_json, read_at FROM playlist_cache WHERE service = ? AND playlist_id = ?`,
			service, playlistID).Scan(&raw, &readAt)
		if err == nil {
			var tracks []matching.Track
			if json.Unmarshal([]byte(raw), &tracks) == nil {
				e = &cacheEntry{tracks: tracks, readAt: time.Unix(readAt, 0)}
				c.entries[cacheKey(service, playlistID)] = e
				ok = true
			}
		}
	}
	if !ok {
		return nil, time.Time{}, false
	}
	return append([]matching.Track(nil), e.tracks...), e.readAt, true
}

// Put stores a fresh full read.
func (c *PlaylistCache) Put(ctx context.Context, service, playlistID string, tracks []matching.Track) {
	c.mu.Lock()
	e := &cacheEntry{tracks: append([]matching.Track(nil), tracks...), readAt: time.Now(), dirty: true}
	c.entries[cacheKey(service, playlistID)] = e
	c.mu.Unlock()
	c.flushOne(ctx, service, playlistID)
}

// Append records a track Spotube added, if the playlist is cached.
func (c *PlaylistCache) Append(service, playlistID string, track matching.Track) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.entries[cacheKey(service, playlistID)]; ok {
		e.tracks = append(e.tracks, track)
		e.dirty = true
	}
}

// Drop forgets a playlist (it changed, or its mapping was deleted).
func (c *PlaylistCache) Drop(ctx context.Context, service, playlistID string) {
	c.mu.Lock()
	delete(c.entries, cacheKey(service, playlistID))
	c.mu.Unlock()
	if c.db != nil {
		_, _ = c.db.ExecContext(ctx, `DELETE FROM playlist_cache WHERE service = ? AND playlist_id = ?`, service, playlistID)
	}
}

// Flush writes every changed entry to the database.
func (c *PlaylistCache) Flush(ctx context.Context) {
	c.mu.Lock()
	var dirty [][2]string
	for k, e := range c.entries {
		if e.dirty {
			service, id, _ := strings.Cut(k, "|")
			dirty = append(dirty, [2]string{service, id})
		}
	}
	c.mu.Unlock()
	for _, d := range dirty {
		c.flushOne(ctx, d[0], d[1])
	}
}

func (c *PlaylistCache) flushOne(ctx context.Context, service, playlistID string) {
	c.mu.Lock()
	e, ok := c.entries[cacheKey(service, playlistID)]
	if !ok || !e.dirty || c.db == nil {
		c.mu.Unlock()
		return
	}
	raw, err := json.Marshal(e.tracks)
	count, readAt := len(e.tracks), e.readAt.Unix()
	e.dirty = false
	c.mu.Unlock()
	if err != nil {
		return
	}
	_, _ = c.db.ExecContext(ctx,
		`INSERT INTO playlist_cache (service, playlist_id, tracks_json, item_count, read_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(service, playlist_id) DO UPDATE SET tracks_json = excluded.tracks_json,
		   item_count = excluded.item_count, read_at = excluded.read_at`,
		service, playlistID, string(raw), count, readAt)
}
