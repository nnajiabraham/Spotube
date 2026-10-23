// Package tracklinks remembers which Spotify track and YouTube video are the same song, and
// which songs are known not to exist on a service. Links are global: a song's counterpart
// does not depend on which playlist pair asks.
package tracklinks

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	OriginAuto   = "auto"
	OriginManual = "manual"
)

// Link pairs a Spotify track with a YouTube video. An empty side means the song is known
// not to exist on that service.
type Link struct {
	ID        string   `json:"id"`
	SpotifyID string   `json:"spotify_id,omitempty"`
	YouTubeID string   `json:"youtube_id,omitempty"`
	Title     string   `json:"title,omitempty"`
	Artist    string   `json:"artist,omitempty"`
	Origin    string   `json:"origin"`
	Score     *float64 `json:"score,omitempty"`
	Created   int64    `json:"created"`
	Updated   int64    `json:"updated"`
}

// Counterpart returns the linked ID on service ("spotify" or "youtube"); missing is true when
// the link records that the song is not available there.
func (l Link) Counterpart(service string) (id string, missing bool) {
	if service == "spotify" {
		return l.SpotifyID, l.SpotifyID == ""
	}
	return l.YouTubeID, l.YouTubeID == ""
}

// Store reads and writes track_links.
type Store struct {
	DB *sql.DB
}

const columns = `id, COALESCE(spotify_id, ''), COALESCE(youtube_id, ''), COALESCE(title, ''), COALESCE(artist, ''), origin, score, created, updated`

func scan(row interface{ Scan(...any) error }) (Link, error) {
	var l Link
	var score sql.NullFloat64
	err := row.Scan(&l.ID, &l.SpotifyID, &l.YouTubeID, &l.Title, &l.Artist, &l.Origin, &score, &l.Created, &l.Updated)
	if score.Valid {
		l.Score = &score.Float64
	}
	return l, err
}

func column(service string) (string, error) {
	switch service {
	case "spotify":
		return "spotify_id", nil
	case "youtube":
		return "youtube_id", nil
	}
	return "", errors.New("unknown service: " + service)
}

// Find returns the link for a track on service, or nil.
func (s *Store) Find(ctx context.Context, service, id string) (*Link, error) {
	col, err := column(service)
	if err != nil {
		return nil, err
	}
	l, err := scan(s.DB.QueryRowContext(ctx, `SELECT `+columns+` FROM track_links WHERE `+col+` = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// Get returns a link by its row ID, or nil.
func (s *Store) Get(ctx context.Context, id string) (*Link, error) {
	l, err := scan(s.DB.QueryRowContext(ctx, `SELECT `+columns+` FROM track_links WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// LinkAuto records a pair found by the sync engine. It never overrides an existing link on
// either side, so a user's choice always wins.
func (s *Store) LinkAuto(ctx context.Context, spotifyID, youtubeID, title, artist string, score float64) error {
	if spotifyID == "" || youtubeID == "" {
		return nil
	}
	now := time.Now().Unix()
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO track_links (id, spotify_id, youtube_id, title, artist, origin, score, created, updated)
		 VALUES (?, ?, ?, ?, ?, 'auto', ?, ?, ?) ON CONFLICT DO NOTHING`,
		uuid.NewString(), spotifyID, youtubeID, nullable(title), nullable(artist), score, now, now)
	return err
}

// Set records a user's decision, replacing whatever was linked to either ID before. An empty
// spotifyID or youtubeID marks the song as not available on that service.
func (s *Store) Set(ctx context.Context, spotifyID, youtubeID, title, artist string) (*Link, error) {
	if spotifyID == "" && youtubeID == "" {
		return nil, errors.New("a link needs at least one track")
	}
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if spotifyID != "" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM track_links WHERE spotify_id = ?`, spotifyID); err != nil {
			return nil, err
		}
	}
	if youtubeID != "" {
		if _, err := tx.ExecContext(ctx, `DELETE FROM track_links WHERE youtube_id = ?`, youtubeID); err != nil {
			return nil, err
		}
	}
	now := time.Now().Unix()
	l := Link{
		ID: uuid.NewString(), SpotifyID: spotifyID, YouTubeID: youtubeID,
		Title: strings.TrimSpace(title), Artist: strings.TrimSpace(artist),
		Origin: OriginManual, Created: now, Updated: now,
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO track_links (id, spotify_id, youtube_id, title, artist, origin, score, created, updated)
		 VALUES (?, ?, ?, ?, ?, 'manual', NULL, ?, ?)`,
		l.ID, nullable(spotifyID), nullable(youtubeID), nullable(l.Title), nullable(l.Artist), now, now); err != nil {
		return nil, err
	}
	return &l, tx.Commit()
}

// Delete removes a link so the engine decides again on the next sync.
func (s *Store) Delete(ctx context.Context, id string) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM track_links WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// Filter values for List.
const (
	FilterLinked         = "linked"
	FilterMissingSpotify = "missing_spotify"
	FilterMissingYouTube = "missing_youtube"
)

// List returns links newest first, optionally filtered and text-searched.
func (s *Store) List(ctx context.Context, filter, query string, page, perPage int) ([]Link, int, error) {
	where := []string{"1=1"}
	var args []any
	switch filter {
	case FilterLinked:
		where = append(where, "spotify_id IS NOT NULL AND youtube_id IS NOT NULL")
	case FilterMissingSpotify:
		where = append(where, "spotify_id IS NULL")
	case FilterMissingYouTube:
		where = append(where, "youtube_id IS NULL")
	}
	if q := strings.TrimSpace(query); q != "" {
		where = append(where, "(title LIKE ? OR artist LIKE ? OR spotify_id = ? OR youtube_id = ?)")
		like := "%" + q + "%"
		args = append(args, like, like, q, q)
	}
	cond := strings.Join(where, " AND ")

	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM track_links WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.DB.QueryContext(ctx,
		`SELECT `+columns+` FROM track_links WHERE `+cond+` ORDER BY updated DESC, id LIMIT ? OFFSET ?`,
		append(args, perPage, (page-1)*perPage)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	links := []Link{}
	for rows.Next() {
		l, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		links = append(links, l)
	}
	return links, total, rows.Err()
}

// Index is an in-memory view of all links for one analysis pass.
type Index struct {
	bySpotify map[string]Link
	byYouTube map[string]Link
}

// LoadIndex reads every link.
func (s *Store) LoadIndex(ctx context.Context) (*Index, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+columns+` FROM track_links`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ix := &Index{bySpotify: map[string]Link{}, byYouTube: map[string]Link{}}
	for rows.Next() {
		l, err := scan(rows)
		if err != nil {
			return nil, err
		}
		if l.SpotifyID != "" {
			ix.bySpotify[l.SpotifyID] = l
		}
		if l.YouTubeID != "" {
			ix.byYouTube[l.YouTubeID] = l
		}
	}
	return ix, rows.Err()
}

// For returns the link for a track on service. A nil Index has no links.
func (ix *Index) For(service, id string) (Link, bool) {
	if ix == nil {
		return Link{}, false
	}
	var l Link
	var ok bool
	if service == "spotify" {
		l, ok = ix.bySpotify[id]
	} else {
		l, ok = ix.byYouTube[id]
	}
	return l, ok
}

func nullable(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
