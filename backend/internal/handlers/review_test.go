package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/apiusage"
	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/platform"
	"github.com/manlikeabro/spotube/internal/tracklinks"
)

// recordingExecutor reports the item's status at the moment it is executed.
type recordingExecutor struct {
	db           *sql.DB
	calls        int
	statusAtCall string
}

func (r *recordingExecutor) ExecuteOne(_ context.Context, id string) (model.SyncItems, error) {
	r.calls++
	_ = r.db.QueryRow(`SELECT status FROM sync_items WHERE id = ?`, id).Scan(&r.statusAtCall)
	_, _ = r.db.Exec(`UPDATE sync_items SET status = 'done' WHERE id = ?`, id)
	var item model.SyncItems
	item.ID = &id
	item.MappingID = "mapping-1"
	item.Status = "done"
	return item, nil
}

func resolve(t *testing.T, db *sql.DB, exec SyncItemExecutor, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	RegisterSyncItemsRoutes(e.Group("/api/collections/sync_items/records"), NewSyncItemsHandler(db, exec))
	req := httptest.NewRequest(http.MethodPost, "/api/collections/sync_items/records/"+id+"/resolve", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestResolveMatch_LinksAndAddsImmediately(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()
	insertSyncItemFixture(t, db, "sync-1", "mapping-1", "add", "youtube", "review")
	insertSyncItemFixture(t, db, "sync-2", "mapping-2", "add", "youtube", "review")

	exec := &recordingExecutor{db: db}
	rec := resolve(t, db, exec, "sync-1", `{"action":"match","target_id":"https://music.youtube.com/watch?v=dQw4w9WgXcQ&si=x"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	assert.Equal(t, 1, exec.calls)
	assert.Equal(t, "pending", exec.statusAtCall, "item is requeued before running")
	link, err := (&tracklinks.Store{DB: db}).Find(context.Background(), "spotify", "track-1")
	require.NoError(t, err)
	require.NotNil(t, link)
	assert.Equal(t, "dQw4w9WgXcQ", link.YouTubeID)
	assert.Equal(t, tracklinks.OriginManual, link.Origin)

	var other string
	require.NoError(t, db.QueryRow(`SELECT status FROM sync_items WHERE id = 'sync-2'`).Scan(&other))
	assert.Equal(t, "pending", other, "the same song in another mapping is unblocked too")
}

func TestResolveMatch_RejectsInvalidTarget(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()
	insertSyncItemFixture(t, db, "sync-1", "mapping-1", "add", "youtube", "review")

	exec := &recordingExecutor{db: db}
	rec := resolve(t, db, exec, "sync-1", `{"action":"match","target_id":"https://open.spotify.com/track/4uLU6hMCjMI75M1A2tKUQC"}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Zero(t, exec.calls)
}

func TestResolveUnavailable_SkipsEverywhereWithoutAPICalls(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()
	insertSyncItemFixture(t, db, "sync-1", "mapping-1", "add", "youtube", "review")
	insertSyncItemFixture(t, db, "sync-2", "mapping-2", "add", "youtube", "pending")

	exec := &recordingExecutor{db: db}
	rec := resolve(t, db, exec, "sync-1", `{"action":"unavailable"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Zero(t, exec.calls)

	var resp SyncItemResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "skipped", resp.Status)
	assert.Equal(t, "not on YouTube", resp.ErrorMessage)

	var other string
	require.NoError(t, db.QueryRow(`SELECT status FROM sync_items WHERE id = 'sync-2'`).Scan(&other))
	assert.Equal(t, "skipped", other)

	link, err := (&tracklinks.Store{DB: db}).Find(context.Background(), "spotify", "track-1")
	require.NoError(t, err)
	require.NotNil(t, link)
	_, missing := link.Counterpart("youtube")
	assert.True(t, missing)
}

func TestResolveRetry_RequeuesAndRuns(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()
	insertSyncItemFixture(t, db, "sync-1", "mapping-1", "add", "spotify", "review")

	exec := &recordingExecutor{db: db}
	rec := resolve(t, db, exec, "sync-1", `{"action":"retry"}`)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, 1, exec.calls)
	assert.Equal(t, "pending", exec.statusAtCall)
}

func TestResolve_RejectsFinishedItems(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()
	insertSyncItemFixture(t, db, "sync-1", "mapping-1", "add", "youtube", "done")

	rec := resolve(t, db, &recordingExecutor{db: db}, "sync-1", `{"action":"retry"}`)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestSyncItemsList_IncludesLastRun(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()
	insertSyncItemFixture(t, db, "sync-1", "mapping-1", "add", "youtube", "review")
	now := time.Now().Unix()
	_, err := db.Exec(`INSERT INTO activity_logs (id, level, message, job_type, created, sync_item_id, details_json) VALUES
		('old', 'warn', 'x', 'executor', ?, 'sync-1', '{"data":{"search_query":"old"}}'),
		('new', 'warn', 'x', 'executor', ?, 'sync-1', '{"data":{"search_query":"new"}}')`, now-10, now)
	require.NoError(t, err)

	e := echo.New()
	RegisterSyncItemsRoutes(e.Group("/api/collections/sync_items/records"), NewSyncItemsHandler(db, nil))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/collections/sync_items/records?status=review&include=last_run", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var resp SyncItemsListResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Items, 1)
	assert.JSONEq(t, `{"search_query":"new"}`, string(resp.Items[0].LastRun))
}

type fakeCatalog struct {
	hits    []platform.SearchHit
	byID    map[string]platform.SearchHit
	queries []string
}

func (f *fakeCatalog) Search(_ context.Context, q string, _ int) ([]platform.SearchHit, error) {
	f.queries = append(f.queries, q)
	return f.hits, nil
}

func (f *fakeCatalog) get(id string) (*platform.SearchHit, error) {
	if hit, ok := f.byID[id]; ok {
		return &hit, nil
	}
	return nil, nil
}

func (f *fakeCatalog) GetTrack(_ context.Context, id string) (*platform.SearchHit, error) { return f.get(id) }
func (f *fakeCatalog) GetVideo(_ context.Context, id string) (*platform.SearchHit, error) { return f.get(id) }

func tracksServer(db *sql.DB, sp, yt *fakeCatalog) *echo.Echo {
	e := echo.New()
	RegisterTracksRoutes(e.Group("/api"), NewTracksHandler(db, sp, yt))
	return e
}

func get(e *echo.Echo, url string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec
}

func TestTrackSearch_CachesAndScoresAgainstItem(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()
	insertSyncItemFixture(t, db, "sync-1", "mapping-1", "add", "youtube", "review")
	yt := &fakeCatalog{hits: []platform.SearchHit{
		{ID: "aaaaaaaaaaa", Title: "Something Else", Artist: "Nobody", DurationMs: 200000},
		{ID: "bbbbbbbbbbb", Title: "Song A", Artist: "Artist A", DurationMs: 200000},
	}}
	e := tracksServer(db, &fakeCatalog{}, yt)

	rec := get(e, "/api/tracks/search?service=youtube&q=Song%20A&item_id=sync-1")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var first SearchResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &first))
	assert.False(t, first.Cached)
	assert.Equal(t, 101, first.QuotaCost)
	require.Len(t, first.Items, 2)
	assert.Equal(t, "bbbbbbbbbbb", first.Items[0].ID, "best match first")
	require.NotNil(t, first.Items[0].Breakdown)

	rec = get(e, "/api/tracks/search?service=youtube&q=song%20a&item_id=sync-1")
	var second SearchResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &second))
	assert.True(t, second.Cached)
	assert.Zero(t, second.QuotaCost)
	assert.Len(t, yt.queries, 1, "the repeat search is served from cache")
}

func TestTrackSearch_YouTubeQuotaGuard(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()
	_, err := db.Exec(`INSERT INTO api_usage (day, service, units, calls) VALUES (?, 'youtube', 9600, 50)`, apiusage.Day(time.Now()))
	require.NoError(t, err)
	yt, sp := &fakeCatalog{}, &fakeCatalog{}
	e := tracksServer(db, sp, yt)

	assert.Equal(t, http.StatusTooManyRequests, get(e, "/api/tracks/search?service=youtube&q=x").Code)
	assert.Empty(t, yt.queries)
	assert.Equal(t, http.StatusOK, get(e, "/api/tracks/search?service=spotify&q=x").Code, "Spotify is unaffected")
}

func TestTrackLookup_ParsesLinks(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()
	sp := &fakeCatalog{byID: map[string]platform.SearchHit{"4uLU6hMCjMI75M1A2tKUQC": {ID: "4uLU6hMCjMI75M1A2tKUQC", Title: "Song"}}}
	e := tracksServer(db, sp, &fakeCatalog{})

	rec := get(e, "/api/tracks/lookup?service=spotify&ref=https://open.spotify.com/track/4uLU6hMCjMI75M1A2tKUQC?si=abc")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"title":"Song"`)

	assert.Equal(t, http.StatusBadRequest, get(e, "/api/tracks/lookup?service=spotify&ref=not-a-link").Code)
	assert.Equal(t, http.StatusNotFound, get(e, "/api/tracks/lookup?service=spotify&ref=0000000000000000000000").Code)
}

func TestTrackLinks_ListAndDelete(t *testing.T) {
	db, cleanup := setupSyncItemsTestDB(t)
	defer cleanup()
	store := &tracklinks.Store{DB: db}
	ctx := context.Background()
	require.NoError(t, store.LinkAuto(ctx, "sp1", "yt1", "Song", "Artist", 0.9))
	gone, err := store.Set(ctx, "sp2", "", "Rare", "Someone")
	require.NoError(t, err)
	e := tracksServer(db, &fakeCatalog{}, &fakeCatalog{})

	var page TrackLinksResponse
	require.NoError(t, json.Unmarshal(get(e, "/api/track-links?filter=missing_youtube").Body.Bytes(), &page))
	require.Len(t, page.Items, 1)
	assert.Equal(t, "sp2", page.Items[0].SpotifyID)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/track-links/"+gone.ID, nil))
	assert.Equal(t, http.StatusNoContent, rec.Code)
	require.NoError(t, json.Unmarshal(get(e, "/api/track-links").Body.Bytes(), &page))
	assert.Equal(t, 1, page.TotalItems)
	assert.Equal(t, http.StatusBadRequest, get(e, "/api/track-links?filter=bogus").Code)
}
