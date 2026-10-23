package quota_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/apiusage"
	"github.com/manlikeabro/spotube/internal/migrate"
	"github.com/manlikeabro/spotube/internal/quota"
	"github.com/manlikeabro/spotube/internal/sqliteconn"
)

var pacific = func() *time.Location {
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		panic(err)
	}
	return loc
}()

// budgetAt returns a budget with the given limit, reserve and daily reads, frozen at now.
func budgetAt(now *time.Time, limit, reserve, dailyReads int) *quota.Budget {
	b := quota.New(nil, func() int { return limit }, func() int { return reserve }, func() int { return dailyReads })
	b.Now = func() time.Time { return *now }
	return b
}

func TestBudget_ClassesGetDifferentAllowances(t *testing.T) {
	// Midnight Pacific: the whole day's reads are still expected.
	now := time.Date(2026, 10, 23, 0, 0, 0, 0, pacific)
	b := budgetAt(&now, 10000, 500, 1000)
	ctx := context.Background()

	require.NoError(t, b.Charge(quota.WithClass(ctx, quota.Drain), 8500)) // 10000-500-1000
	assert.ErrorIs(t, b.Allows(quota.Drain, 1), quota.ErrBudget, "the drain stops before scheduled reads' share")

	require.NoError(t, b.Allows(quota.Read, 1000))
	assert.ErrorIs(t, b.Allows(quota.Read, 1001), quota.ErrBudget, "reads stop at the reserve")

	require.NoError(t, b.Allows(quota.Interactive, 1450))
	assert.ErrorIs(t, b.Allows(quota.Interactive, 1451), quota.ErrBudget, "interactive keeps the safety margin")

	st := b.Status()
	assert.Equal(t, 8500, st.Used)
	assert.Equal(t, 1450, st.Available)
	assert.Equal(t, 0, st.BackgroundLeft)
	assert.Equal(t, 1000, st.ReadsStillExpected)
}

func TestBudget_ReadsStillExpectedShrinkThroughTheDay(t *testing.T) {
	now := time.Date(2026, 10, 23, 18, 0, 0, 0, pacific) // a quarter of the day left
	b := budgetAt(&now, 10000, 0, 1000)
	assert.Equal(t, 250, b.Status().ReadsStillExpected)
	require.NoError(t, b.Allows(quota.Drain, 9750))
	assert.Error(t, b.Allows(quota.Drain, 9751))
}

func TestBudget_ChargeRefundAndRefusalDetails(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, pacific)
	b := budgetAt(&now, 1000, 0, 0)
	ctx := context.Background()

	require.NoError(t, b.Charge(ctx, 900))
	b.Refund(100)
	assert.Equal(t, 800, b.Status().Used)
	b.Refund(5000)
	assert.Equal(t, 0, b.Status().Used, "refunds never go below zero")

	require.NoError(t, b.Charge(ctx, 950))
	err := b.Charge(ctx, 1)
	var qe *quota.Error
	require.ErrorAs(t, err, &qe)
	assert.Equal(t, quota.Interactive, qe.Class)
	assert.Equal(t, 1, qe.Needed)
	assert.Equal(t, 0, qe.Available)
	assert.False(t, qe.Exhausted)
	assert.Equal(t, time.Date(2026, 10, 24, 0, 0, 0, 0, pacific), qe.RetryAt.In(pacific), "retry at the reset")
	assert.Contains(t, qe.Error(), "Waiting for YouTube quota")
}

func TestBudget_ExhaustedLatchRechecksAndResets(t *testing.T) {
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, pacific)
	b := budgetAt(&now, 10000, 0, 0)

	b.Exhausted()
	err := b.Allows(quota.Interactive, 1)
	var qe *quota.Error
	require.ErrorAs(t, err, &qe)
	assert.True(t, qe.Exhausted)
	assert.True(t, b.Status().Exhausted)
	assert.Equal(t, 0, b.Status().Available)

	now = now.Add(quota.ExhaustedRecheck)
	assert.NoError(t, b.Allows(quota.Interactive, 1), "re-checked after 30 minutes")

	// Close to the reset the latch ends at the reset, not 30 minutes later.
	now = time.Date(2026, 10, 23, 23, 50, 0, 0, pacific)
	b.Exhausted()
	assert.Error(t, b.Allows(quota.Interactive, 1))
	now = time.Date(2026, 10, 24, 0, 0, 1, 0, pacific)
	assert.NoError(t, b.Allows(quota.Interactive, 1))
}

func TestBudget_RolloverStartsTheDayFromRecordedUsage(t *testing.T) {
	db, err := sqliteconn.OpenWithPragmas(filepath.Join(t.TempDir(), "q.db"))
	require.NoError(t, err)
	require.NoError(t, migrate.Up(db))
	defer db.Close()

	now := time.Date(2026, 10, 23, 12, 0, 0, 0, pacific)
	_, err = db.Exec(`INSERT INTO api_usage (day, service, units, calls) VALUES (?, 'youtube', 4000, 10)`, apiusage.Day(now))
	require.NoError(t, err)

	b := quota.New(db, func() int { return 10000 }, nil, nil)
	b.Now = func() time.Time { return now }
	assert.Equal(t, 4000, b.Status().Used, "seeded from api_usage")
	require.NoError(t, b.Charge(context.Background(), 100))
	assert.Equal(t, 4100, b.Status().Used)

	now = time.Date(2026, 10, 24, 0, 5, 0, 0, pacific)
	assert.Equal(t, 0, b.Status().Used, "a new quota day starts empty")
	assert.Equal(t, apiusage.Day(now), b.Status().Day)
}

