package schedule

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustLoad(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	require.NoError(t, err)
	return loc
}

func TestValidate(t *testing.T) {
	ok := Schedule{IntervalMinutes: 60, Start: "07:30", Days: Weekdays, TZ: "Europe/London"}
	require.NoError(t, ok.Validate())
	for name, s := range map[string]Schedule{
		"too often":   {IntervalMinutes: 4, Start: "00:00", Days: EveryDay},
		"too rare":    {IntervalMinutes: 1441, Start: "00:00", Days: EveryDay},
		"bad start":   {IntervalMinutes: 60, Start: "7:3", Days: EveryDay},
		"bad hour":    {IntervalMinutes: 60, Start: "24:00", Days: EveryDay},
		"no days":     {IntervalMinutes: 60, Start: "00:00", Days: 0},
		"extra bits":  {IntervalMinutes: 60, Start: "00:00", Days: 128},
		"unknown tz":  {IntervalMinutes: 60, Start: "00:00", Days: EveryDay, TZ: "Mars/Olympus"},
		"no colon":    {IntervalMinutes: 60, Start: "0730", Days: EveryDay},
		"letters":     {IntervalMinutes: 60, Start: "ab:cd", Days: EveryDay},
		"single mins": {IntervalMinutes: 60, Start: "07:3", Days: EveryDay},
	} {
		assert.Error(t, s.Validate(), name)
	}
}

func TestNext_IntervalFromStartWithinTheDay(t *testing.T) {
	ny := mustLoad(t, "America/New_York")
	s := Schedule{IntervalMinutes: 90, Start: "08:00", Days: EveryDay, TZ: "America/New_York"}
	at := func(h, m int) time.Time { return time.Date(2026, 10, 21, h, m, 0, 0, ny) } // a Wednesday

	assert.Equal(t, at(8, 0), s.Next(at(3, 0)), "before the start: the start")
	assert.Equal(t, at(9, 30), s.Next(at(8, 0)), "strictly after")
	assert.Equal(t, at(11, 0), s.Next(at(9, 45)))
	// 08:00 + 90m*10 = 23:00 is the last run; after it, tomorrow's start.
	assert.Equal(t, at(23, 0), s.Next(at(22, 31)))
	assert.Equal(t, time.Date(2026, 10, 22, 8, 0, 0, 0, ny), s.Next(at(23, 0)))
}

func TestNext_DailyOnChosenDays(t *testing.T) {
	ldn := mustLoad(t, "Europe/London")
	s := Schedule{IntervalMinutes: 1440, Start: "03:00", Days: Weekdays, TZ: "Europe/London"}
	fri := time.Date(2026, 10, 23, 4, 0, 0, 0, ldn)
	assert.Equal(t, time.Weekday(5), fri.Weekday())
	assert.Equal(t, time.Date(2026, 10, 26, 3, 0, 0, 0, ldn), s.Next(fri), "skips the weekend")

	sat := Schedule{IntervalMinutes: 1440, Start: "10:00", Days: 1 << time.Saturday, TZ: "Europe/London"}
	assert.Equal(t, time.Date(2026, 10, 24, 10, 0, 0, 0, ldn), sat.Next(fri))
	assert.Equal(t, time.Date(2026, 10, 31, 10, 0, 0, 0, ldn), sat.Next(time.Date(2026, 10, 24, 10, 0, 0, 0, ldn)), "a week later")
}

func TestNext_DSTTransitions(t *testing.T) {
	ny := mustLoad(t, "America/New_York")
	// Spring forward, 2026-03-08: 02:00-03:00 does not exist.
	s := Schedule{IntervalMinutes: 1440, Start: "02:30", Days: EveryDay, TZ: "America/New_York"}
	got := s.Next(time.Date(2026, 3, 8, 0, 0, 0, 0, ny))
	assert.Equal(t, 3, got.In(ny).Hour(), "a start inside the gap runs just after it")
	assert.Equal(t, 8, got.In(ny).Day())

	// Fall back, 2026-11-01: the day has 25 hours; hourly runs keep going until midnight.
	h := Schedule{IntervalMinutes: 60, Start: "00:00", Days: EveryDay, TZ: "America/New_York"}
	last := time.Date(2026, 11, 1, 23, 0, 0, 0, ny)
	assert.Equal(t, time.Date(2026, 11, 2, 0, 0, 0, 0, ny), h.Next(last))
	n := 0
	for tm := time.Date(2026, 11, 1, 0, 0, 0, 0, ny); tm.Before(time.Date(2026, 11, 2, 0, 0, 0, 0, ny)); tm = h.Next(tm) {
		n++
		require.Less(t, n, 30)
	}
	assert.Equal(t, 25, n, "one run per real hour")
}

func TestNext_UsesServerZoneWhenUnset(t *testing.T) {
	s := Schedule{IntervalMinutes: 1440, Start: "05:00", Days: EveryDay}
	got := s.Next(time.Now())
	assert.Equal(t, 5, got.In(time.Local).Hour())
}

