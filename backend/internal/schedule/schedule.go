// Package schedule computes when a mapping syncs and recommends schedules that keep YouTube
// reads affordable and spread mappings apart.
//
// One model covers every case: run every IntervalMinutes starting at Start (HH:MM, in TZ) on
// the days in Days, until midnight. "Once a day at 03:00 on weekdays" is interval 1440, start
// 03:00, days Mon–Fri. Each day starts again at Start, so intervals that don't divide a day
// still behave predictably.
package schedule

import (
	"errors"
	"fmt"
	"math"
	"math/bits"
	"strconv"
	"strings"
	"time"

	"github.com/manlikeabro/spotube/internal/quota"
)

// Day bits, Sunday = 1 ... Saturday = 64.
const (
	EveryDay = 127
	Weekdays = 0b0111110
	Weekends = 0b1000001
)

const (
	MinInterval = 5
	MaxInterval = 1440
)

// Schedule says when a mapping runs.
type Schedule struct {
	IntervalMinutes int    `json:"interval_minutes"`
	Start           string `json:"schedule_start"` // HH:MM
	Days            int    `json:"schedule_days"`
	TZ              string `json:"schedule_tz,omitempty"`
}

// Default is the schedule of mappings created before schedules existed.
func Default(interval int) Schedule {
	return Schedule{IntervalMinutes: interval, Start: "00:00", Days: EveryDay}
}

// ParseStart parses HH:MM into minutes after midnight.
func ParseStart(s string) (int, error) {
	h, m, ok := strings.Cut(strings.TrimSpace(s), ":")
	if !ok {
		return 0, errors.New("start must be HH:MM")
	}
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if err1 != nil || err2 != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 || len(m) != 2 {
		return 0, errors.New("start must be HH:MM")
	}
	return hh*60 + mm, nil
}

// FormatStart formats minutes after midnight as HH:MM.
func FormatStart(minutes int) string {
	minutes = ((minutes % 1440) + 1440) % 1440
	return fmt.Sprintf("%02d:%02d", minutes/60, minutes%60)
}

// Validate checks the fields.
func (s Schedule) Validate() error {
	if s.IntervalMinutes < MinInterval || s.IntervalMinutes > MaxInterval {
		return fmt.Errorf("interval must be %d-%d minutes", MinInterval, MaxInterval)
	}
	if _, err := ParseStart(s.Start); err != nil {
		return err
	}
	if s.Days < 1 || s.Days > EveryDay {
		return errors.New("pick at least one day")
	}
	if s.TZ != "" {
		if _, err := time.LoadLocation(s.TZ); err != nil {
			return fmt.Errorf("unknown time zone %q", s.TZ)
		}
	}
	return nil
}

// Location is the schedule's time zone (the server's when unset).
func (s Schedule) Location() *time.Location {
	if s.TZ != "" {
		if loc, err := time.LoadLocation(s.TZ); err == nil {
			return loc
		}
	}
	return time.Local
}

// On reports whether the schedule runs on weekday d.
func (s Schedule) On(d time.Weekday) bool { return s.Days&(1<<uint(d)) != 0 }

func (s Schedule) startMinutes() int {
	m, _ := ParseStart(s.Start)
	return m
}

func (s Schedule) interval() time.Duration {
	n := s.IntervalMinutes
	if n < MinInterval {
		n = 60
	}
	return time.Duration(n) * time.Minute
}

// Next returns the first run strictly after t. A mapping that missed runs (the server was
// down) is simply due; the scheduler then asks for the next run after now, so missed runs
// are never replayed one by one.
func (s Schedule) Next(after time.Time) time.Time {
	loc := s.Location()
	local := after.In(loc)
	start := s.startMinutes()
	step := s.interval()
	for d := 0; d <= 8; d++ {
		day := time.Date(local.Year(), local.Month(), local.Day()+d, 0, 0, 0, 0, loc)
		if !s.On(day.Weekday()) {
			continue
		}
		first := time.Date(day.Year(), day.Month(), day.Day(), start/60, start%60, 0, 0, loc)
		// A start inside a spring-forward gap (02:30 when clocks jump 02:00 -> 03:00) comes
		// back from time.Date an hour early; move it after the gap (03:30).
		if got := first.Hour()*60 + first.Minute(); got < start && first.Day() == day.Day() {
			first = first.Add(time.Duration(start-got) * time.Minute)
		}
		end := time.Date(day.Year(), day.Month(), day.Day()+1, 0, 0, 0, 0, loc)
		if after.Before(first) {
			return first
		}
		k := after.Sub(first)/step + 1
		if next := first.Add(k * step); next.Before(end) {
			return next
		}
	}
	return after.Add(24 * time.Hour) // unreachable while Days is valid
}

// RunsPerDay is the average number of runs per day across the week.
func (s Schedule) RunsPerDay() float64 {
	perDay := len(s.runMinutes())
	return float64(perDay) * float64(bits.OnesCount(uint(s.Days&EveryDay))) / 7
}

// runMinutes lists the wall-clock run times of one enabled day, in minutes after midnight.
func (s Schedule) runMinutes() []int {
	n := s.IntervalMinutes
	if n < MinInterval {
		n = 60
	}
	var out []int
	for m := s.startMinutes(); m < 1440; m += n {
		out = append(out, m)
	}
	return out
}

// utcRunMinutes is runMinutes shifted to UTC minutes of the day (using today's offset), so
// mappings in different zones can be compared.
func (s Schedule) utcRunMinutes(now time.Time) []int {
	_, offset := now.In(s.Location()).Zone()
	shift := offset / 60
	out := s.runMinutes()
	for i, m := range out {
		out[i] = ((m-shift)%1440 + 1440) % 1440
	}
	return out
}

// FullReadUnits is the YouTube cost of reading a playlist of videos in full: 1 unit per 50
// items, plus durations for the (roughly half) videos with no remembered link.
func FullReadUnits(videos int) int {
	return max(1, quota.FullReadUnits(videos))
}

// FullReadEvery is how stale a YouTube playlist read may get before an unchanged probe still
// forces a fresh read (catches a swap that keeps the count). It grows with size: a day per
// 1,000 videos, at most a week.
func FullReadEvery(videos int) time.Duration {
	days := min(max(1, videos/1000), 7)
	return time.Duration(days) * 24 * time.Hour
}

// DailyReadUnits estimates the YouTube units a mapping's scheduled syncs use per day: a
// 1-unit probe per run plus about one full read a day when the playlist changes (Spotube's
// own inserts don't count as changes).
func DailyReadUnits(s Schedule, youtubeVideos int) int {
	runs := s.RunsPerDay()
	if runs == 0 {
		return 0
	}
	return int(math.Ceil(runs + float64(FullReadUnits(youtubeVideos))*min(runs, 1)))
}
