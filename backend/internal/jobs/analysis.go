package jobs

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
	spotify "github.com/zmb3/spotify/v2"
	"google.golang.org/api/youtube/v3"

	"github.com/manlikeabro/spotube/internal/activitylogger"
	"github.com/manlikeabro/spotube/internal/auth"
	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/db/table"
)

// AnalysisJob handles playlist analysis and sync item generation.
type AnalysisJob struct {
	db             *sql.DB
	logger         zerolog.Logger
	activityLogger *activitylogger.Logger
	clientFactory  *auth.ClientFactory
}

func NewAnalysisJob(deps JobDeps, clientFactory *auth.ClientFactory) *AnalysisJob {
	return &AnalysisJob{
		db:             deps.DB,
		logger:         deps.Logger,
		activityLogger: deps.ActivityLogger,
		clientFactory:  clientFactory,
	}
}

// Run executes the analysis job logic.
func (j *AnalysisJob) Run(ctx context.Context) error {
	j.logger.Info().Msg("starting analysis job")

	mappings, err := j.getMappingsForAnalysis(ctx)
	if err != nil {
		j.logger.Error().Err(err).Msg("failed to get mappings for analysis")
		return err
	}

	if len(mappings) == 0 {
		j.logger.Debug().Msg("no mappings need analysis")
		return nil
	}

	j.logger.Info().Int("count", len(mappings)).Msg("found mappings for analysis")

	for _, mapping := range mappings {
		err := j.analyzeMapping(ctx, mapping)
		if err != nil {
			j.logger.Error().Err(err).Str("mapping_id", lo.FromPtr(mapping.ID)).Msg("failed to analyze mapping")
			j.activityLogger.RecordError("Analysis failed: "+err.Error(), lo.FromPtr(mapping.ID), "analysis")
		}
	}

	j.logger.Info().Msg("analysis job completed")
	return nil
}

func (j *AnalysisJob) getMappingsForAnalysis(ctx context.Context) ([]model.Mappings, error) {
	now := time.Now().Unix()

	var mappings []model.Mappings
	err := table.Mappings.
		SELECT(table.Mappings.AllColumns).
		Query(j.db, &mappings)
	if err != nil {
		return nil, err
	}

	eligible := make([]model.Mappings, 0, len(mappings))
	for _, mapping := range mappings {
		intervalMinutes := int64(mapping.IntervalMinutes)
		if intervalMinutes <= 0 {
			intervalMinutes = 60
		}

		var lastAnalysis int64
		if mapping.LastAnalysisAt != nil {
			lastAnalysis = int64(*mapping.LastAnalysisAt)
		}

		if lastAnalysis == 0 || now-lastAnalysis >= intervalMinutes*60 {
			eligible = append(eligible, mapping)
		}
	}

	return eligible, nil
}

func (j *AnalysisJob) analyzeMapping(ctx context.Context, mapping model.Mappings) error {
	mappingID := lo.FromPtr(mapping.ID)
	j.logger.Info().Str("mapping_id", mappingID).Msg("analyzing mapping")

	spotifyTracks := []TrackInfo{}
	youtubeTracks := []TrackInfo{}

	if mapping.SyncTracks != 0 {
		spotifyClient, err := j.clientFactory.GetSpotifyClient(ctx)
		if err != nil {
			return err
		}

		youtubeService, err := j.clientFactory.GetYouTubeService(ctx)
		if err != nil {
			return err
		}

		spotifyTracks, err = j.getSpotifyPlaylistTracks(ctx, spotifyClient, mapping.SpotifyPlaylistID)
		if err != nil {
			return err
		}

		youtubeTracks, err = j.getYouTubePlaylistTracks(ctx, youtubeService, mapping.YoutubePlaylistID)
		if err != nil {
			return err
		}
	}

	syncItems := j.generateSyncItems(mapping, spotifyTracks, youtubeTracks)

	if len(syncItems) > 0 {
		err := j.insertSyncItems(ctx, syncItems)
		if err != nil {
			return err
		}
	}

	err := j.updateMappingAnalysisTimestamp(ctx, mappingID, len(spotifyTracks), len(youtubeTracks))
	if err != nil {
		return err
	}

	j.activityLogger.RecordInfo("Analysis completed, found "+strconv.Itoa(len(syncItems))+" differences", mappingID, "analysis")
	j.logger.Info().Str("mapping_id", mappingID).Int("sync_items", len(syncItems)).Msg("mapping analysis completed")

	return nil
}

func (j *AnalysisJob) getSpotifyPlaylistTracks(ctx context.Context, client *spotify.Client, playlistID string) ([]TrackInfo, error) {
	var tracks []TrackInfo
	offset := 0

	for {
		page, err := client.GetPlaylistItems(ctx, spotify.ID(playlistID), spotify.Offset(offset), spotify.Limit(100))
		if err != nil {
			return nil, err
		}

		for _, item := range page.Items {
			if item.Track.Track == nil {
				continue
			}
			track := item.Track.Track
			artist := ""
			if len(track.Artists) > 0 {
				artist = track.Artists[0].Name
			}
			tracks = append(tracks, TrackInfo{
				ID:     string(track.ID),
				Title:  track.Name,
				Artist: artist,
			})
		}

		if page.Next == "" || len(page.Items) == 0 {
			break
		}
		offset += len(page.Items)
	}

	return tracks, nil
}

