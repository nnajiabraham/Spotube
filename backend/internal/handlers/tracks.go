package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/samber/lo"

	"github.com/manlikeabro/spotube/internal/apiusage"
	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/db/table"
	"github.com/manlikeabro/spotube/internal/jobs"
	"github.com/manlikeabro/spotube/internal/matching"
	"github.com/manlikeabro/spotube/internal/platform"
	"github.com/manlikeabro/spotube/internal/quota"
	"github.com/manlikeabro/spotube/internal/tracklinks"

	"github.com/go-jet/jet/v2/sqlite"
)

// SpotifyCatalog is the Spotify search used by the review queue.
type SpotifyCatalog interface {
	Search(ctx context.Context, query string, limit int) ([]platform.SearchHit, error)
	GetTrack(ctx context.Context, id string) (*platform.SearchHit, error)
}

// YouTubeCatalog is the YouTube search used by the review queue.
type YouTubeCatalog interface {
	Search(ctx context.Context, query string, limit int) ([]platform.SearchHit, error)
	GetVideo(ctx context.Context, id string) (*platform.SearchHit, error)
}

const (
	searchLimit    = 10
	searchCacheTTL = 30 * time.Minute
)

// TracksHandler serves in-app track search and the remembered track links.
type TracksHandler struct {
	DB      *sql.DB
	Spotify SpotifyCatalog
	YouTube YouTubeCatalog
	// Budget refuses a YouTube search (search plus durations, 101 units) that won't fit
	// before any of it is spent; nil leaves it to the transport.
	Budget *quota.Budget

	mu    sync.Mutex
	cache map[string]cachedSearch
	now   func() time.Time
}

type cachedSearch struct {
	hits    []platform.SearchHit
	expires time.Time
}

func NewTracksHandler(db *sql.DB, sp SpotifyCatalog, yt YouTubeCatalog) *TracksHandler {
	return &TracksHandler{DB: db, Spotify: sp, YouTube: yt, cache: map[string]cachedSearch{}, now: time.Now}
}

func RegisterTracksRoutes(group *echo.Group, h *TracksHandler) {
	group.GET("/tracks/search", h.Search)
	group.GET("/tracks/lookup", h.Lookup)
	group.GET("/track-links", h.ListLinks)
	group.DELETE("/track-links/:id", h.DeleteLink)
}

// ScoredHit is a search result, scored against the sync item's source track when one is given.
type ScoredHit struct {
	platform.SearchHit
	Score     *float64        `json:"score,omitempty"`
	Breakdown *matching.Score `json:"breakdown,omitempty"`
}

// SearchResponse is returned by GET /api/tracks/search.
type SearchResponse struct {
	Items  []ScoredHit `json:"items"`
	Cached bool        `json:"cached"`
	// QuotaCost is the YouTube quota this search spent (0 for Spotify and cached results).
	QuotaCost int `json:"quota_cost"`
}

// Search runs GET /api/tracks/search?service=&q=&item_id=. Results are cached for 30
// minutes; YouTube searches cost 101 quota units and are refused near the daily limit.
func (h *TracksHandler) Search(c echo.Context) error {
	service := c.QueryParam("service")
	query := strings.Join(strings.Fields(c.QueryParam("q")), " ")
	if service != "spotify" && service != "youtube" {
		return echo.NewHTTPError(http.StatusBadRequest, "service must be spotify or youtube")
	}
	if query == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "q is required")
	}
	if len(query) > 200 {
		return echo.NewHTTPError(http.StatusBadRequest, "q is too long")
	}
	source, err := h.sourceFor(c)
	if err != nil {
		return err
	}

	key := service + "|" + strings.ToLower(query)
	hits, cached := h.cached(key)
	cost := 0
	if !cached {
		if service == "youtube" {
			// The quota budget refuses the search (429) when it won't fit; see searchError.
			if h.Budget != nil {
				if err := h.Budget.Allows(quota.Interactive, quota.SearchCost); err != nil {
					return searchError(err)
				}
			}
			hits, err = h.YouTube.Search(c.Request().Context(), query, searchLimit)
			if err != nil {
				return searchError(err)
			}
			cost = quota.SearchCost
		} else {
			hits, err = h.Spotify.Search(c.Request().Context(), query, searchLimit)
			if err != nil {
				return searchError(err)
			}
		}
		h.store(key, hits)
	}
	return c.JSON(http.StatusOK, SearchResponse{Items: scoreHits(source, service, hits), Cached: cached, QuotaCost: cost})
}