// TestTransport_GateChargesRefundsAndLatches exercises the transport with a real budget.
func TestTransport_GateChargesRefundsAndLatches(t *testing.T) {
	db, err := sqliteconn.OpenWithPragmas(filepath.Join(t.TempDir(), "t.db"))
	require.NoError(t, err)
	require.NoError(t, migrate.Up(db))
	defer db.Close()
	apiusage.SetDB(db)
	defer apiusage.SetDB(nil)

	b := quota.New(db, func() int { return 300 }, func() int { return 0 }, nil)
	apiusage.SetGate(b)
	defer apiusage.SetGate(nil)

	answer := http.StatusOK
	body := `{}`
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(answer)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	client := &http.Client{Transport: &apiusage.Transport{Base: http.DefaultTransport}}
	do := func(ctx context.Context, method, path string) (*http.Response, error) {
		req, _ := http.NewRequestWithContext(ctx, method, srv.URL+path, nil)
		resp, err := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
		return resp, err
	}
	ctx := context.Background()

	// A search (100) goes out and is counted.
	_, err = do(ctx, http.MethodGet, "/youtube/v3/search")
	require.NoError(t, err)
	assert.Equal(t, 100, b.Status().Used)

	// A second search would pass 300-50; refused before it is sent.
	_, err = do(ctx, http.MethodGet, "/youtube/v3/search")
	require.NoError(t, err)
	_, err = do(ctx, http.MethodGet, "/youtube/v3/search")
	assert.ErrorIs(t, err, quota.ErrBudget)
	assert.Equal(t, 2, calls, "the refused call never reached the server")
	assert.Equal(t, 200, b.Status().Used)

	// Google says the quota is used up: refunded, and nothing more is sent.
	answer, body = http.StatusForbidden, `{"error":{"errors":[{"reason":"quotaExceeded"}]}}`
	_, err = do(ctx, http.MethodGet, "/youtube/v3/playlists")
	require.NoError(t, err)
	assert.Equal(t, 200, b.Status().Used)
	assert.True(t, b.Status().Exhausted)
	_, err = do(ctx, http.MethodGet, "/youtube/v3/playlists")
	assert.ErrorIs(t, err, quota.ErrBudget)
	assert.Equal(t, 3, calls)

	used, err := apiusage.Today(db, "youtube")
	require.NoError(t, err)
	assert.Equal(t, 200, used, "only calls that cost quota are recorded")
}

func TestTransport_RateLimitIsNotExhaustion(t *testing.T) {
	b := quota.New(nil, func() int { return 10000 }, nil, nil)
	apiusage.SetGate(b)
	defer apiusage.SetGate(nil)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"errors":[{"reason":"rateLimitExceeded"}]}}`))
	}))
	defer srv.Close()
	client := &http.Client{Transport: &apiusage.Transport{Base: http.DefaultTransport}}
	resp, err := client.Get(srv.URL + "/youtube/v3/playlists")
	require.NoError(t, err)
	resp.Body.Close()
	assert.False(t, b.Status().Exhausted)
}

func TestTransport_RefundsWhenTheRequestFails(t *testing.T) {
	b := quota.New(nil, func() int { return 10000 }, nil, nil)
	apiusage.SetGate(b)
	defer apiusage.SetGate(nil)
	client := &http.Client{Transport: &apiusage.Transport{Base: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection refused")
	})}}
	_, err := client.Get("http://127.0.0.1:1/youtube/v3/search")
	require.Error(t, err)
	assert.Equal(t, 0, b.Status().Used)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestEstimates(t *testing.T) {
	assert.Equal(t, 0, quota.FullReadUnits(0))
	assert.Equal(t, 2, quota.FullReadUnits(50))    // 1 page + 1 duration batch
	assert.Equal(t, 30, quota.FullReadUnits(1000)) // 20 pages + 10

	assert.Equal(t, 0, quota.DaysToDrain(100, 100, 5000))
	assert.Equal(t, 1, quota.DaysToDrain(5100, 100, 5000))
	assert.Equal(t, 2, quota.DaysToDrain(5101, 100, 5000))
	assert.Equal(t, 1, quota.DaysToDrain(100, -20, 5000), "a negative balance counts as nothing left")
	assert.Equal(t, -1, quota.DaysToDrain(100, 0, 0))

	w := quota.Work{ToYouTube: 10, ToYouTubeLinked: 4, ToSpotify: 99, YouTubeReadVideos: 50, YouTubePlaylists: 1}
	st := quota.Status{Limit: 10000, Reserve: 500, DailyReads: 500, BackgroundLeft: 1000}
	e := quota.EstimateWork(w, st, 2000)
	assert.Equal(t, 1010, e.SearchUnits)
	assert.Equal(t, 700, e.InsertUnits)
	assert.Equal(t, 2, e.ReadUnits)
	assert.Equal(t, 50, e.CreateUnits)
	assert.Equal(t, 1762, e.Units)
	assert.Equal(t, 9000, e.DailyCapacity)
	assert.Equal(t, 1, e.Days, "3,762 with 1,000 left today finishes tomorrow")
	assert.Equal(t, quota.Work{ToYouTube: 11, ToYouTubeLinked: 4, ToSpotify: 99, YouTubeReadVideos: 50, YouTubePlaylists: 1}, w.Add(quota.Work{ToYouTube: 1}))
}
