package jobs

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
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
		WHERE(
			table.Mappings.LastAnalysisAt.IS_NULL().
				OR(table.Mappings.LastAnalysisAt.LT(sqlite.Int(now-int64(60*60)))), // Older than 1 hour default
		).
		Query(j.db, &mappings)

	return mappings, err
}

func (j *AnalysisJob) analyzeMapping(ctx context.Context, mapping model.Mappings) error {
	mappingID := lo.FromPtr(mapping.ID)
	j.logger.Info().Str("mapping_id", mappingID).Msg("analyzing mapping")

	// Get Spotify and YouTube clients
	spotifyClient, err := j.clientFactory.GetSpotifyClient(ctx)
	if err != nil {
		return err
	}

	youtubeService, err := j.clientFactory.GetYouTubeService(ctx)
	if err != nil {
		return err
	}

	// Fetch playlists from both services
	spotifyTracks, err := j.getSpotifyPlaylistTracks(ctx, spotifyClient, mapping.SpotifyPlaylistID)
	if err != nil {
		return err
	}

	youtubeTracks, err := j.getYouTubePlaylistTracks(ctx, youtubeService, mapping.YoutubePlaylistID)
	if err != nil {
		return err
	}

	// Compare and generate sync items
	syncItems := j.generateSyncItems(mappingID, spotifyTracks, youtubeTracks)

	// Insert sync items
	if len(syncItems) > 0 {
		err = j.insertSyncItems(ctx, syncItems)
		if err != nil {
			return err
		}
	}

	// Update mapping timestamp
	err = j.updateMappingAnalysisTimestamp(ctx, mappingID)
	if err != nil {
		return err
	}

	j.activityLogger.RecordInfo("Analysis completed, found "+string(rune(len(syncItems)))+" differences", mappingID, "analysis")
	j.logger.Info().Str("mapping_id", mappingID).Int("sync_items", len(syncItems)).Msg("mapping analysis completed")

	return nil
}

func (j *AnalysisJob) getSpotifyPlaylistTracks(ctx context.Context, client *spotify.Client, playlistID string) ([]TrackInfo, error) {
	// TODO: Implement Spotify playlist fetching with proper pagination
	// For now, return empty slice as placeholder
	return []TrackInfo{}, nil
}

func (j *AnalysisJob) getYouTubePlaylistTracks(ctx context.Context, service *youtube.Service, playlistID string) ([]TrackInfo, error) {
	// TODO: Implement YouTube playlist fetching with proper pagination
	// For now, return empty slice as placeholder
	return []TrackInfo{}, nil
}

func (j *AnalysisJob) generateSyncItems(mappingID string, spotifyTracks, youtubeTracks []TrackInfo) []model.SyncItems {
	var syncItems []model.SyncItems
	now := time.Now().Unix()

	// Find tracks in Spotify but not YouTube -> add to YouTube
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

	// Find tracks in YouTube but not Spotify -> add to Spotify
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
		if track.ID == target.ID {
			return true
		}
		// Also check by title/artist similarity (basic matching)
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
			// Skip duplicates (unique constraint violation)
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				continue
			}
			return err
		}
	}
	return nil
}

func (j *AnalysisJob) updateMappingAnalysisTimestamp(ctx context.Context, mappingID string) error {
	now := time.Now().Unix()
	_, err := j.db.Exec("UPDATE mappings SET last_analysis_at = ?, updated = ? WHERE id = ?", now, now, mappingID)
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
