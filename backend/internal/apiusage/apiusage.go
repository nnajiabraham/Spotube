// Package apiusage measures YouTube Data API quota by recording the unit cost of every
// request the app makes, per quota day (quota resets at midnight Pacific time).
package apiusage

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// YouTubeDailyQuota is the default daily quota of a Google Cloud project.
const YouTubeDailyQuota = 10000

var limitFunc atomic.Pointer[func() int]

// SetLimit makes Limit (and Remaining) follow a setting instead of the default.
func SetLimit(f func() int) { limitFunc.Store(&f) }

// Limit returns today's YouTube quota limit.
func Limit() int {
	if f := limitFunc.Load(); f != nil {
		return (*f)()
	}
	return YouTubeDailyQuota
}

// Gate decides YouTube calls before they are sent (see internal/quota).
type Gate interface {
	// Charge takes units for a request about to go out, or returns an error to refuse it.
	Charge(ctx context.Context, units int) error
	// Refund returns units for a request that cost nothing.
	Refund(units int)
	// Exhausted is called when Google answers that the daily quota is used up.
	Exhausted()
}

type gateHolder struct{ Gate }

var gate atomic.Pointer[gateHolder]

// SetGate installs the gate every YouTube request passes through (nil removes it).
func SetGate(g Gate) {
	if g == nil {
		gate.Store(nil)
		return
	}
	gate.Store(&gateHolder{g})
}

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
	return UsedOn(db, service, Day(time.Now()))
}

// UsedOn returns units used on a quota day (see Day) for service.
func UsedOn(db *sql.DB, service, day string) (int, error) {
	var units int
	err := db.QueryRow(`SELECT COALESCE(SUM(units), 0) FROM api_usage WHERE day = ? AND service = ?`,
		day, service).Scan(&units)
	return units, err
}

// Remaining returns YouTube quota units left today.
func Remaining(db *sql.DB) int {
	used, err := Today(db, "youtube")
	if err != nil {
		return Limit()
	}
	return Limit() - used
}

// Transport gates and counts YouTube API requests. A request is charged to the gate before
// it is sent; one that never reached Google, or that Google refused because the daily quota
// is used up, is refunded. Any other response costs quota and is recorded.
type Transport struct {
	Base http.RoundTripper
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !strings.Contains(req.URL.Path, "/youtube/v3/") {
		return t.Base.RoundTrip(req)
	}
	cost := YouTubeCost(req.Method, req.URL.Path)
	g := gate.Load()
	if g != nil {
		if err := g.Charge(req.Context(), cost); err != nil {
			if req.Body != nil {
				req.Body.Close()
			}
			return nil, err
		}
	}
	resp, err := t.Base.RoundTrip(req)
	if resp == nil {
		if g != nil {
			g.Refund(cost)
		}
		return resp, err
	}
	if resp.StatusCode == http.StatusForbidden && quotaExhausted(resp) {
		if g != nil {
			g.Refund(cost)
			g.Exhausted()
		}
		return resp, err
	}
	Record("youtube", cost)
	return resp, err
}

// quotaExhausted reports whether a 403 says the daily quota is used up (not a per-minute rate
// limit), leaving the body readable for the caller.
func quotaExhausted(resp *http.Response) bool {
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return false
	}
	return IsQuotaExhaustedMessage(string(body))
}

// IsQuotaExhaustedMessage reports whether an API error text means the daily quota is used
// up. Per-minute limits ("rateLimitExceeded", "userRateLimitExceeded") are not.
func IsQuotaExhaustedMessage(s string) bool {
	return strings.Contains(s, "quotaExceeded") || strings.Contains(s, "dailyLimitExceeded")
}
