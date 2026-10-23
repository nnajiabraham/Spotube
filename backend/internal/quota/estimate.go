package quota

import (
	"context"
	"database/sql"
	"math"
)

// YouTube costs used in estimates.
const (
	SearchCost   = 101 // search.list + videos.list for durations
	InsertCost   = 50
	PlaylistCost = 50
)

// FullReadUnits is the cost of reading n videos (list pages plus durations for about half).
func FullReadUnits(n int) int {
	if n <= 0 {
		return 0
	}
	pages := int(math.Ceil(float64(n) / 50))
	return pages + (pages+1)/2
}

// Work is a job to estimate: what would be copied and read.
type Work struct {
	// ToYouTube are songs with no remembered YouTube video: a search and an insert each.
	ToYouTube int `json:"to_youtube"`
	// ToYouTubeLinked are songs Spotube already matched: an insert each.
	ToYouTubeLinked int `json:"to_youtube_linked"`
	// ToSpotify are songs to add to Spotify (no YouTube quota).
	ToSpotify int `json:"to_spotify"`
	// YouTubeReadVideos is the size of YouTube playlists to read.
	YouTubeReadVideos int `json:"youtube_read_videos"`
	// YouTubePlaylists is how many YouTube playlists to create.
	YouTubePlaylists int `json:"youtube_playlists"`
}

// Add sums two jobs.
func (w Work) Add(o Work) Work {
	return Work{
		ToYouTube:         w.ToYouTube + o.ToYouTube,
		ToYouTubeLinked:   w.ToYouTubeLinked + o.ToYouTubeLinked,
		ToSpotify:         w.ToSpotify + o.ToSpotify,
		YouTubeReadVideos: w.YouTubeReadVideos + o.YouTubeReadVideos,
		YouTubePlaylists:  w.YouTubePlaylists + o.YouTubePlaylists,
	}
}

// Estimate is the YouTube cost of a job and how long the queue takes to finish it.
type Estimate struct {
	Units       int `json:"units"`
	SearchUnits int `json:"search_units"`
	InsertUnits int `json:"insert_units"`
	ReadUnits   int `json:"read_units"`
	CreateUnits int `json:"create_units"`
	// BacklogUnits is queued work ahead of this job.
	BacklogUnits int `json:"backlog_units"`
	// DailyCapacity is what the queue gets on a full day (limit - reserve - scheduled reads).
	DailyCapacity int `json:"daily_capacity"`
	// Days until the job is done: 0 = today, -1 = never (no capacity left for the queue).
	Days int `json:"days"`
}

// Units returns w's YouTube cost.
func (w Work) Units() (search, insert, read, create int) {
	return w.ToYouTube * SearchCost, (w.ToYouTube + w.ToYouTubeLinked) * InsertCost,
		FullReadUnits(w.YouTubeReadVideos), w.YouTubePlaylists * PlaylistCost
}

// DailyCapacity is the units the queue gets on a full day.
func DailyCapacity(s Status) int {
	return s.Limit - s.Reserve - s.DailyReads
}

// DaysToDrain is how many quota resets pass before units of queued work are done, given what
// is left for the queue today and a full day's capacity. -1 means never.
func DaysToDrain(units, todayLeft, capacity int) int {
	if units <= todayLeft {
		return 0
	}
	if capacity <= 0 {
		return -1
	}
	return int(math.Ceil(float64(units-max(todayLeft, 0)) / float64(capacity)))
}

// EstimateWork prices w behind backlogUnits of queued work.
func EstimateWork(w Work, s Status, backlogUnits int) Estimate {
	search, insert, read, create := w.Units()
	e := Estimate{
		SearchUnits:   search,
		InsertUnits:   insert,
		ReadUnits:     read,
		CreateUnits:   create,
		Units:         search + insert + read + create,
		BacklogUnits:  backlogUnits,
		DailyCapacity: DailyCapacity(s),
	}
	e.Days = DaysToDrain(backlogUnits+e.Units, s.BackgroundLeft, e.DailyCapacity)
	return e
}

// Backlog is the queued YouTube work, and its cost.
type Backlog struct {
	YouTubeUnlinked int `json:"youtube_unlinked"`
	YouTubeLinked   int `json:"youtube_linked"`
	YouTubeRenames  int `json:"youtube_renames"`
	Spotify         int `json:"spotify"`
	Units           int `json:"units"`
}

// LoadBacklog counts applicable queued items (pending, or failed under the retry cap).
func LoadBacklog(ctx context.Context, db *sql.DB) (Backlog, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT si.service, si.operation,
		       EXISTS (SELECT 1 FROM track_links tl WHERE si.service = 'youtube' AND tl.spotify_id = si.track_id AND tl.youtube_id IS NOT NULL),
		       COUNT(*)
		FROM sync_items si
		WHERE si.status = 'pending' OR (si.status = 'error' AND si.attempt_count < 3)
		GROUP BY 1, 2, 3`)
	if err != nil {
		return Backlog{}, err
	}
	defer rows.Close()
	var b Backlog
	for rows.Next() {
		var service, op string
		var linked bool
		var n int
		if err := rows.Scan(&service, &op, &linked, &n); err != nil {
			return Backlog{}, err
		}
		switch {
		case service == "spotify":
			b.Spotify += n
		case op == "rename":
			b.YouTubeRenames += n
		case linked:
			b.YouTubeLinked += n
		default:
			b.YouTubeUnlinked += n
		}
	}
	b.Units = b.YouTubeUnlinked*(SearchCost+InsertCost) + b.YouTubeLinked*InsertCost + b.YouTubeRenames*PlaylistCost
	return b, rows.Err()
}
