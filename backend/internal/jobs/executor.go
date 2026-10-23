package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-jet/jet/v2/sqlite"
	"github.com/rs/zerolog"
	"google.golang.org/api/googleapi"

	"github.com/manlikeabro/spotube/internal/activitylogger"
	"github.com/manlikeabro/spotube/internal/auth"
	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/db/table"
	"github.com/manlikeabro/spotube/internal/matching"
	"github.com/manlikeabro/spotube/internal/platform"
	"github.com/manlikeabro/spotube/internal/synccontext"
	"github.com/manlikeabro/spotube/internal/tracklinks"
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
	e.spotify = &platform.Spotify{Factory: clientFactory}
	e.youtube = &platform.YouTube{Factory: clientFactory}
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
	e.recordExecutorLog(mappingID, itemID, item.Service, run, execErr)

	if errors.Is(execErr, ErrNotAvailable) {
		updated, skipErr := e.markSkipped(itemID, "not on "+serviceLabel(item.Service))
		if skipErr != nil {
			return model.SyncItems{}, skipErr
		}
		return updated, ErrNotAvailable
	}
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

// ErrNoConfidentMatch means the search found nothing good enough; the item waits for review
// instead of being retried automatically.
var ErrNoConfidentMatch = errors.New("no confident match")

// ErrNotAvailable means the user marked the song as not available on the destination service.
var ErrNotAvailable = errors.New("not available on destination service")

// destination bundles the playlist operations for the service an item is added to.
type destination struct {
	platform   matching.Platform
	playlistID string
	list       func(context.Context, string) ([]matching.Track, error)
	search     func(context.Context, string, string, int) ([]platformSearchHit, error)
	add        func(context.Context, string, string) error
}

func (e *Executor) destinationFor(service string, mapping model.Mappings) (destination, bool) {
	switch service {
	case "youtube":
		return destination{matching.PlatformYouTube, mapping.YoutubePlaylistID,
			e.youtube.ListPlaylistTracks, e.youtube.SearchVideos, e.youtube.AddVideoToPlaylist}, true
	case "spotify":
		return destination{matching.PlatformSpotify, mapping.SpotifyPlaylistID,
			e.spotify.ListPlaylistTracks, e.spotify.SearchTracks, e.spotify.AddTrackToPlaylist}, true
	}
	return destination{}, false
}

// SourceTrack rebuilds the source track for an add item, preferring the full track (with
// duration and channel) recorded by analysis.
func SourceTrack(item model.SyncItems) matching.Track {
	if raw := stringFromPtr(item.AnalysisContextJSON); raw != "" {
		var ctx synccontext.AnalysisItemContext
		if json.Unmarshal([]byte(raw), &ctx) == nil && ctx.Decision.Source.ID == stringFromPtr(item.TrackID) {
			return ctx.Decision.Source
		}
	}
	id := stringFromPtr(item.TrackID)
	title := strings.TrimSpace(stringFromPtr(item.TrackTitle))
	artist := strings.TrimSpace(stringFromPtr(item.TrackArtist))
	if item.Service == "youtube" {
		return matching.SpotifyTrack(id, title, artist)
	}
	if artist == "" {
		// Items queued before titles were parsed at analysis time hold the raw video title.
		return matching.YouTubeFromRaw(id, title)
	}
	return matching.Track{Platform: matching.PlatformYouTube, ID: id, Title: title, Artist: artist}
}