// Lookup runs GET /api/tracks/lookup?service=&ref=&item_id=, resolving a pasted link or ID.
func (h *TracksHandler) Lookup(c echo.Context) error {
	service := c.QueryParam("service")
	ref := c.QueryParam("ref")
	var (
		id  string
		ok  bool
		hit *platform.SearchHit
		err error
	)
	switch service {
	case "spotify":
		if id, ok = platform.ParseSpotifyTrackRef(ref); ok {
			hit, err = h.Spotify.GetTrack(c.Request().Context(), id)
		}
	case "youtube":
		if id, ok = platform.ParseYouTubeVideoRef(ref); ok {
			hit, err = h.YouTube.GetVideo(c.Request().Context(), id)
		}
	default:
		return echo.NewHTTPError(http.StatusBadRequest, "service must be spotify or youtube")
	}
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "not a "+serviceName(service)+" track link or ID")
	}
	if err != nil {
		return searchError(err)
	}
	if hit == nil {
		return echo.NewHTTPError(http.StatusNotFound, "no "+serviceName(service)+" track with that ID")
	}
	source, err := h.sourceFor(c)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, scoreHits(source, service, []platform.SearchHit{*hit})[0])
}

// sourceFor loads the source track of item_id, if given, to score results against.
func (h *TracksHandler) sourceFor(c echo.Context) (*matching.Track, error) {
	itemID := strings.TrimSpace(c.QueryParam("item_id"))
	if itemID == "" {
		return nil, nil
	}
	var items []model.SyncItems
	err := table.SyncItems.SELECT(table.SyncItems.AllColumns).
		WHERE(table.SyncItems.ID.EQ(sqlite.String(itemID))).
		LIMIT(1).
		Query(h.DB, &items)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, "failed to load sync item")
	}
	if len(items) == 0 {
		return nil, echo.NewHTTPError(http.StatusNotFound, "sync item not found")
	}
	source := jobs.SourceTrack(items[0])
	return &source, nil
}

// scoreHits scores each hit against source and orders best first; without a source the
// service's own ranking is kept.
func scoreHits(source *matching.Track, service string, hits []platform.SearchHit) []ScoredHit {
	p := matching.PlatformYouTube
	if service == "spotify" {
		p = matching.PlatformSpotify
	}
	out := make([]ScoredHit, 0, len(hits))
	for _, hit := range hits {
		scored := ScoredHit{SearchHit: hit}
		if source != nil {
			breakdown := matching.ScoreCandidate(*source, hit.Track(p))
			scored.Score = &breakdown.Total
			scored.Breakdown = &breakdown
		}
		out = append(out, scored)
	}
	if source != nil {
		sort.SliceStable(out, func(i, j int) bool { return *out[i].Score > *out[j].Score })
	}
	return out
}

func (h *TracksHandler) cached(key string) ([]platform.SearchHit, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	entry, ok := h.cache[key]
	if !ok || h.now().After(entry.expires) {
		return nil, false
	}
	return entry.hits, true
}

func (h *TracksHandler) store(key string, hits []platform.SearchHit) {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	for k, entry := range h.cache {
		if now.After(entry.expires) {
			delete(h.cache, k)
		}
	}
	h.cache[key] = cachedSearch{hits: hits, expires: now.Add(searchCacheTTL)}
}

func searchError(err error) error {
	msg := err.Error()
	if errors.Is(err, quota.ErrBudget) || apiusage.IsQuotaExhaustedMessage(msg) {
		return echo.NewHTTPError(http.StatusTooManyRequests,
			"YouTube quota is almost used up for today; paste a video link instead (it costs 1 unit)")
	}
	if errors.Is(err, context.Canceled) {
		return echo.NewHTTPError(http.StatusRequestTimeout, "search cancelled")
	}
	return echo.NewHTTPError(http.StatusBadGateway, "search failed: "+msg)
}

// TrackLinksResponse is a page of remembered links.
type TrackLinksResponse struct {
	Items      []tracklinks.Link `json:"items"`
	Page       int               `json:"page"`
	PerPage    int               `json:"perPage"`
	TotalItems int               `json:"totalItems"`
	TotalPages int               `json:"totalPages"`
}

// ListLinks runs GET /api/track-links?filter=&q=&page=&per_page=.
func (h *TracksHandler) ListLinks(c echo.Context) error {
	page := max(parseIntParam(c.QueryParam("page"), 1), 1)
	perPage := min(max(parseIntParam(c.QueryParam("per_page"), 50), 1), 200)
	filter := c.QueryParam("filter")
	if filter != "" && !lo.Contains([]string{tracklinks.FilterLinked, tracklinks.FilterMissingSpotify, tracklinks.FilterMissingYouTube}, filter) {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid filter")
	}
	links, total, err := (&tracklinks.Store{DB: h.DB}).List(c.Request().Context(), filter, c.QueryParam("q"), page, perPage)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list track links")
	}
	return c.JSON(http.StatusOK, TrackLinksResponse{
		Items: links, Page: page, PerPage: perPage, TotalItems: total,
		TotalPages: (total + perPage - 1) / perPage,
	})
}

// DeleteLink runs DELETE /api/track-links/:id; the engine decides afresh on the next sync.
func (h *TracksHandler) DeleteLink(c echo.Context) error {
	deleted, err := (&tracklinks.Store{DB: h.DB}).Delete(c.Request().Context(), c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete track link")
	}
	if !deleted {
		return echo.NewHTTPError(http.StatusNotFound, "track link not found")
	}
	return c.NoContent(http.StatusNoContent)
}
