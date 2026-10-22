package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
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
	"github.com/manlikeabro/spotube/internal/matching"
	"github.com/manlikeabro/spotube/internal/synccontext"
)

// AnalysisJob compares mapped playlists, queues sync items, and (optionally) applies them.
type AnalysisJob struct {
	db             *sql.DB
	logger         zerolog.Logger
	activityLogger *activitylogger.Logger
	clientFactory  *auth.ClientFactory

	// applier applies queued items right after analysis; nil disables auto-apply.
	applier    *Executor
	applyLimit int
}

func NewAnalysisJob(deps JobDeps, clientFactory *auth.ClientFactory) *AnalysisJob {
	return &AnalysisJob{
		db:             deps.DB,
		logger:         deps.Logger,
		activityLogger: deps.ActivityLogger,
		clientFactory:  clientFactory,
	}
}

// EnableAutoApply makes each run apply up to limit pending items per mapping after analysis.
func (j *AnalysisJob) EnableAutoApply(executor *Executor, limit int) {
	if limit <= 0 {
		limit = defaultApplyLimit
	}
	j.applier = executor
	j.applyLimit = limit
}

// defaultApplyLimit caps items applied per mapping per run. YouTube search costs 100 of the
// default 10,000 daily quota units, so large backlogs drain over several runs.
const defaultApplyLimit = 25

// Run analyzes every mapping that is due and applies its queue when auto-apply is enabled.
func (j *AnalysisJob) Run(ctx context.Context) error {
	mappings, err := j.getMappingsForAnalysis(ctx)
	if err != nil {
		j.logger.Error().Err(err).Msg("failed to get mappings for analysis")
		return err
	}

	if len(mappings) == 0 {
		j.logger.Debug().Msg("no mappings due for sync")
		return nil
	}

	j.logger.Info().Int("count", len(mappings)).Msg("sync run starting")

	for _, mapping := range mappings {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		j.SyncMapping(ctx, mapping)
	}

	j.logger.Info().Msg("sync run completed")
	return nil
}

// SyncMapping analyzes one mapping, applies its queue when enabled, and records the outcome.
func (j *AnalysisJob) SyncMapping(ctx context.Context, mapping model.Mappings) {
	mappingID := lo.FromPtr(mapping.ID)

	snapshot, err := j.analyzeMapping(ctx, mapping)
	if err == nil && j.applier != nil && mapping.SyncTracks != 0 {
		var batch BatchResult
		batch, err = j.applier.ExecutePending(ctx, mappingID, j.applyLimit, snapshot)
		if batch.Attempted > 0 {
			j.activityLogger.RecordInfo(batch.Summary(), mappingID, "executor")
		}
	}

	status, errMsg := "ok", ""
	if err != nil {
		status, errMsg = "error", truncateExecutorError(err.Error())
		if errors.Is(err, auth.ErrReconnectRequired) || errors.Is(err, auth.ErrNoTokenFound) {
			status = "reconnect_required"
		}
		j.logger.Error().Err(err).Str("mapping_id", mappingID).Msg("mapping sync failed")
		j.activityLogger.RecordError("Sync failed: "+err.Error(), mappingID, "analysis")
	}
	// Stamp the run even on failure so a broken mapping waits for its interval instead of
	// retrying (and logging an error) on every cron tick.
	if stampErr := j.recordSyncResult(mappingID, status, errMsg); stampErr != nil {
		j.logger.Error().Err(stampErr).Str("mapping_id", mappingID).Msg("failed to record sync result")
	}
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

func (j *AnalysisJob) analyzeMapping(ctx context.Context, mapping model.Mappings) (*PlaylistSnapshot, error) {
	mappingID := lo.FromPtr(mapping.ID)
	j.logger.Info().Str("mapping_id", mappingID).Msg("analyzing mapping")

	spotifyTracks := []TrackInfo{}
	youtubeTracks := []TrackInfo{}

	if mapping.SyncTracks == 0 && mapping.SyncName == 0 {
		return nil, nil
	}

	spotifyClient, err := j.clientFactory.GetSpotifyClient(ctx)
	if err != nil {
		return nil, err
	}
	youtubeService, err := j.clientFactory.GetYouTubeService(ctx)
	if err != nil {
		return nil, err
	}

	if mapping.SyncName != 0 {
		if err := j.refreshMappingPlaylistNames(ctx, &mapping, spotifyClient, youtubeService); err != nil {
			return nil, err
		}
	}

	if mapping.SyncTracks != 0 {
		spotifyTracks, err = j.getSpotifyPlaylistTracks(ctx, spotifyClient, mapping.SpotifyPlaylistID)
		if err != nil {
			return nil, err
		}
		youtubeTracks, err = j.getYouTubePlaylistTracks(ctx, youtubeService, mapping.YoutubePlaylistID)
		if err != nil {
			return nil, err
		}
	}

	result := j.generateSyncItems(mapping, spotifyTracks, youtubeTracks)

	inserted := 0
	if len(result.items) > 0 {
		inserted, err = j.insertSyncItems(ctx, result.items)
		if err != nil {
			return nil, err
		}
	}

	if err := j.updateMappingAnalysisTimestamp(ctx, mappingID, len(spotifyTracks), len(youtubeTracks)); err != nil {
		return nil, err
	}

	alreadyQueued := len(result.items) - inserted
	msg := "Analysis completed, found " + strconv.Itoa(inserted) + " sync items"
	if alreadyQueued > 0 {
		msg += " (" + strconv.Itoa(alreadyQueued) + " already queued)"
	}
	summary := synccontext.Envelope{
		Version: synccontext.Version,
		Kind:    "analysis_run",
		Data: synccontext.AnalysisRun{
			MappingID:      mappingID,
			SpotifyCount:   len(spotifyTracks),
			YoutubeCount:   len(youtubeTracks),
			QueuedAdds:     inserted,
			MatchedSkipped: result.matchedSkipped,
			AlreadyQueued:  alreadyQueued,
		},
	}
	_ = j.activityLogger.RecordWithDetails("info", msg, mappingID, "analysis", "", summary)
	j.logger.Info().
		Str("mapping_id", mappingID).
		Int("generated", len(result.items)).
		Int("inserted", inserted).
		Int("matched_skipped", result.matchedSkipped).
		Msg("mapping analysis completed")

	snapshot := &PlaylistSnapshot{}
	if mapping.SyncTracks != 0 {
		snapshot.Spotify = map[string][]matching.Track{mapping.SpotifyPlaylistID: trackInfosToSpotifyMatching(spotifyTracks)}
		snapshot.YouTube = map[string][]matching.Track{mapping.YoutubePlaylistID: trackInfosToYouTubeMatching(youtubeTracks)}
	}
	return snapshot, nil
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
			if videoID == "" || item.Snippet == nil {
				continue
			}
			tracks = append(tracks, TrackInfo{
				ID:       videoID,
				Title:    item.Snippet.Title,
				RawTitle: item.Snippet.Title,
				Channel:  item.Snippet.VideoOwnerChannelTitle,
			})
		}

		pageToken = response.NextPageToken
		if pageToken == "" {
			break
		}
	}

	return tracks, nil
}

