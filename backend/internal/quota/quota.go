// Package quota decides whether a YouTube Data API call may run, so background sync spreads
// the daily quota instead of spending it at midnight and starving everything else.
//
// Every YouTube request passes through apiusage.Transport, which charges the budget before
// sending and refunds requests that never reached Google or were refused for quota. Work is
// classed by the request context:
//
//   - Interactive (default): the user is waiting: searches, creating a playlist, Sync now's
//     read, restores. May use the whole limit minus a small safety margin.
//   - Read: scheduled analysis and backup reads. May use the limit minus the interactive
//     reserve.
//   - Drain: applying the queue. May use the limit minus the reserve minus the reads still
//     expected before the quota resets, so scheduled syncs keep working all day.
package quota

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/manlikeabro/spotube/internal/apiusage"
)

// Class is the kind of work a YouTube call belongs to.
type Class int

const (
	Interactive Class = iota
	Read
	Drain
)

func (c Class) String() string {
	switch c {
	case Read:
		return "read"
	case Drain:
		return "drain"
	}
	return "interactive"
}

type classKey struct{}

// WithClass marks ctx's YouTube calls as class c.
func WithClass(ctx context.Context, c Class) context.Context {
	return context.WithValue(ctx, classKey{}, c)
}

// ClassOf returns ctx's class (Interactive when unset).
func ClassOf(ctx context.Context) Class {
	if c, ok := ctx.Value(classKey{}).(Class); ok {
		return c
	}
	return Interactive
}

// SafetyMargin is kept back even from interactive work, so a count that is slightly off
// never pushes the project over Google's limit.
const SafetyMargin = 50

// ExhaustedRecheck is how long a quota error from Google stops YouTube calls before one is
// tried again. Google's own count can differ from ours (another app on the same project, or a
// reset that is not exactly at midnight), so a refusal is re-checked rather than trusted all
// day.
const ExhaustedRecheck = 30 * time.Minute

// ErrBudget is matched (errors.Is) by every budget refusal.
var ErrBudget = errors.New("youtube quota budget")

// Error is a budget refusal.
type Error struct {
	Class     Class
	Needed    int
	Available int
	Exhausted bool // Google itself reported the quota used up
	RetryAt   time.Time
}

func (e *Error) Error() string {
	if e.Exhausted {
		return fmt.Sprintf("YouTube reported its daily quota used up; retrying after %s", e.RetryAt.Local().Format("15:04"))
	}
	return fmt.Sprintf("Waiting for YouTube quota: %s work needs %d units, %d available (resets %s)",
		e.Class, e.Needed, max(e.Available, 0), e.RetryAt.Local().Format("15:04"))
}

func (e *Error) Is(target error) bool { return target == ErrBudget }

// Budget tracks today's YouTube usage in memory (seeded from api_usage) and decides calls.
type Budget struct {
	db *sql.DB
	// Limit and Reserve are read on every decision, so settings changes apply at once.
	Limit   func() int
	Reserve func() int
	// DailyReads estimates the read units scheduled syncs use per day; the part still to come
	// before the reset is kept away from the drain.
	DailyReads func() int
	Now        func() time.Time

	mu             sync.Mutex
	day            string
	used           int
	exhaustedUntil time.Time
}

// New returns a budget backed by db's api_usage table.
func New(db *sql.DB, limit, reserve, dailyReads func() int) *Budget {
	return &Budget{db: db, Limit: limit, Reserve: reserve, DailyReads: dailyReads, Now: time.Now}
}

// rollover loads today's usage when the quota day changes (and on first use). Callers hold mu.
func (b *Budget) rollover(now time.Time) {
	day := apiusage.Day(now)
	if day == b.day {
		return
	}
	b.day = day
	b.used = 0
	b.exhaustedUntil = time.Time{}
	if b.db != nil {
		if used, err := apiusage.UsedOn(b.db, "youtube", day); err == nil {
			b.used = used
		}
	}
}

// ResetAt is when the quota day containing now ends.
func ResetAt(now time.Time) time.Time {
	return apiusage.DayStart(now).AddDate(0, 0, 1)
}

