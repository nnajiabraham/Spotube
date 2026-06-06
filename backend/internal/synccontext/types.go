package synccontext

import "github.com/manlikeabro/spotube/internal/matching"

const Version = 1

// Envelope wraps versioned JSON stored in activity_logs.details_json.
type Envelope struct {
	Version int    `json:"version"`
	Kind    string `json:"kind"`
	Data    any    `json:"data"`
}

// AnalysisRun summarizes a mapping analysis pass.
type AnalysisRun struct {
	MappingID      string `json:"mapping_id"`
	SpotifyCount   int    `json:"spotify_count"`
	YoutubeCount   int    `json:"youtube_count"`
	QueuedAdds     int    `json:"queued_adds"`
	MatchedSkipped int    `json:"matched_skipped"`
	AlreadyQueued  int    `json:"already_queued"`
}

// SearchCandidate is a ranked destination search hit.
type SearchCandidate struct {
	Rank    int     `json:"rank"`
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Artist  string  `json:"artist,omitempty"`
	Channel string  `json:"channel,omitempty"`
	Score   float64 `json:"score"`
}

// ExecutorRun captures executor search, selection, and outcome for one sync item.
type ExecutorRun struct {
	SyncItemID        string             `json:"sync_item_id"`
	Operation         string             `json:"operation"`
	Source            matching.Track     `json:"source"`
	SearchQuery       string             `json:"search_query"`
	Candidates        []SearchCandidate  `json:"candidates,omitempty"`
	Selected          *matching.Track    `json:"selected,omitempty"`
	DestPlaylistID    string             `json:"dest_playlist_id"`
	AlreadyInPlaylist bool               `json:"already_in_playlist"`
	PlaylistMatch     *matching.Decision `json:"playlist_match,omitempty"`
	Outcome           string             `json:"outcome"`
	Error             string             `json:"error,omitempty"`
}