// executeAdd copies one track to the destination playlist. A remembered link is used
// directly (no search); otherwise the destination is searched and a confident match is
// linked for next time.
func (e *Executor) executeAdd(ctx context.Context, item model.SyncItems, mapping model.Mappings) (synccontext.ExecutorRun, error) {
	source := SourceTrack(item)
	run := synccontext.ExecutorRun{Operation: "add", Source: source}

	dest, ok := e.destinationFor(item.Service, mapping)
	if !ok {
		return run, fmt.Errorf("unsupported service: %s", item.Service)
	}
	run.DestPlaylistID = dest.playlistID
	fail := func(err error) (synccontext.ExecutorRun, error) {
		run.Outcome = "error"
		run.Error = err.Error()
		return run, err
	}

	destTracks, err := dest.list(ctx, dest.playlistID)
	if err != nil {
		return fail(err)
	}

	links := e.linkStore()
	link, err := links.Find(ctx, string(source.Platform), source.ID)
	if err != nil {
		return fail(err)
	}
	if link != nil {
		target, missing := link.Counterpart(item.Service)
		if missing {
			run.Outcome = "not_available"
			return run, ErrNotAvailable
		}
		run.Linked = true
		run.Selected = &matching.Track{Platform: dest.platform, ID: target, Title: link.Title, Artist: link.Artist}
		if playlistContainsID(destTracks, target) {
			run.AlreadyInPlaylist = true
			run.Outcome = "skipped_duplicate"
			return run, ErrAlreadyInDestination
		}
		if err := dest.add(ctx, dest.playlistID, target); err != nil {
			return fail(err)
		}
		run.Outcome = "added"
		return run, nil
	}

	remember := func(target matching.Track, score float64) {
		spotifyID, youtubeID := source.ID, target.ID
		if source.Platform == matching.PlatformYouTube {
			spotifyID, youtubeID = target.ID, source.ID
		}
		if err := links.LinkAuto(ctx, spotifyID, youtubeID, source.Title, source.Artist, score); err != nil {
			e.logger.Warn().Err(err).Str("sync_item_id", stringFromPtr(item.ID)).Msg("failed to record track link")
		}
	}

	playlistMatch := matching.FindMatch(source, destTracks)
	run.PlaylistMatch = &playlistMatch
	if playlistMatch.Matched {
		remember(*playlistMatch.BestMatch, playlistMatch.Score)
		run.AlreadyInPlaylist = true
		run.Outcome = "skipped_duplicate"
		return run, ErrAlreadyInDestination
	}

	run.SearchQuery = strings.TrimSpace(source.Title + " " + source.Artist)
	hits, err := dest.search(ctx, source.Title, source.Artist, executorSearchLimit)
	if err != nil {
		return fail(err)
	}
	selected, candidates, pickErr := pickSearchCandidate(source, hits, dest.platform)
	run.Candidates = candidates
	if pickErr != nil {
		run.Outcome = "no_match"
		run.Error = pickErr.Error()
		return run, fmt.Errorf("%w: %s", ErrNoConfidentMatch, pickErr.Error())
	}
	run.Selected = &selected

	if playlistContainsID(destTracks, selected.ID) {
		remember(selected, bestScore(candidates))
		run.AlreadyInPlaylist = true
		run.Outcome = "skipped_duplicate"
		return run, ErrAlreadyInDestination
	}
	if err := dest.add(ctx, dest.playlistID, selected.ID); err != nil {
		return fail(err)
	}
	remember(selected, bestScore(candidates))
	run.Outcome = "added"
	return run, nil
}

func (e *Executor) linkStore() *tracklinks.Store {
	return &tracklinks.Store{DB: e.db}
}

func bestScore(candidates []synccontext.SearchCandidate) float64 {
	best := 0.0
	for _, c := range candidates {
		best = max(best, c.Score)
	}
	return best
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
		return matching.Track{}, nil, fmt.Errorf("no results on %s", platform)
	}

	candidates := make([]synccontext.SearchCandidate, 0, len(hits))
	var best matching.Track
	bestTotal := 0.0
	for i, hit := range hits {
		candidate := hit.Track(platform)
		score := matching.ScoreCandidate(source, candidate)
		candidates = append(candidates, synccontext.SearchCandidate{
			Rank:       i + 1,
			ID:         hit.ID,
			Title:      hit.Title,
			Artist:     hit.Artist,
			Channel:    firstNonEmpty(hit.Channel, hit.Album),
			RawTitle:   hit.RawTitle,
			DurationMs: hit.DurationMs,
			Image:      hit.Image,
			Score:      score.Total,
			Breakdown:  &score,
		})
		if score.Total > bestTotal {
			bestTotal = score.Total
			best = candidate
		}
	}
	if bestTotal < matching.MatchThreshold {
		return matching.Track{}, candidates, fmt.Errorf("no confident match on %s (best score %.2f)", platform, bestTotal)
	}
	return best, candidates, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func playlistContainsID(tracks []matching.Track, id string) bool {
	for _, track := range tracks {
		if track.ID == id {
			return true
		}
	}
	return false
}

func (e *Executor) recordExecutorLog(mappingID, itemID, dest string, run synccontext.ExecutorRun, execErr error) {
	level := "info"
	switch {
	case execErr == nil, errors.Is(execErr, ErrAlreadyInDestination), errors.Is(execErr, ErrNotAvailable):
	case errors.Is(execErr, ErrNoConfidentMatch):
		level = "warn"
	default:
		level = "error"
		if run.Error == "" {
			run.Error = execErr.Error()
		}
	}
	message := synccontext.BriefExecutorLine(run, dest)
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
		if errors.Is(execErr, ErrNoConfidentMatch) {
			status = "review"
		}
		errMsg = truncateExecutorError(strings.TrimPrefix(execErr.Error(), ErrNoConfidentMatch.Error()+": "))
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
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		// 409 is YouTube's "already in playlist"; other 4xx codes are real failures.
		return gerr.Code == 409
	}
	return strings.Contains(strings.ToLower(err.Error()), "already")
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

// platformSearchHit is a search result from either service.
type platformSearchHit = platform.SearchHit

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

// PlaylistSnapshot holds destination playlist contents already fetched during analysis,
// keyed by playlist ID, so applying a batch does not re-list playlists per item.
type PlaylistSnapshot struct {
	Spotify map[string][]matching.Track
	YouTube map[string][]matching.Track
}

// BatchResult counts outcomes of ExecutePending.
type BatchResult struct {
	Attempted int
	Done      int
	Skipped   int
	Review    int
	Failed    int
}

