// Package apiusage measures YouTube Data API quota by recording the unit cost of every
// request the app makes, per quota day (quota resets at midnight Pacific time).
package apiusage

import (
	"database/sql"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// YouTubeDailyQuota is the default daily quota of a Google Cloud project.
const YouTubeDailyQuota = 10000

var quotaZone = func() *time.Location {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		return time.FixedZone("PT", -8*3600)
	}
	return loc
}()

var defaultDB atomic.Pointer[sql.DB]

// SetDB sets the database usage is recorded to. Until it is set, Record is a no-op.
func SetDB(db *sql.DB) { defaultDB.Store(db) }

// Day returns the quota day containing t, as YYYY-MM-DD in Pacific time.
func Day(t time.Time) string { return t.In(quotaZone).Format("2006-01-02") }

// DayStart returns the start of the quota day containing t.
func DayStart(t time.Time) time.Time {
	p := t.In(quotaZone)
	return time.Date(p.Year(), p.Month(), p.Day(), 0, 0, 0, 0, quotaZone)
}

// YouTubeCost returns the quota cost of a YouTube Data API v3 request.
func YouTubeCost(method, path string) int {
	resource := path[strings.LastIndex(path, "/")+1:]
	if resource == "search" {
		return 100
	}
	if method == http.MethodGet {
		return 1
	}
	// insert, update and delete on playlists and playlistItems.
	return 50
}

// Record adds units to today's total for service.
func Record(service string, units int) {
	db := defaultDB.Load()
	if db == nil || units <= 0 {
		return
	}
	_, _ = db.Exec(
		`INSERT INTO api_usage (day, service, units, calls) VALUES (?, ?, ?, 1)
		 ON CONFLICT(day, service) DO UPDATE SET units = units + excluded.units, calls = calls + 1`,
		Day(time.Now()), service, units)
}

// Today returns units used today for service.
func Today(db *sql.DB, service string) (int, error) {
	var units int
	err := db.QueryRow(`SELECT COALESCE(SUM(units), 0) FROM api_usage WHERE day = ? AND service = ?`,
		Day(time.Now()), service).Scan(&units)
	return units, err
}

// Remaining returns YouTube quota units left today.
func Remaining(db *sql.DB) int {
	used, err := Today(db, "youtube")
	if err != nil {
		return YouTubeDailyQuota
	}
	return YouTubeDailyQuota - used
}

// Transport counts YouTube API requests that reach Google (any response status costs quota).
type Transport struct {
	Base http.RoundTripper
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.Base.RoundTrip(req)
	if resp != nil && strings.Contains(req.URL.Path, "/youtube/v3/") {
		Record("youtube", YouTubeCost(req.Method, req.URL.Path))
	}
	return resp, err
}
