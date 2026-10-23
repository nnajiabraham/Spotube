package schedule

import (
	"fmt"
	"time"
)

// Existing is a mapping already scheduled, for load and spacing.
type Existing struct {
	Schedule      Schedule
	YouTubeVideos int
}

// Recommendation is a suggested schedule with the reasoning behind it.
type Recommendation struct {
	Schedule Schedule `json:"schedule"`
	// Daily YouTube read units of all mappings before and after adding this one.
	ReadsBefore int    `json:"reads_before"`
	ReadsAfter  int    `json:"reads_after"`
	Reason      string `json:"reason"`
	Warning     string `json:"warning,omitempty"`
}

// readShare is the part of the daily quota scheduled reads may take before the
// recommendation slows the new mapping down; the rest is for copying tracks.
const readShare = 0.15

// intervalSteps are the intervals the recommendation moves through, fastest first.
var intervalSteps = []int{30, 60, 120, 360, 720, 1440}

// sizeInterval is the starting interval for a playlist of n songs (the larger side).
func sizeInterval(n int) int {
	switch {
	case n <= 300:
		return 30
	case n <= 1500:
		return 120
	case n <= 5000:
		return 360
	}
	return 1440
}

// Recommend suggests a schedule for a new mapping of spotifyTracks / youtubeVideos songs,
// given the mappings that exist, the daily quota and the user's time zone.
func Recommend(spotifyTracks, youtubeVideos int, existing []Existing, dailyLimit int, tz string, now time.Time) Recommendation {
	before := 0
	for _, e := range existing {
		before += DailyReadUnits(e.Schedule, e.YouTubeVideos)
	}
	size := max(spotifyTracks, youtubeVideos)
	interval := sizeInterval(size)
	reason := fmt.Sprintf("%s for a playlist of %d songs", every(interval), size)

	// The new playlist's YouTube side: what exists, or what the copy will create.
	videos := max(youtubeVideos, spotifyTracks)
	budget := int(float64(dailyLimit) * readShare)
	sched := Schedule{IntervalMinutes: interval, Days: EveryDay, TZ: tz}
	after := before + DailyReadUnits(withStart(sched, "00:00"), videos)
	for after > budget && sched.IntervalMinutes < MaxInterval {
		sched.IntervalMinutes = nextStep(sched.IntervalMinutes)
		after = before + DailyReadUnits(withStart(sched, "00:00"), videos)
		reason = fmt.Sprintf("%s so scheduled reads stay under %d%% of the daily quota", every(sched.IntervalMinutes), int(readShare*100))
	}
	rec := Recommendation{ReadsBefore: before, ReadsAfter: after, Reason: reason}
	if after > budget {
		rec.Warning = fmt.Sprintf("Scheduled reads would use about %d of %d daily units even once a day; consider fewer mappings or a higher quota.", after, dailyLimit)
	}
	sched.Start = bestStart(sched, existing, now)
	rec.Schedule = sched
	return rec
}

func withStart(s Schedule, start string) Schedule { s.Start = start; return s }

func nextStep(interval int) int {
	for _, s := range intervalSteps {
		if s > interval {
			return s
		}
	}
	return MaxInterval
}

func every(interval int) string {
	switch {
	case interval == 1440:
		return "Once a day"
	case interval%60 == 0 && interval > 60:
		return fmt.Sprintf("Every %d hours", interval/60)
	case interval == 60:
		return "Every hour"
	}
	return fmt.Sprintf("Every %d minutes", interval)
}

// bestStart picks the start time whose runs are furthest from every other mapping's runs
// (maximising the smallest gap). Daily schedules prefer the night when gaps tie.
func bestStart(s Schedule, existing []Existing, now time.Time) string {
	var busy [1440]bool
	found := false
	for _, e := range existing {
		for _, m := range e.Schedule.utcRunMinutes(now) {
			busy[m] = true
			found = true
		}
	}
	first := 0
	if s.IntervalMinutes >= MaxInterval {
		first = 2 * 60 // quiet hours first
	}
	if !found {
		return FormatStart(first)
	}
	dist := distances(busy)
	span := min(s.IntervalMinutes, 1440)
	best, bestScore := first, -1
	for i := 0; i < span; i += 5 {
		start := (first + i) % 1440
		cand := s
		cand.Start = FormatStart(start)
		score := 1440
		for _, m := range cand.utcRunMinutes(now) {
			score = min(score, dist[m])
		}
		if score > bestScore {
			best, bestScore = start, score
		}
	}
	return FormatStart(best)
}

// distances gives, for each minute of the day, the circular distance to the nearest busy one.
func distances(busy [1440]bool) [1440]int {
	var d [1440]int
	for i := range d {
		d[i] = 1440
	}
	for pass := 0; pass < 2; pass++ { // two sweeps each way handle wrap-around
		for i := 0; i < 1440; i++ {
			if busy[i] {
				d[i] = 0
			} else if p := d[(i+1439)%1440] + 1; p < d[i] {
				d[i] = p
			}
		}
		for i := 1439; i >= 0; i-- {
			if n := d[(i+1)%1440] + 1; n < d[i] {
				d[i] = n
			}
		}
	}
	return d
}