// readsStillExpected is the share of a day's scheduled reads left before the reset.
func (b *Budget) readsStillExpected(now time.Time) int {
	if b.DailyReads == nil {
		return 0
	}
	start, end := apiusage.DayStart(now), ResetAt(now)
	left := end.Sub(now).Seconds() / end.Sub(start).Seconds()
	return int(float64(b.DailyReads()) * left)
}

func (b *Budget) allowance(c Class, now time.Time) int {
	limit := b.Limit()
	reserve := 0
	if b.Reserve != nil {
		reserve = b.Reserve()
	}
	switch c {
	case Read:
		return limit - reserve
	case Drain:
		return limit - reserve - b.readsStillExpected(now)
	}
	return limit - SafetyMargin
}

func (b *Budget) refusal(c Class, units int, now time.Time) error {
	if now.Before(b.exhaustedUntil) {
		return &Error{Class: c, Needed: units, Exhausted: true, RetryAt: b.exhaustedUntil}
	}
	if avail := b.allowance(c, now) - b.used; units > avail {
		return &Error{Class: c, Needed: units, Available: avail, RetryAt: ResetAt(now)}
	}
	return nil
}

// Allows reports whether units of class c fit now, without charging. Multi-call jobs (a
// search plus an insert, a full playlist read) check the whole job first so they don't stop
// half-way and waste what they already spent.
func (b *Budget) Allows(c Class, units int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.Now()
	b.rollover(now)
	return b.refusal(c, units, now)
}

// Charge takes units for a call about to be sent by ctx's class, or refuses.
func (b *Budget) Charge(ctx context.Context, units int) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.Now()
	b.rollover(now)
	if err := b.refusal(ClassOf(ctx), units, now); err != nil {
		return err
	}
	b.used += units
	return nil
}

// Refund returns units charged for a call that never cost quota.
func (b *Budget) Refund(units int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.used = max(0, b.used-units)
}

// Exhausted records Google's quotaExceeded answer: no YouTube calls until the re-check time
// (or the reset, if sooner).
func (b *Budget) Exhausted() {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.Now()
	b.rollover(now)
	until := now.Add(ExhaustedRecheck)
	if reset := ResetAt(now); reset.Before(until) {
		until = reset
	}
	b.exhaustedUntil = until
}

// Status is the budget at a moment, for the API.
type Status struct {
	Day                string    `json:"day"`
	Used               int       `json:"used"`
	Limit              int       `json:"limit"`
	Reserve            int       `json:"reserve"`
	ReadsStillExpected int       `json:"reads_still_expected"`
	DailyReads         int       `json:"daily_reads"`
	Available          int       `json:"available"`            // interactive
	BackgroundLeft     int       `json:"background_available"` // drain
	Exhausted          bool      `json:"exhausted"`
	RetryAt            *int64    `json:"retry_at,omitempty"`
	ResetAt            time.Time `json:"-"`
	ResetAtUnix        int64     `json:"reset_at"`
}

// Status returns the current budget.
func (b *Budget) Status() Status {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := b.Now()
	b.rollover(now)
	reserve := 0
	if b.Reserve != nil {
		reserve = b.Reserve()
	}
	daily := 0
	if b.DailyReads != nil {
		daily = b.DailyReads()
	}
	s := Status{
		Day:                b.day,
		Used:               b.used,
		Limit:              b.Limit(),
		Reserve:            reserve,
		DailyReads:         daily,
		ReadsStillExpected: b.readsStillExpected(now),
		Available:          max(0, b.allowance(Interactive, now)-b.used),
		BackgroundLeft:     max(0, b.allowance(Drain, now)-b.used),
		ResetAt:            ResetAt(now),
	}
	s.ResetAtUnix = s.ResetAt.Unix()
	if now.Before(b.exhaustedUntil) {
		s.Exhausted = true
		s.Available, s.BackgroundLeft = 0, 0
		t := b.exhaustedUntil.Unix()
		s.RetryAt = &t
	}
	return s
}
