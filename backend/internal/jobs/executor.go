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
)

const maxErrorMessageLen = 512

// ErrSyncItemNotFound is returned when a sync item id does not exist.
var ErrSyncItemNotFound = errors.New("sync item not found")

// ErrSyncItemNotExecutable is returned when status is not pending or error.
var ErrSyncItemNotExecutable = errors.New("sync item is not executable")

// ErrSyncItemBlacklisted is returned when the track is on the mapping blacklist.
var ErrSyncItemBlacklisted = errors.New("track blacklisted")

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

	execErr := e.runOperation(ctx, item, mapping)
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

func (e *Executor) runOperation(ctx context.Context, item model.SyncItems, mapping model.Mappings) error {
	switch item.Operation {
	case "add":
		return e.executeAdd(ctx, item, mapping)
	case "rename":
		return e.executeRename(ctx, item, mapping)
	default:
		return fmt.Errorf("unsupported operation: %s", item.Operation)
	}
}

func (e *Executor) executeAdd(ctx context.Context, item model.SyncItems, mapping model.Mappings) error {
	title := strings.TrimSpace(stringFromPtr(item.TrackTitle))
	artist := strings.TrimSpace(stringFromPtr(item.TrackArtist))

	switch item.Service {
	case "youtube":
		// track_id is the source (Spotify) id — search YouTube for a matching video.
		videoID, err := e.youtube.SearchVideo(ctx, title, artist)
		if err != nil {
			return err
		}
		if videoID == "" {
			return fmt.Errorf("no match on youtube")
		}
		return e.youtube.AddVideoToPlaylist(ctx, mapping.YoutubePlaylistID, videoID)
	case "spotify":
		// track_id is the source (YouTube) video id — search Spotify for a matching track.
		spotifyTrackID, err := e.spotify.SearchTrack(ctx, title, artist)
		if err != nil {
			return err
		}
		if spotifyTrackID == "" {
			return fmt.Errorf("no match on spotify")
		}
		return e.spotify.AddTrackToPlaylist(ctx, mapping.SpotifyPlaylistID, spotifyTrackID)
	default:
		return fmt.Errorf("unsupported service: %s", item.Service)
	}
}

func (e *Executor) executeRename(ctx context.Context, item model.SyncItems, mapping model.Mappings) error {
	newTitle := strings.TrimSpace(stringFromPtr(item.TrackTitle))
	if newTitle == "" {
		return errors.New("rename requires target title")
	}

	switch item.Service {
	case "youtube":
		playlistID := mapping.YoutubePlaylistID
		if trackID := strings.TrimSpace(stringFromPtr(item.TrackID)); trackID != "" {
			playlistID = trackID
		}
		return e.youtube.RenamePlaylist(ctx, playlistID, newTitle)
	case "spotify":
		playlistID := mapping.SpotifyPlaylistID
		if trackID := strings.TrimSpace(stringFromPtr(item.TrackID)); trackID != "" {
			playlistID = trackID
		}
		return e.spotify.RenamePlaylist(ctx, playlistID, newTitle)
	default:
		return fmt.Errorf("unsupported service: %s", item.Service)
	}
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
		e.activityLogger.RecordError(
			fmt.Sprintf("Executor failed (%s/%s): %s", item.Service, item.Operation, errMsg),
			mappingID,
			"executor",
		)
	} else {
		e.activityLogger.RecordInfo(
			fmt.Sprintf("Executor completed (%s/%s)", item.Service, item.Operation),
			mappingID,
			"executor",
		)
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

// Platform abstractions for testing.

type spotifyPlatform interface {
	SearchTrack(ctx context.Context, title, artist string) (string, error)
	AddTrackToPlaylist(ctx context.Context, playlistID, trackID string) error
	RenamePlaylist(ctx context.Context, playlistID, name string) error
}

type youtubePlatform interface {
	SearchVideo(ctx context.Context, title, artist string) (string, error)
	AddVideoToPlaylist(ctx context.Context, playlistID, videoID string) error
	RenamePlaylist(ctx context.Context, playlistID, title string) error
}

type liveSpotifyPlatform struct {
	factory *auth.ClientFactory
}

func (p *liveSpotifyPlatform) SearchTrack(ctx context.Context, title, artist string) (string, error) {
	client, err := p.factory.GetSpotifyClient(ctx)
	if err != nil {
		return "", err
	}
	query := strings.TrimSpace(title + " " + artist)
	if query == "" {
		return "", nil
	}
	result, err := client.Search(ctx, query, spotify.SearchTypeTrack, spotify.Limit(1))
	if err != nil {
		return "", err
	}
	if result.Tracks == nil || len(result.Tracks.Tracks) == 0 {
		return "", nil
	}
	return string(result.Tracks.Tracks[0].ID), nil
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

func (p *liveYouTubePlatform) SearchVideo(ctx context.Context, title, artist string) (string, error) {
	service, err := p.factory.GetYouTubeService(ctx)
	if err != nil {
		return "", err
	}
	query := strings.TrimSpace(title + " " + artist)
	if query == "" {
		return "", nil
	}
	call := service.Search.List([]string{"id"}).Q(query).Type("video").MaxResults(1)
	response, err := call.Do()
	if err != nil {
		return "", err
	}
	if len(response.Items) == 0 || response.Items[0].Id == nil {
		return "", nil
	}
	videoID := response.Items[0].Id.VideoId
	if videoID == "" {
		return "", nil
	}
	return videoID, nil
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