type syncItemsGenerateResult struct {
	items          []model.SyncItems
	matchedSkipped int
}

func (j *AnalysisJob) generateSyncItems(mapping model.Mappings, spotifyTracks, youtubeTracks []TrackInfo) syncItemsGenerateResult {
	var syncItems []model.SyncItems
	matchedSkipped := 0
	now := time.Now().Unix()
	mappingID := lo.FromPtr(mapping.ID)

	youtubeMatching := trackInfosToYouTubeMatching(youtubeTracks)
	spotifyMatching := trackInfosToSpotifyMatching(spotifyTracks)

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
				TrackID:      stringPtr(renameTrackIDKey),
				TrackTitle:   stringPtr(spotifyName),
				Status:       "pending",
				AttemptCount: 0,
				Created:      int32(now),
				Updated:      int32(now),
			})
		}
	}

	if mapping.SyncTracks == 0 {
		return syncItemsGenerateResult{items: syncItems}
	}

	for _, track := range spotifyTracks {
		source := matching.SpotifyTrack(track.ID, track.Title, track.Artist)
		decision := matching.FindMatch(source, youtubeMatching)
		if decision.Matched {
			matchedSkipped++
			j.logger.Debug().Str("mapping_id", mappingID).Msg(matching.BriefAnalysisLine(decision, "youtube", false))
			continue
		}
		decision.QueuedAdd = true
		contextJSON := marshalAnalysisContext(decision)
		syncItems = append(syncItems, model.SyncItems{
			ID:                  stringPtr(uuid.NewString()),
			MappingID:           mappingID,
			Operation:           "add",
			Service:             "youtube",
			TrackID:             stringPtr(track.ID),
			TrackTitle:          stringPtr(track.Title),
			TrackArtist:         stringPtr(track.Artist),
			Status:              "pending",
			AttemptCount:        0,
			Created:             int32(now),
			Updated:             int32(now),
			AnalysisContextJSON: contextJSON,
		})
		j.logger.Info().Str("mapping_id", mappingID).Msg(matching.BriefAnalysisLine(decision, "youtube", true))
	}

	for _, track := range youtubeTracks {
		rawTitle := track.RawTitle
		if rawTitle == "" {
			rawTitle = track.Title
		}
		source := matching.YouTubeTrack(track.ID, rawTitle, track.Channel)
		decision := matching.FindMatch(source, spotifyMatching)
		if decision.Matched {
			matchedSkipped++
			j.logger.Debug().Str("mapping_id", mappingID).Msg(matching.BriefAnalysisLine(decision, "spotify", false))
			continue
		}
		decision.QueuedAdd = true
		contextJSON := marshalAnalysisContext(decision)
		syncItems = append(syncItems, model.SyncItems{
			ID:                  stringPtr(uuid.NewString()),
			MappingID:           mappingID,
			Operation:           "add",
			Service:             "spotify",
			TrackID:             stringPtr(track.ID),
			TrackTitle:          stringPtr(source.Title),
			TrackArtist:         stringPtr(source.Artist),
			Status:              "pending",
			AttemptCount:        0,
			Created:             int32(now),
			Updated:             int32(now),
			AnalysisContextJSON: contextJSON,
		})
		j.logger.Info().Str("mapping_id", mappingID).Msg(matching.BriefAnalysisLine(decision, "spotify", true))
	}

	return syncItemsGenerateResult{items: syncItems, matchedSkipped: matchedSkipped}
}