func TestRunsPerDayAndReads(t *testing.T) {
	assert.Equal(t, 24.0, Schedule{IntervalMinutes: 60, Start: "00:00", Days: EveryDay}.RunsPerDay())
	assert.Equal(t, 12.0, Schedule{IntervalMinutes: 60, Start: "12:00", Days: EveryDay}.RunsPerDay())
	assert.InDelta(t, 5.0/7, Schedule{IntervalMinutes: 1440, Start: "03:00", Days: Weekdays}.RunsPerDay(), 1e-9)

	// 24 probes plus about one full read of 1,000 videos (30 units).
	assert.Equal(t, 54, DailyReadUnits(Schedule{IntervalMinutes: 60, Start: "00:00", Days: EveryDay}, 1000))
	// Weekly: 1/7 of a probe and 1/7 of a read.
	assert.Equal(t, 5, DailyReadUnits(Schedule{IntervalMinutes: 1440, Start: "00:00", Days: 1}, 1000))
	assert.Equal(t, 2, DailyReadUnits(Schedule{IntervalMinutes: 1440, Start: "00:00", Days: EveryDay}, 0), "an empty playlist still costs a probe and a list call")

	assert.Equal(t, 24*time.Hour, FullReadEvery(10))
	assert.Equal(t, 3*24*time.Hour, FullReadEvery(3200))
	assert.Equal(t, 7*24*time.Hour, FullReadEvery(50000))
}

func TestRecommend_SizeTiers(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct{ songs, interval int }{{50, 30}, {300, 30}, {301, 120}, {1500, 120}, {5000, 360}, {9000, 1440}} {
		r := Recommend(c.songs, 0, nil, 1_000_000, "UTC", now)
		assert.Equal(t, c.interval, r.Schedule.IntervalMinutes, "%d songs", c.songs)
		assert.Equal(t, EveryDay, r.Schedule.Days)
		assert.Equal(t, "UTC", r.Schedule.TZ)
		assert.Empty(t, r.Warning)
		require.NoError(t, r.Schedule.Validate())
	}
	assert.Equal(t, "02:00", Recommend(9000, 0, nil, 1_000_000, "UTC", now).Schedule.Start, "daily runs start at night")
}

func TestRecommend_SlowsDownWhenReadsWouldCrowdTheQuota(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	busy := []Existing{}
	for i := 0; i < 30; i++ {
		busy = append(busy, Existing{Schedule: Schedule{IntervalMinutes: 30, Start: "00:00", Days: EveryDay}, YouTubeVideos: 200})
	}
	r := Recommend(100, 100, busy, 10000, "UTC", now)
	assert.Greater(t, r.ReadsBefore, 1500)
	assert.Equal(t, 1440, r.Schedule.IntervalMinutes, "already over 15%: slowed to daily")
	assert.NotEmpty(t, r.Warning)

	light := []Existing{{Schedule: Schedule{IntervalMinutes: 30, Start: "00:00", Days: EveryDay}, YouTubeVideos: 200}}
	r = Recommend(100, 100, light, 10000, "UTC", now)
	assert.Equal(t, 30, r.Schedule.IntervalMinutes)
	assert.LessOrEqual(t, r.ReadsAfter, 1500)
	assert.Empty(t, r.Warning)

	// A small quota (15% of 600 = 90) steps up only as far as needed: 54 + 51 at 30 minutes
	// is too much, 54 + 27 hourly fits.
	r = Recommend(100, 100, light, 600, "UTC", now)
	assert.Equal(t, 60, r.Schedule.IntervalMinutes)
	assert.Contains(t, r.Reason, "15%")
}

func TestRecommend_SpacesStartsAwayFromOtherMappings(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	// Daily mappings at 02:00 and 14:00 UTC: the gap is largest at 08:00 or 20:00.
	existing := []Existing{
		{Schedule: Schedule{IntervalMinutes: 1440, Start: "02:00", Days: EveryDay, TZ: "UTC"}},
		{Schedule: Schedule{IntervalMinutes: 1440, Start: "14:00", Days: EveryDay, TZ: "UTC"}},
	}
	r := Recommend(9000, 0, existing, 1_000_000, "UTC", now)
	assert.Contains(t, []string{"08:00", "20:00"}, r.Schedule.Start)

	// An hourly mapping on :00 puts a new hourly one at :30.
	hourly := []Existing{{Schedule: Schedule{IntervalMinutes: 60, Start: "00:00", Days: EveryDay, TZ: "UTC"}}}
	assert.Equal(t, "00:30", recommendInterval(t, 60, hourly, now))
	assert.True(t, strings.HasSuffix(recommendInterval(t, 120, hourly, now), ":30"))

	// Zones are compared in UTC: 02:00 in New York is 06:00 or 07:00 UTC.
	ny := []Existing{{Schedule: Schedule{IntervalMinutes: 1440, Start: "02:00", Days: EveryDay, TZ: "America/New_York"}}}
	start := Recommend(9000, 0, ny, 1_000_000, "UTC", now).Schedule.Start
	m, _ := ParseStart(start)
	assert.InDelta(t, (6*60+12*60)%1440, m, 60, "opposite side of the clock from 06:00 UTC, got %s", start)
}

// recommendInterval returns bestStart for a schedule of a given interval.
func recommendInterval(t *testing.T, interval int, existing []Existing, now time.Time) string {
	t.Helper()
	return bestStart(Schedule{IntervalMinutes: interval, Days: EveryDay, TZ: "UTC"}, existing, now)
}

func TestDistancesWrapAround(t *testing.T) {
	var busy [1440]bool
	busy[10] = true
	d := distances(busy)
	assert.Equal(t, 0, d[10])
	assert.Equal(t, 11, d[1439], "wraps past midnight")
	assert.Equal(t, 720, d[730])
}
