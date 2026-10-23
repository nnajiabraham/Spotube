package backups

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"time"
)

// Export is the downloadable form of a snapshot.
type Export struct {
	Spotube      string  `json:"spotube_backup"`
	Service      string  `json:"service"`
	PlaylistID   string  `json:"playlist_id"`
	PlaylistName string  `json:"playlist_name"`
	PlaylistURL  string  `json:"playlist_url"`
	TakenAt      string  `json:"taken_at"`
	ConfirmedAt  string  `json:"confirmed_at"`
	TrackCount   int     `json:"track_count"`
	Tracks       []Track `json:"tracks"`
}

// NewExport describes snap of backup b for download.
func NewExport(b Backup, snap Snapshot) Export {
	name := snap.PlaylistName
	if name == "" {
		name = b.PlaylistName
	}
	return Export{
		Spotube:      "1",
		Service:      b.Service,
		PlaylistID:   b.PlaylistID,
		PlaylistName: name,
		PlaylistURL:  PlaylistURL(b.Service, b.PlaylistID),
		TakenAt:      time.Unix(snap.Created, 0).UTC().Format(time.RFC3339),
		ConfirmedAt:  time.Unix(snap.ConfirmedAt, 0).UTC().Format(time.RFC3339),
		TrackCount:   len(snap.Tracks),
		Tracks:       snap.Tracks,
	}
}

// CSV renders the tracks with one row each.
func (e Export) CSV() ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write([]string{"position", "title", "artist", "duration_ms", "spotify_id", "youtube_id", "youtube_title", "youtube_channel"})
	for _, t := range e.Tracks {
		_ = w.Write([]string{
			strconv.Itoa(t.Position + 1), t.Title, t.Artist, strconv.Itoa(t.DurationMs),
			t.SpotifyID, t.YouTubeID, t.RawTitle, t.Channel,
		})
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}