// Summary is a one-line activity log message for the batch.
func (b BatchResult) Summary() string {
	return fmt.Sprintf("Applied sync queue: %d added, %d skipped, %d need review, %d failed (of %d)",
		b.Done, b.Skipped, b.Review, b.Failed, b.Attempted)
}

// maxAutoAttempts stops auto-apply from retrying an item that keeps failing; manual Execute still works.
const maxAutoAttempts = 3

// ExecutePending applies up to limit pending (or retryable error) items for one mapping.
// It stops early when a provider needs reconnecting or YouTube quota is exhausted, since
// every remaining item would fail the same way.
func (e *Executor) ExecutePending(ctx context.Context, mappingID string, limit int, seed *PlaylistSnapshot) (BatchResult, error) {
	var result BatchResult
	rows, err := e.db.QueryContext(ctx,
		`SELECT id FROM sync_items
		 WHERE mapping_id = ? AND (status = 'pending' OR (status = 'error' AND attempt_count < ?))
		 ORDER BY created, id LIMIT ?`,
		mappingID, maxAutoAttempts, limit,
	)
	if err != nil {
		return result, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return result, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return result, err
	}

	batch := *e
	batch.spotify = newCachedSpotify(e.spotify, seed)
	batch.youtube = newCachedYouTube(e.youtube, seed)

	for _, id := range ids {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		result.Attempted++
		item, execErr := batch.ExecuteOne(ctx, id)
		switch {
		case execErr == nil:
			result.Done++
		case item.Status == "skipped":
			result.Skipped++
		case item.Status == "review":
			result.Review++
		default:
			result.Failed++
			if errors.Is(execErr, auth.ErrReconnectRequired) || isQuotaExceeded(execErr) {
				return result, execErr
			}
		}
	}
	return result, nil
}

func serviceLabel(service string) string {
	if service == "youtube" {
		return "YouTube"
	}
	return "Spotify"
}

func isQuotaExceeded(err error) bool {
	var gerr *googleapi.Error
	if !errors.As(err, &gerr) || gerr.Code != 403 {
		return false
	}
	for _, item := range gerr.Errors {
		if item.Reason == "quotaExceeded" || item.Reason == "dailyLimitExceeded" {
			return true
		}
	}
	return false
}

// cachedSpotify memoizes playlist listings for one batch and records tracks it adds.
type cachedSpotify struct {
	spotifyPlatform
	lists map[string][]matching.Track
}

func newCachedSpotify(base spotifyPlatform, seed *PlaylistSnapshot) *cachedSpotify {
	c := &cachedSpotify{spotifyPlatform: base, lists: map[string][]matching.Track{}}
	if seed != nil {
		for id, tracks := range seed.Spotify {
			c.lists[id] = append([]matching.Track(nil), tracks...)
		}
	}
	return c
}

func (c *cachedSpotify) ListPlaylistTracks(ctx context.Context, playlistID string) ([]matching.Track, error) {
	if tracks, ok := c.lists[playlistID]; ok {
		return tracks, nil
	}
	tracks, err := c.spotifyPlatform.ListPlaylistTracks(ctx, playlistID)
	if err == nil {
		c.lists[playlistID] = tracks
	}
	return tracks, err
}

func (c *cachedSpotify) AddTrackToPlaylist(ctx context.Context, playlistID, trackID string) error {
	if err := c.spotifyPlatform.AddTrackToPlaylist(ctx, playlistID, trackID); err != nil {
		return err
	}
	if tracks, ok := c.lists[playlistID]; ok {
		c.lists[playlistID] = append(tracks, matching.Track{Platform: matching.PlatformSpotify, ID: trackID})
	}
	return nil
}

// cachedYouTube memoizes playlist listings for one batch and records videos it adds.
type cachedYouTube struct {
	youtubePlatform
	lists map[string][]matching.Track
}

func newCachedYouTube(base youtubePlatform, seed *PlaylistSnapshot) *cachedYouTube {
	c := &cachedYouTube{youtubePlatform: base, lists: map[string][]matching.Track{}}
	if seed != nil {
		for id, tracks := range seed.YouTube {
			c.lists[id] = append([]matching.Track(nil), tracks...)
		}
	}
	return c
}

func (c *cachedYouTube) ListPlaylistTracks(ctx context.Context, playlistID string) ([]matching.Track, error) {
	if tracks, ok := c.lists[playlistID]; ok {
		return tracks, nil
	}
	tracks, err := c.youtubePlatform.ListPlaylistTracks(ctx, playlistID)
	if err == nil {
		c.lists[playlistID] = tracks
	}
	return tracks, err
}

func (c *cachedYouTube) AddVideoToPlaylist(ctx context.Context, playlistID, videoID string) error {
	if err := c.youtubePlatform.AddVideoToPlaylist(ctx, playlistID, videoID); err != nil {
		return err
	}
	if tracks, ok := c.lists[playlistID]; ok {
		c.lists[playlistID] = append(tracks, matching.Track{Platform: matching.PlatformYouTube, ID: videoID})
	}
	return nil
}
