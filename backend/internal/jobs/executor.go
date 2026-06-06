package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/rs/zerolog"
	spotify "github.com/zmb3/spotify/v2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/youtube/v3"

	"github.com/manlikeabro/spotube/internal/activitylogger"
	"github.com/manlikeabro/spotube/internal/auth"
	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/db/table"
	"github.com/manlikeabro/spotube/internal/matching"
	"github.com/manlikeabro/spotube/internal/synccontext"
)

const (
	maxErrorMessageLen  = 512
	executorSearchLimit = 5
)

// ErrSyncItemNotFound is returned when a sync item id does not exist.
var ErrSyncItemNotFound = errors.New("sync item not found")

// ErrSyncItemNotExecutable is returned when status is not pending or error.
var ErrSyncItemNotExecutable = errors.New("sync item is not executable")

// ErrSyncItemBlacklisted is returned when the track is on the mapping blacklist.
var ErrSyncItemBlacklisted = errors.New("track blacklisted")

// ErrSyncItemAlreadyInDestination is returned when the destination playlist already contains an equivalent track.
var ErrSyncItemAlreadyInDestination = errors.New("track already in destination playlist")

// Executor applies a single sync_items row to Spotify or YouTube.
type Executor struct {
	db             *sql.DB
	logger         zerolog.Logger
	activityLogger *activitylogger.Logger
	clientFactory  *auth.ClientFactory
	spotify        spotifyPlatform
	youtube        youtubePlatform
}

// NewExecutor creates an executor that uses live API clients from clientFactory.
func NewExecutor(deps JobDeps, clientFactory *auth.ClientFactory) *Executor {
	e := &Executor{
		db:             deps.DB,
		logger:         deps.Logger,
		activityLogger: deps.ActivityLogger,
		clientFactory:  clientFactory,
	}
	e.spotify = &liveSpotifyPlatform{factory: clientFactory}
	e.youtube = &liveYouTubePlatform{factory: clientFactory}
	return e
}

// ExecuteOne runs executor logic for a single sync item id.
func (e *Executor) ExecuteOne(ctx context.Context, itemID string) (model.SyncItems, error) {
	item, mapping, err := e.loadItemWithMapping(ctx, itemID)
	if err != nil {
		return model.SyncItems{}, err
	}

	mappingID := item.MappingID
	if item.TrackID != nil && strings.TrimSpace(*item.TrackID) != "" {
		blacklisted, err := e.isBlacklisted(mappingID, item.Service, *item.TrackID)
		if err != nil {
			return model.SyncItems{}, err
		}
		if blacklisted {
			updated, skipErr := e.markSkipped(itemID, "track blacklisted")
			if skipErr != nil {
				return model.SyncItems{}, skipErr
			}
			return updated, ErrSyncItemBlacklisted
		}
	}

	claimed, err := e.claimForExecution(itemID)
	if err != nil {
		return model.SyncItems{}, err
	}
	if !claimed {
		return model.SyncItems{}, ErrSyncItemNotExecutable
	}

	run, execErr := e.runOperation(ctx, item, mapping)
	run.SyncItemID = itemID
	e.recordExecutorLog(mappingID, itemID, run, execErr)

	if errors.Is(execErr, ErrAlreadyInDestination) {
		updated, skipErr := e.markSkipped(itemID, "already in destination playlist")
		if skipErr != nil {
			return model.SyncItems{}, skipErr
		}
		return updated, ErrSyncItemAlreadyInDestination
	}

	return e.finalizeExecution(ctx, itemID, mappingID, item, execErr)
}

func (e *Executor) loadItemWithMapping(ctx context.Context, itemID string) (model.SyncItems, model.Mappings, error) {
	_ = ctx

	var items []model.SyncItems
	err := table.SyncItems.
		SELECT(table.SyncItems.AllColumns).
		WHERE(table.SyncItems.ID.EQ(sqlite.String(itemID))).
		LIMIT(1).
		Query(e.db, &items)
	if err != nil {
		return model.SyncItems{}, model.Mappings{}, err
	}
	if len(items) == 0 {
		return model.SyncItems{}, model.Mappings{}, ErrSyncItemNotFound
	}

	item := items[0]
	if item.Status != "pending" && item.Status != "error" {
		return model.SyncItems{}, model.Mappings{}, ErrSyncItemNotExecutable
	}

	var mappings []model.Mappings
	err = table.Mappings.
		SELECT(table.Mappings.AllColumns).
		WHERE(table.Mappings.ID.EQ(sqlite.String(item.MappingID))).
		LIMIT(1).
		Query(e.db, &mappings)
	if err != nil {
		return model.SyncItems{}, model.Mappings{}, err
	}
	if len(mappings) == 0 {
		return model.SyncItems{}, model.Mappings{}, fmt.Errorf("mapping not found for sync item")
	}

	return item, mappings[0], nil
}