func trackInfosToYouTubeMatching(tracks []TrackInfo) []matching.Track {
	out := make([]matching.Track, 0, len(tracks))
	for _, track := range tracks {
		raw := track.RawTitle
		if raw == "" {
			raw = track.Title
		}
		out = append(out, matching.YouTubeTrack(track.ID, raw, track.Channel))
	}
	return out
}

func trackInfosToSpotifyMatching(tracks []TrackInfo) []matching.Track {
	out := make([]matching.Track, 0, len(tracks))
	for _, track := range tracks {
		out = append(out, matching.SpotifyTrack(track.ID, track.Title, track.Artist))
	}
	return out
}

func marshalAnalysisContext(decision matching.Decision) *string {
	payload := synccontext.AnalysisItemContext{
		Version:  synccontext.Version,
		Decision: decision,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	s := string(raw)
	return &s
}

func (j *AnalysisJob) insertSyncItems(ctx context.Context, syncItems []model.SyncItems) (int, error) {
	inserted := 0
	for _, item := range syncItems {
		if item.Operation == "rename" {
			now := time.Now().Unix()
			title := lo.FromPtr(item.TrackTitle)
			result, err := j.db.Exec(
				`UPDATE sync_items SET track_title = ?, status = 'pending', error_message = NULL, updated = ?, attempt_count = 0
				 WHERE mapping_id = ? AND operation = 'rename' AND service = ? AND track_id = ?`,
				title, now, item.MappingID, item.Service, renameTrackIDKey,
			)
			if err != nil {
				return inserted, err
			}
			rows, _ := result.RowsAffected()
			if rows > 0 {
				inserted++
				continue
			}
		}

		_, err := table.SyncItems.
			INSERT(table.SyncItems.AllColumns).
			MODEL(item).
			Exec(j.db)
		if err != nil {
			if strings.Contains(err.Error(), "UNIQUE constraint failed") {
				j.logger.Debug().
					Str("mapping_id", item.MappingID).
					Str("operation", item.Operation).
					Str("service", item.Service).
					Str("track_id", lo.FromPtr(item.TrackID)).
					Msg("sync item already queued, skipping")
				continue
			}
			return inserted, err
		}
		inserted++
	}
	return inserted, nil
}

// refreshMappingPlaylistNames loads current playlist titles from both services and stores them,
// so rename detection compares live names rather than whatever was saved at mapping creation.
func (j *AnalysisJob) refreshMappingPlaylistNames(
	ctx context.Context,
	mapping *model.Mappings,
	spotifyClient *spotify.Client,
	youtubeService *youtube.Service,
) error {
	playlist, err := spotifyClient.GetPlaylist(ctx, spotify.ID(mapping.SpotifyPlaylistID), spotify.Fields("name"))
	if err != nil {
		return err
	}
	mapping.SpotifyPlaylistName = stringPtr(strings.TrimSpace(playlist.Name))

	response, err := youtubeService.Playlists.List([]string{"snippet"}).Id(mapping.YoutubePlaylistID).Context(ctx).Do()
	if err != nil {
		return err
	}
	if len(response.Items) > 0 && response.Items[0].Snippet != nil {
		mapping.YoutubePlaylistName = stringPtr(strings.TrimSpace(response.Items[0].Snippet.Title))
	}

	_, err = j.db.Exec(
		"UPDATE mappings SET spotify_playlist_name = ?, youtube_playlist_name = ? WHERE id = ?",
		mapping.SpotifyPlaylistName, mapping.YoutubePlaylistName, lo.FromPtr(mapping.ID),
	)
	return err
}

func (j *AnalysisJob) recordSyncResult(mappingID, status, errMsg string) error {
	now := time.Now().Unix()
	_, err := j.db.Exec(
		"UPDATE mappings SET last_analysis_at = ?, last_sync_status = ?, last_sync_error = ?, updated = ? WHERE id = ?",
		now, status, nullableErrorMessage(errMsg), now, mappingID,
	)
	return err
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

// renameTrackIDKey is the stable track_id for rename sync items (avoids UNIQUE collisions with real track IDs).
const renameTrackIDKey = "__rename__"

// TrackInfo represents basic track information for comparison.
type TrackInfo struct {
	ID       string
	Title    string
	Artist   string
	RawTitle string
	// Channel is the YouTube uploader channel ("Artist - Topic" for YouTube Music tracks).
	Channel string
}

func stringPtr(s string) *string {
	return &s
}