func (j *AnalysisJob) getYouTubePlaylistTracks(ctx context.Context, service *youtube.Service, playlistID string) ([]TrackInfo, error) {
	var tracks []TrackInfo
	pageToken := ""

	for {
		call := service.PlaylistItems.List([]string{"snippet", "contentDetails"}).
			PlaylistId(playlistID).
			MaxResults(50)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}

		response, err := call.Do()
		if err != nil {
			return nil, err
		}

		for _, item := range response.Items {
			videoID := ""
			if item.ContentDetails != nil {
				videoID = item.ContentDetails.VideoId
			}
			title := ""
			if item.Snippet != nil {
				title = item.Snippet.Title
			}
			if videoID == "" {
				continue
			}
			tracks = append(tracks, TrackInfo{
				ID:     videoID,
				Title:  title,
				Artist: "",
			})
		}

		pageToken = response.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return tracks, nil
}

func (j *AnalysisJob) generateSyncItems(mapping model.Mappings, spotifyTracks, youtubeTracks []TrackInfo) []model.SyncItems {
	var syncItems []model.SyncItems
	now := time.Now().Unix()
	mappingID := lo.FromPtr(mapping.ID)

	if mapping.SyncName != 0 {
		spotifyName := strings.TrimSpace(lo.FromPtr(mapping.SpotifyPlaylistName))
		youtubeName := strings.TrimSpace(lo.FromPtr(mapping.YoutubePlaylistName))
		if spotifyName != "" && youtubeName != "" && !strings.EqualFold(spotifyName, youtubeName) {
			// Spotify playlist name is the source of truth when titles differ.
			syncItems = append(syncItems, model.SyncItems{
				ID:           stringPtr(uuid.NewString()),
				MappingID:    mappingID,
				Operation:    "rename",
				Service:      "youtube",
				TrackID:      stringPtr(mapping.YoutubePlaylistID),
				TrackTitle:   stringPtr(spotifyName),
				Status:       "pending",
				AttemptCount: 0,
				Created:      int32(now),
				Updated:      int32(now),
			})
		}
	}

	if mapping.SyncTracks == 0 {
		return syncItems
	}

	for _, track := range spotifyTracks {
		if !j.containsTrack(youtubeTracks, track) {
			syncItems = append(syncItems, model.SyncItems{
				ID:           stringPtr(uuid.NewString()),
				MappingID:    mappingID,
				Operation:    "add",
				Service:      "youtube",
				TrackID:      stringPtr(track.ID),
				TrackTitle:   stringPtr(track.Title),
				TrackArtist:  stringPtr(track.Artist),
				Status:       "pending",
				AttemptCount: 0,
				Created:      int32(now),
				Updated:      int32(now),
			})
		}
	}

	for _, track := range youtubeTracks {
		if !j.containsTrack(spotifyTracks, track) {
			syncItems = append(syncItems, model.SyncItems{
				ID:           stringPtr(uuid.NewString()),
				MappingID:    mappingID,
				Operation:    "add",
				Service:      "spotify",
				TrackID:      stringPtr(track.ID),
				TrackTitle:   stringPtr(track.Title),
				TrackArtist:  stringPtr(track.Artist),
				Status:       "pending",
				AttemptCount: 0,
				Created:      int32(now),
				Updated:      int32(now),
			})
		}
	}

	return syncItems
}

func (j *AnalysisJob) containsTrack(tracks []TrackInfo, target TrackInfo) bool {
	for _, track := range tracks {
		if track.ID != "" && target.ID != "" && track.ID == target.ID {
			return true
		}
		if strings.EqualFold(track.Title, target.Title) &&
			strings.EqualFold(track.Artist, target.Artist) {
			return true
		}
	}
	return false
}

func (j *AnalysisJob) insertSyncItems(ctx context.Context, syncItems []model.SyncItems) error {
	for _, item := range syncItems {
		_, err := table.SyncItems.
			INSERT(table.SyncItems.AllColumns).
			MODEL(item).
			Exec(j.db)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				continue
			}
			return err
		}
	}
	return nil
}

func (j *AnalysisJob) updateMappingAnalysisTimestamp(ctx context.Context, mappingID string, spotifyCount, youtubeCount int) error {
	now := time.Now().Unix()
	tracksCount := spotifyCount
	if youtubeCount > tracksCount {
		tracksCount = youtubeCount
	}
	_, err := j.db.Exec(
		"UPDATE mappings SET last_analysis_at = ?, tracks_count = ?, updated = ? WHERE id = ?",
		now, tracksCount, now, mappingID,
	)
	return err
}

// TrackInfo represents basic track information for comparison.
type TrackInfo struct {
	ID     string
	Title  string
	Artist string
}

func stringPtr(s string) *string {
	return &s
}