func (e *Executor) isBlacklisted(mappingID, service, trackID string) (bool, error) {
	var entries []model.Blacklist
	err := table.Blacklist.
		SELECT(table.Blacklist.ID).
		WHERE(
			table.Blacklist.MappingID.EQ(sqlite.String(mappingID)).
				AND(table.Blacklist.Service.EQ(sqlite.String(service))).
				AND(table.Blacklist.TrackID.EQ(sqlite.String(trackID))),
		).
		LIMIT(1).
		Query(e.db, &entries)
	if err != nil {
		return false, err
	}
	return len(entries) > 0, nil
}

func (e *Executor) claimForExecution(itemID string) (bool, error) {
	now := time.Now().Unix()
	result, err := e.db.Exec(
		`UPDATE sync_items SET status = 'running', updated = ?, last_attempt_at = ? WHERE id = ? AND status IN ('pending', 'error')`,
		now, now, itemID,
	)
	if err != nil {
		return false, err
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

func (e *Executor) markSkipped(itemID, message string) (model.SyncItems, error) {
	now := time.Now().Unix()
	_, err := e.db.Exec(
		`UPDATE sync_items SET status = 'skipped', error_message = ?, updated = ?, last_attempt_at = ? WHERE id = ?`,
		truncateExecutorError(message), now, now, itemID,
	)
	if err != nil {
		return model.SyncItems{}, err
	}
	return e.reloadItem(itemID)
}

func (e *Executor) runOperation(ctx context.Context, item model.SyncItems, mapping model.Mappings) (synccontext.ExecutorRun, error) {
	switch item.Operation {
	case "add":
		return e.executeAdd(ctx, item, mapping)
	case "rename":
		return e.executeRename(ctx, item, mapping)
	default:
		return synccontext.ExecutorRun{}, fmt.Errorf("unsupported operation: %s", item.Operation)
	}
}

var ErrAlreadyInDestination = errors.New("already in destination playlist")

func (e *Executor) executeAdd(ctx context.Context, item model.SyncItems, mapping model.Mappings) (synccontext.ExecutorRun, error) {
	title := strings.TrimSpace(stringFromPtr(item.TrackTitle))
	artist := strings.TrimSpace(stringFromPtr(item.TrackArtist))
	query := strings.TrimSpace(title + " " + artist)

	run := synccontext.ExecutorRun{
		Operation:   "add",
		SearchQuery: query,
	}

	switch item.Service {
	case "youtube":
		source := matching.SpotifyTrack(stringFromPtr(item.TrackID), title, artist)
		run.Source = source
		run.DestPlaylistID = mapping.YoutubePlaylistID

		destTracks, err := e.youtube.ListPlaylistTracks(ctx, mapping.YoutubePlaylistID)
		if err != nil {
			run.Outcome = "error"
			run.Error = err.Error()
			return run, err
		}
		playlistMatch := matching.FindMatch(source, destTracks)
		run.PlaylistMatch = &playlistMatch
		if playlistMatch.Matched {
			run.AlreadyInPlaylist = true
			run.Outcome = "skipped_duplicate"
			return run, ErrAlreadyInDestination
		}

		hits, err := e.youtube.SearchVideos(ctx, title, artist, executorSearchLimit)
		if err != nil {
			run.Outcome = "error"
			run.Error = err.Error()
			return run, err
		}
		selected, candidates, pickErr := pickSearchCandidate(source, hits, matching.PlatformYouTube)
		run.Candidates = candidates
		if pickErr != nil {
			run.Outcome = "no_match"
			run.Error = pickErr.Error()
			return run, pickErr
		}
		run.Selected = &selected

		if playlistContainsID(destTracks, selected.ID) {
			run.AlreadyInPlaylist = true
			run.Outcome = "skipped_duplicate"
			return run, ErrAlreadyInDestination
		}

		if err := e.youtube.AddVideoToPlaylist(ctx, mapping.YoutubePlaylistID, selected.ID); err != nil {
			run.Outcome = "error"
			run.Error = err.Error()
			return run, err
		}
		run.Outcome = "added"
		return run, nil

	case "spotify":
		rawTitle := title
		if artist != "" {
			rawTitle = title + " - " + artist
		}
		source := matching.YouTubeFromRaw(stringFromPtr(item.TrackID), rawTitle)
		run.Source = source
		run.DestPlaylistID = mapping.SpotifyPlaylistID

		destTracks, err := e.spotify.ListPlaylistTracks(ctx, mapping.SpotifyPlaylistID)
		if err != nil {
			run.Outcome = "error"
			run.Error = err.Error()
			return run, err
		}
		playlistMatch := matching.FindMatch(source, destTracks)
		run.PlaylistMatch = &playlistMatch
		if playlistMatch.Matched {
			run.AlreadyInPlaylist = true
			run.Outcome = "skipped_duplicate"
			return run, ErrAlreadyInDestination
		}

		hits, err := e.spotify.SearchTracks(ctx, title, artist, executorSearchLimit)
		if err != nil {
			run.Outcome = "error"
			run.Error = err.Error()
			return run, err
		}
		selected, candidates, pickErr := pickSearchCandidate(source, hits, matching.PlatformSpotify)
		run.Candidates = candidates
		if pickErr != nil {
			run.Outcome = "no_match"
			run.Error = pickErr.Error()
			return run, pickErr
		}
		run.Selected = &selected

		if playlistContainsID(destTracks, selected.ID) {
			run.AlreadyInPlaylist = true
			run.Outcome = "skipped_duplicate"
			return run, ErrAlreadyInDestination
		}

		if err := e.spotify.AddTrackToPlaylist(ctx, mapping.SpotifyPlaylistID, selected.ID); err != nil {
			run.Outcome = "error"
			run.Error = err.Error()
			return run, err
		}
		run.Outcome = "added"
		return run, nil
	default:
		return run, fmt.Errorf("unsupported service: %s", item.Service)
	}
}

func (e *Executor) executeRename(ctx context.Context, item model.SyncItems, mapping model.Mappings) (synccontext.ExecutorRun, error) {
	newTitle := strings.TrimSpace(stringFromPtr(item.TrackTitle))
	run := synccontext.ExecutorRun{
		Operation: "rename",
		Source: matching.Track{
			Title: newTitle,
		},
	}

	if newTitle == "" {
		run.Outcome = "error"
		run.Error = "rename requires target title"
		return run, errors.New(run.Error)
	}

	var err error
	switch item.Service {
	case "youtube":
		run.DestPlaylistID = mapping.YoutubePlaylistID
		err = e.youtube.RenamePlaylist(ctx, mapping.YoutubePlaylistID, newTitle)
	case "spotify":
		run.DestPlaylistID = mapping.SpotifyPlaylistID
		err = e.spotify.RenamePlaylist(ctx, mapping.SpotifyPlaylistID, newTitle)
	default:
		run.Outcome = "error"
		run.Error = fmt.Sprintf("unsupported service: %s", item.Service)
		return run, fmt.Errorf("%s", run.Error)
	}
	if err != nil {
		run.Outcome = "error"
		run.Error = err.Error()
		return run, err
	}
	run.Outcome = "renamed"
	return run, nil
}

func pickSearchCandidate(source matching.Track, hits []platformSearchHit, platform matching.Platform) (matching.Track, []synccontext.SearchCandidate, error) {
	if len(hits) == 0 {
		return matching.Track{}, nil, fmt.Errorf("no match on %s", platform)
	}

	candidates := make([]synccontext.SearchCandidate, 0, len(hits))
	var best matching.Track
	bestScore := 0.0
	for i, hit := range hits {
		candidate := matching.Track{
			Platform: platform,
			ID:       hit.ID,
			Title:    hit.Title,
			Artist:   hit.Artist,
		}
		score := matching.ScorePair(source, candidate)
		candidates = append(candidates, synccontext.SearchCandidate{
			Rank:    i + 1,
			ID:      hit.ID,
			Title:   hit.Title,
			Artist:  hit.Artist,
			Channel: hit.Channel,
			Score:   score,
		})
		if score > bestScore {
			bestScore = score
			best = candidate
		}
	}
	if bestScore < matching.MatchThreshold {
		return matching.Track{}, candidates, fmt.Errorf("no confident match on %s (best score %.2f)", platform, bestScore)
	}
	return best, candidates, nil
}

func playlistContainsID(tracks []matching.Track, id string) bool {
	for _, track := range tracks {
		if track.ID == id {
			return true
		}
	}
	return false
}

func (e *Executor) recordExecutorLog(mappingID, itemID string, run synccontext.ExecutorRun, execErr error) {
	level := "info"
	message := synccontext.BriefExecutorLine(run)
	if execErr != nil && !errors.Is(execErr, ErrAlreadyInDestination) {
		level = "error"
		if run.Error != "" {
			message = message + ": " + run.Error
		} else {
			message = message + ": " + execErr.Error()
		}
	}
	details := synccontext.Envelope{
		Version: synccontext.Version,
		Kind:    "executor_run",
		Data:    run,
	}
	_ = e.activityLogger.RecordWithDetails(level, message, mappingID, "executor", itemID, details)
	e.logger.Info().Str("sync_item_id", itemID).Str("mapping_id", mappingID).Msg(message)
}

func (e *Executor) finalizeExecution(ctx context.Context, itemID, mappingID string, item model.SyncItems, execErr error) (model.SyncItems, error) {
	_ = ctx
	now := time.Now().Unix()
	attemptCount := int(item.AttemptCount) + 1

	status := "done"
	errMsg := ""
	if execErr != nil {
		if isIdempotentSuccess(execErr) {
			execErr = nil
		}
	}
	if execErr != nil {
		status = "error"
		errMsg = truncateExecutorError(execErr.Error())
	}

	_, err := e.db.Exec(
		`UPDATE sync_items SET status = ?, error_message = ?, attempt_count = ?, updated = ?, last_attempt_at = ? WHERE id = ?`,
		status, nullableErrorMessage(errMsg), attemptCount, now, now, itemID,
	)
	if err != nil {
		return model.SyncItems{}, err
	}

	updated, err := e.reloadItem(itemID)
	if err != nil {
		return model.SyncItems{}, err
	}
	if execErr != nil {
		return updated, execErr
	}
	return updated, nil
}

func (e *Executor) reloadItem(itemID string) (model.SyncItems, error) {
	var items []model.SyncItems
	err := table.SyncItems.
		SELECT(table.SyncItems.AllColumns).
		WHERE(table.SyncItems.ID.EQ(sqlite.String(itemID))).
		LIMIT(1).
		Query(e.db, &items)
	if err != nil {
		return model.SyncItems{}, err
	}
	if len(items) == 0 {
		return model.SyncItems{}, ErrSyncItemNotFound
	}
	return items[0], nil
}

func isIdempotentSuccess(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "already") {
		return true
	}
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		if gerr.Code == 400 || gerr.Code == 409 {
			return true
		}
	}
	return false
}

