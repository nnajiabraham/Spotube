package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/manlikeabro/spotube/internal/matching"
	"github.com/manlikeabro/spotube/internal/quota"
	"github.com/manlikeabro/spotube/internal/tracklinks"
)

// SpotifyPlaylistReader reads a Spotify playlist (free), to price copying it exactly.
type SpotifyPlaylistReader interface {
	ListPlaylistTracks(ctx context.Context, playlistID string) ([]matching.Track, error)
}

// QuotaHandler reports the YouTube quota budget and prices proposed work.
type QuotaHandler struct {
	DB      *sql.DB
	Budget  *quota.Budget
	Spotify SpotifyPlaylistReader
}

func RegisterQuotaRoutes(g *echo.Group, h *QuotaHandler) {
	g.GET("", h.Status)
	g.POST("/estimate", h.Estimate)
}

// QuotaStatusResponse is GET /api/quota.
type QuotaStatusResponse struct {
	quota.Status
	Backlog       quota.Backlog `json:"backlog"`
	DailyCapacity int           `json:"daily_capacity"`
	// DaysToDrain is how many quota resets the queued YouTube work needs (0 = today, -1 = never).
	DaysToDrain int `json:"days_to_drain"`
}

func (h *QuotaHandler) Status(c echo.Context) error {
	st := h.Budget.Status()
	backlog, err := quota.LoadBacklog(c.Request().Context(), h.DB)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load the queue")
	}
	capacity := quota.DailyCapacity(st)
	return c.JSON(http.StatusOK, QuotaStatusResponse{
		Status:        st,
		Backlog:       backlog,
		DailyCapacity: capacity,
		DaysToDrain:   quota.DaysToDrain(backlog.Units, st.BackgroundLeft, capacity),
	})
}

// EstimatePair is one proposed mapping. Create names the side Spotube would create.
type EstimatePair struct {
	SpotifyPlaylistID string `json:"spotify_playlist_id,omitempty"`
	SpotifyTracks     int    `json:"spotify_tracks"`
	YoutubeVideos     int    `json:"youtube_videos"`
	Create            string `json:"create,omitempty"`
}

// EstimateRequest is POST /api/quota/estimate.
type EstimateRequest struct {
	Pairs []EstimatePair `json:"pairs"`
}

// EstimateResponse prices the pairs together, behind the current queue.
type EstimateResponse struct {
	quota.Estimate
	Work quota.Work `json:"work"`
	// UpperBound: some songs may already be on both sides, so the real cost can be lower.
	UpperBound bool `json:"upper_bound"`
	// Exact: Spotify playlists were read to count songs Spotube already matched.
	Exact bool `json:"exact"`
	// OverYouTubeLimit: a YouTube playlist would exceed 5,000 videos.
	OverYouTubeLimit bool `json:"over_youtube_limit"`
}

// exactReadLimit is how many Spotify playlists one estimate will read to count known matches.
const exactReadLimit = 5

func (h *QuotaHandler) Estimate(c echo.Context) error {
	var req EstimateRequest
	if err := c.Bind(&req); err != nil || len(req.Pairs) == 0 || len(req.Pairs) > maxMirror {
		return echo.NewHTTPError(http.StatusBadRequest, "pairs must list 1-100 playlists")
	}
	ctx := c.Request().Context()
	var links *tracklinks.Index
	readExact := h.Spotify != nil && len(req.Pairs) <= exactReadLimit
	if readExact {
		var err error
		if links, err = (&tracklinks.Store{DB: h.DB}).LoadIndex(ctx); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to load track links")
		}
	}

	resp := EstimateResponse{Exact: readExact}
	var total quota.Work
	for _, p := range req.Pairs {
		toYouTube, linked := p.SpotifyTracks, 0
		if readExact && strings.TrimSpace(p.SpotifyPlaylistID) != "" {
			tracks, err := h.Spotify.ListPlaylistTracks(ctx, p.SpotifyPlaylistID)
			if err != nil {
				resp.Exact = false
			} else {
				toYouTube, linked = splitByLink(tracks, links)
			}
		} else {
			resp.Exact = false
		}
		var w quota.Work
		switch p.Create {
		case "youtube":
			w = quota.Work{ToYouTube: toYouTube, ToYouTubeLinked: linked, YouTubePlaylists: 1}
			resp.OverYouTubeLimit = resp.OverYouTubeLimit || p.SpotifyTracks > 5000
		case "spotify":
			w = quota.Work{ToSpotify: p.YoutubeVideos, YouTubeReadVideos: p.YoutubeVideos}
		default:
			// Two existing playlists: their overlap is unknown until the first sync reads both.
			w = quota.Work{ToYouTube: toYouTube, ToYouTubeLinked: linked, ToSpotify: p.YoutubeVideos, YouTubeReadVideos: p.YoutubeVideos}
			resp.UpperBound = resp.UpperBound || p.SpotifyTracks > 0 && p.YoutubeVideos > 0
			resp.OverYouTubeLimit = resp.OverYouTubeLimit || p.SpotifyTracks+p.YoutubeVideos > 5000
		}
		total = total.Add(w)
	}

	st := h.Budget.Status()
	backlog, err := quota.LoadBacklog(ctx, h.DB)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load the queue")
	}
	resp.Work = total
	resp.Estimate = quota.EstimateWork(total, st, backlog.Units)
	return c.JSON(http.StatusOK, resp)
}

// splitByLink counts Spotify tracks with no remembered YouTube video and with one. Songs
// marked "not on YouTube" cost nothing and are left out.
func splitByLink(tracks []matching.Track, links *tracklinks.Index) (unlinked, linked int) {
	for _, t := range tracks {
		link, ok := links.For("spotify", t.ID)
		if !ok {
			unlinked++
			continue
		}
		if _, missing := link.Counterpart("youtube"); !missing {
			linked++
		}
	}
	return unlinked, linked
}