func truncateExecutorError(msg string) string {
	msg = strings.TrimSpace(msg)
	if len(msg) <= maxErrorMessageLen {
		return msg
	}
	return msg[:maxErrorMessageLen]
}

func nullableErrorMessage(msg string) interface{} {
	if msg == "" {
		return nil
	}
	return msg
}

func stringFromPtr(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

type platformSearchHit struct {
	ID      string
	Title   string
	Artist  string
	Channel string
}

type spotifyPlatform interface {
	SearchTracks(ctx context.Context, title, artist string, limit int) ([]platformSearchHit, error)
	ListPlaylistTracks(ctx context.Context, playlistID string) ([]matching.Track, error)
	AddTrackToPlaylist(ctx context.Context, playlistID, trackID string) error
	RenamePlaylist(ctx context.Context, playlistID, name string) error
}

type youtubePlatform interface {
	SearchVideos(ctx context.Context, title, artist string, limit int) ([]platformSearchHit, error)
	ListPlaylistTracks(ctx context.Context, playlistID string) ([]matching.Track, error)
	AddVideoToPlaylist(ctx context.Context, playlistID, videoID string) error
	RenamePlaylist(ctx context.Context, playlistID, title string) error
}

type liveSpotifyPlatform struct {
	factory *auth.ClientFactory
}

func (p *liveSpotifyPlatform) SearchTracks(ctx context.Context, title, artist string, limit int) ([]platformSearchHit, error) {
	client, err := p.factory.GetSpotifyClient(ctx)
	if err != nil {
		return nil, err
	}
	query := strings.TrimSpace(title + " " + artist)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 1
	}
	result, err := client.Search(ctx, query, spotify.SearchTypeTrack, spotify.Limit(limit))
	if err != nil {
		return nil, err
	}
	if result.Tracks == nil || len(result.Tracks.Tracks) == 0 {
		return nil, nil
	}
	hits := make([]platformSearchHit, 0, len(result.Tracks.Tracks))
	for _, track := range result.Tracks.Tracks {
		artistName := ""
		if len(track.Artists) > 0 {
			artistName = track.Artists[0].Name
		}
		album := ""
		if track.Album.Name != "" {
			album = track.Album.Name
		}
		hits = append(hits, platformSearchHit{
			ID:      string(track.ID),
			Title:   track.Name,
			Artist:  artistName,
			Channel: album,
		})
	}
	return hits, nil
}

func (p *liveSpotifyPlatform) ListPlaylistTracks(ctx context.Context, playlistID string) ([]matching.Track, error) {
	client, err := p.factory.GetSpotifyClient(ctx)
	if err != nil {
		return nil, err
	}
	var tracks []matching.Track
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
			tracks = append(tracks, matching.SpotifyTrack(string(track.ID), track.Name, artist))
		}
		if page.Next == "" || len(page.Items) == 0 {
			break
		}
		offset += len(page.Items)
	}
	return tracks, nil
}

func (p *liveSpotifyPlatform) AddTrackToPlaylist(ctx context.Context, playlistID, trackID string) error {
	client, err := p.factory.GetSpotifyClient(ctx)
	if err != nil {
		return err
	}
	_, err = client.AddTracksToPlaylist(ctx, spotify.ID(playlistID), spotify.ID(trackID))
	return err
}

func (p *liveSpotifyPlatform) RenamePlaylist(ctx context.Context, playlistID, name string) error {
	client, err := p.factory.GetSpotifyClient(ctx)
	if err != nil {
		return err
	}
	return client.ChangePlaylistName(ctx, spotify.ID(playlistID), name)
}

type liveYouTubePlatform struct {
	factory *auth.ClientFactory
}

func (p *liveYouTubePlatform) SearchVideos(ctx context.Context, title, artist string, limit int) ([]platformSearchHit, error) {
	service, err := p.factory.GetYouTubeService(ctx)
	if err != nil {
		return nil, err
	}
	query := strings.TrimSpace(title + " " + artist)
	if query == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 1
	}
	call := service.Search.List([]string{"snippet"}).Q(query).Type("video").MaxResults(int64(limit))
	response, err := call.Do()
	if err != nil {
		return nil, err
	}
	hits := make([]platformSearchHit, 0, len(response.Items))
	for _, item := range response.Items {
		if item.Id == nil || item.Snippet == nil {
			continue
		}
		videoID := item.Id.VideoId
		if videoID == "" {
			continue
		}
		parsed := matching.YouTubeFromRaw(videoID, item.Snippet.Title)
		channel := ""
		if item.Snippet.ChannelTitle != "" {
			channel = item.Snippet.ChannelTitle
		}
		hits = append(hits, platformSearchHit{
			ID:      videoID,
			Title:   parsed.Title,
			Artist:  parsed.Artist,
			Channel: channel,
		})
	}
	return hits, nil
}

func (p *liveYouTubePlatform) ListPlaylistTracks(ctx context.Context, playlistID string) ([]matching.Track, error) {
	service, err := p.factory.GetYouTubeService(ctx)
	if err != nil {
		return nil, err
	}
	var tracks []matching.Track
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
			rawTitle := ""
			if item.Snippet != nil {
				rawTitle = item.Snippet.Title
			}
			if videoID == "" {
				continue
			}
			tracks = append(tracks, matching.YouTubeFromRaw(videoID, rawTitle))
		}
		pageToken = response.NextPageToken
		if pageToken == "" {
			break
		}
	}
	return tracks, nil
}

func (p *liveYouTubePlatform) AddVideoToPlaylist(ctx context.Context, playlistID, videoID string) error {
	service, err := p.factory.GetYouTubeService(ctx)
	if err != nil {
		return err
	}
	item := &youtube.PlaylistItem{
		Snippet: &youtube.PlaylistItemSnippet{
			PlaylistId: playlistID,
			ResourceId: &youtube.ResourceId{
				Kind:    "youtube#video",
				VideoId: videoID,
			},
		},
	}
	_, err = service.PlaylistItems.Insert([]string{"snippet"}, item).Do()
	return err
}

func (p *liveYouTubePlatform) RenamePlaylist(ctx context.Context, playlistID, title string) error {
	service, err := p.factory.GetYouTubeService(ctx)
	if err != nil {
		return err
	}
	playlist := &youtube.Playlist{
		Id: playlistID,
		Snippet: &youtube.PlaylistSnippet{
			Title: title,
		},
	}
	_, err = service.Playlists.Update([]string{"snippet"}, playlist).Do()
	return err
}
