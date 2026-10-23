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
	spotifyapi "github.com/zmb3/spotify/v2"
	"google.golang.org/api/googleapi"

	"github.com/manlikeabro/spotube/internal/activitylogger"
	"github.com/manlikeabro/spotube/internal/auth"
	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/db/table"
	"github.com/manlikeabro/spotube/internal/matching"
	"github.com/manlikeabro/spotube/internal/platform"
	"github.com/manlikeabro/spotube/internal/quota"
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
	// cache holds destination playlists shared with analysis; nil uses a per-batch cache.
	cache *PlaylistCache
	// budget pre-checks multi-call YouTube work; nil skips the check (the transport still gates
	// each call).
	budget *quota.Budget
}

// NewExecutor creates an executor that uses live API clients from clientFactory.
func NewExecutor(deps JobDeps, clientFactory *auth.ClientFactory) *Executor {
	e := &Executor{
		db:             deps.DB,
		logger:         deps.Logger,
		activityLogger: deps.ActivityLogger,
		clientFactory:  clientFactory,
		cache:          deps.Cache,
		budget:         deps.Budget,
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

	// Nothing was wrong with the item: put it back without using up an attempt.
	if d := classifyDeferral(item.Service, execErr); d != nil {
		e.logger.Info().Str("sync_item_id", itemID).Str("reason", d.Reason).Msg("sync item deferred")
		if d.PauseMapping != "" {
			e.pauseMapping(mappingID, d.PauseMapping, d.Reason)
		}
		updated, err := e.deferItem(itemID, d.Reason)
		if err != nil {
			return model.SyncItems{}, err
		}
		return updated, d
	}
	if isPlaylistFull(execErr) {
		e.pauseMapping(mappingID, "playlist_full", "The YouTube playlist is full (5,000 videos); new tracks can't be added.")
		updated, skipErr := e.markSkipped(itemID, "YouTube playlist is full (5,000 videos)")
		if skipErr != nil {
			return model.SyncItems{}, skipErr
		}
		return updated, ErrPlaylistFull
	}
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
		if err := e.checkBudget(ctx, item.Service, youtubeInsertCost); err != nil {
			return fail(err)
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

	// Check the whole search-and-insert fits before spending the search.
	if err := e.checkBudget(ctx, item.Service, youtubeSearchCost+youtubeInsertCost); err != nil {
		return fail(err)
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

	// Remember the match before inserting: if the insert fails (quota, rate limit), the retry
	// costs 50 units instead of repeating the 101-unit search.
	remember(selected, bestScore(candidates))
	if playlistContainsID(destTracks, selected.ID) {
		run.AlreadyInPlaylist = true
		run.Outcome = "skipped_duplicate"
		return run, ErrAlreadyInDestination
	}
	if err := dest.add(ctx, dest.playlistID, selected.ID); err != nil {
		return fail(err)
	}
	run.Outcome = "added"
	return run, nil
}

// YouTube costs of the calls an add makes.
const (
	youtubeSearchCost = 101 // search.list + videos.list for durations
	youtubeInsertCost = 50
)

// checkBudget refuses YouTube work that won't fit in ctx's quota class.
func (e *Executor) checkBudget(ctx context.Context, service string, units int) error {
	if service != "youtube" || e.budget == nil {
		return nil
	}
	return e.budget.Allows(quota.ClassOf(ctx), units)
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
		// YouTube also answers 409 when inserts arrive too fast ("ABORTED"), so only a 409 that
		// says the video is already there counts as success.
		return gerr.Code == 409 && strings.Contains(strings.ToLower(gerr.Error()), "already")
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

// maxAutoAttempts stops auto-apply from retrying an item that keeps failing; manual Execute still works.
const maxAutoAttempts = 3

// retryBackoffSQL is the wait after the nth failed attempt before auto-apply retries: 15
// minutes after the first, an hour after the second (the third is the last, maxAutoAttempts).
const retryBackoffSQL = `(900 * (1 << (2 * (attempt_count - 1))))`

// applicableItemsSQL selects items auto-apply may run: pending, or failed under the attempt
// cap once its backoff has passed, for mappings that aren't paused.
const applicableItemsSQL = `
	SELECT si.id, si.mapping_id, si.service, si.operation, si.created,
	       EXISTS (SELECT 1 FROM track_links tl WHERE
	         (si.service = 'youtube' AND tl.spotify_id = si.track_id AND tl.youtube_id IS NOT NULL) OR
	         (si.service = 'spotify' AND tl.youtube_id = si.track_id AND tl.spotify_id IS NOT NULL)) AS linked
	FROM sync_items si JOIN mappings m ON m.id = si.mapping_id
	WHERE (si.status = 'pending' OR (si.status = 'error' AND si.attempt_count < ? AND
	       COALESCE(si.last_attempt_at, 0) <= ? - ` + retryBackoffSQL + `))
	  AND COALESCE(m.last_sync_status, '') != 'playlist_missing'
	  AND NOT (COALESCE(m.last_sync_status, '') = 'playlist_full' AND si.service = 'youtube' AND si.operation = 'add')
	  AND ((si.operation = 'add' AND m.sync_tracks != 0) OR (si.operation = 'rename' AND m.sync_name != 0))`

// queuedItem is one applicable item, for ordering.
type queuedItem struct {
	id, mappingID, service, operation string
	created                           int64
	linked                            bool
}

func (e *Executor) applicableItems(ctx context.Context, mappingID string) ([]queuedItem, error) {
	query, args := applicableItemsSQL, []any{maxAutoAttempts, time.Now().Unix()}
	if mappingID != "" {
		query += ` AND si.mapping_id = ?`
		args = append(args, mappingID)
	}
	rows, err := e.db.QueryContext(ctx, query+` ORDER BY si.created, si.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []queuedItem
	for rows.Next() {
		var q queuedItem
		if err := rows.Scan(&q.id, &q.mappingID, &q.service, &q.operation, &q.created, &q.linked); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// orderForDrain puts renames first, then items whose song is already linked (50 units instead
// of 151), then the rest, taking turns across mappings so one huge playlist can't starve the
// others.
func orderForDrain(items []queuedItem) []queuedItem {
	tier := func(q queuedItem) int {
		switch {
		case q.operation == "rename":
			return 0
		case q.linked:
			return 1
		}
		return 2
	}
	var out []queuedItem
	for t := 0; t <= 2; t++ {
		var order []string
		byMapping := map[string][]queuedItem{}
		for _, q := range items {
			if tier(q) != t {
				continue
			}
			if _, seen := byMapping[q.mappingID]; !seen {
				order = append(order, q.mappingID)
			}
			byMapping[q.mappingID] = append(byMapping[q.mappingID], q)
		}
		for added := true; added; {
			added = false
			for _, m := range order {
				if len(byMapping[m]) > 0 {
					out = append(out, byMapping[m][0])
					byMapping[m] = byMapping[m][1:]
					added = true
				}
			}
		}
	}
	return out
}

// BatchResult counts outcomes of ExecutePending and Drain.
type BatchResult struct {
	Attempted int
	Done      int
	Skipped   int
	Review    int
	Failed    int
	// Deferred items went back to the queue untouched (quota, rate limit, paused playlist).
	Deferred int
	// PerMapping breaks the counts down for activity logs.
	PerMapping map[string]*BatchResult `json:"-"`
}

// Summary is a one-line activity log message for the batch.
func (b BatchResult) Summary() string {
	msg := fmt.Sprintf("Applied sync queue: %d added, %d skipped, %d need review, %d failed (of %d)",
		b.Done, b.Skipped, b.Review, b.Failed, b.Attempted)
	if b.Deferred > 0 {
		msg += fmt.Sprintf("; %d waiting for quota", b.Deferred)
	}
	return msg
}

func (b *BatchResult) count(mappingID string, f func(*BatchResult)) {
	f(b)
	if b.PerMapping == nil {
		b.PerMapping = map[string]*BatchResult{}
	}
	m := b.PerMapping[mappingID]
	if m == nil {
		m = &BatchResult{}
		b.PerMapping[mappingID] = m
	}
	f(m)
}

// LockFunc takes a mapping for the duration of one item; it returns false when the mapping
// is busy (being analysed), and the item is left for the next pass.
type LockFunc func(mappingID string) (unlock func(), ok bool)

// ExecutePending applies up to limit applicable items of one mapping. YouTube work stops when
// quota runs out (the items wait, unchanged); Spotify items carry on.
func (e *Executor) ExecutePending(ctx context.Context, mappingID string, limit int) (BatchResult, error) {
	items, err := e.applicableItems(ctx, mappingID)
	if err != nil {
		return BatchResult{}, err
	}
	return e.apply(ctx, orderForDrain(items), limit, nil)
}

// Drain applies up to limit applicable items across every mapping, in drain order.
func (e *Executor) Drain(ctx context.Context, limit int, lock LockFunc) (BatchResult, error) {
	items, err := e.applicableItems(ctx, "")
	if err != nil {
		return BatchResult{}, err
	}
	return e.apply(ctx, orderForDrain(items), limit, lock)
}

func (e *Executor) apply(ctx context.Context, items []queuedItem, limit int, lock LockFunc) (BatchResult, error) {
	var result BatchResult
	batch := *e
	cache := e.cache
	if cache == nil {
		cache = NewPlaylistCache(nil)
	}
	batch.spotify = &cachedSpotify{spotifyPlatform: e.spotify, cache: cache}
	batch.youtube = &cachedYouTube{youtubePlatform: e.youtube, cache: cache, db: e.db}
	defer cache.Flush(context.WithoutCancel(ctx))

	// A service is blocked for the rest of the pass once it runs out of quota or needs
	// reconnecting: every other item for it would fail the same way.
	blocked := map[string]error{}
	if e.budget != nil {
		if err := e.budget.Allows(quota.ClassOf(ctx), youtubeInsertCost); err != nil {
			blocked["youtube"] = err
		}
	}
	// paused holds mappings (playlist missing) and mapping|service pairs (playlist full) to
	// leave alone for the rest of the pass.
	paused := map[string]bool{}
	var stopErr error
	for _, q := range items {
		if result.Attempted >= limit {
			break
		}
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		if paused[q.mappingID] || paused[q.mappingID+"|"+q.service] {
			continue
		}
		if err, ok := blocked[q.service]; ok {
			stopErr = err
			continue
		}
		unlock := func() {}
		if lock != nil {
			var ok bool
			if unlock, ok = lock(q.mappingID); !ok {
				continue
			}
		}
		item, execErr := batch.ExecuteOne(ctx, q.id)
		unlock()
		var deferred *DeferredError
		switch {
		case errors.As(execErr, &deferred):
			result.count(q.mappingID, func(b *BatchResult) { b.Deferred++ })
			if deferred.PauseMapping != "" {
				paused[q.mappingID] = true
			}
			if deferred.BlocksService {
				blocked[q.service] = execErr
				stopErr = execErr
			}
			continue
		case errors.Is(execErr, ErrSyncItemNotExecutable):
			continue // claimed elsewhere (Sync now, a manual apply)
		case errors.Is(execErr, ErrPlaylistFull):
			paused[q.mappingID+"|"+q.service] = true
		}
		result.count(q.mappingID, func(b *BatchResult) { b.Attempted++ })
		switch {
		case execErr == nil:
			result.count(q.mappingID, func(b *BatchResult) { b.Done++ })
		case item.Status == "skipped":
			result.count(q.mappingID, func(b *BatchResult) { b.Skipped++ })
		case item.Status == "review":
			result.count(q.mappingID, func(b *BatchResult) { b.Review++ })
		default:
			result.count(q.mappingID, func(b *BatchResult) { b.Failed++ })
			if errors.Is(execErr, auth.ErrReconnectRequired) || errors.Is(execErr, auth.ErrNoTokenFound) {
				blocked[q.service] = execErr
				stopErr = execErr
			}
		}
	}
	return result, stopErr
}

// DeferredError means an item went back to the queue without using an attempt.
type DeferredError struct {
	Reason string
	Cause  error
	// BlocksService: every other item for the service would hit the same wall (quota).
	BlocksService bool
	// PauseMapping is the mapping status to set ("playlist_missing"), if any.
	PauseMapping string
}

func (d *DeferredError) Error() string { return d.Reason }
func (d *DeferredError) Unwrap() error { return d.Cause }

// ErrPlaylistFull means the YouTube playlist reached its 5,000-video limit.
var ErrPlaylistFull = errors.New("youtube playlist is full")

// classifyDeferral decides whether a failure is the item's fault or the environment's.
func classifyDeferral(service string, err error) *DeferredError {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, quota.ErrBudget):
		return &DeferredError{Reason: err.Error(), Cause: err, BlocksService: true}
	case isQuotaExceeded(err):
		return &DeferredError{Reason: "Waiting for YouTube quota (Google reported today's quota used up; it resets at midnight Pacific)", Cause: err, BlocksService: true}
	case isRateLimited(err):
		return &DeferredError{Reason: "Rate limited by " + serviceLabel(service) + "; will retry", Cause: err, BlocksService: true}
	case errors.Is(err, platform.ErrPlaylistNotFound) || isPlaylistNotFound(err):
		return &DeferredError{Reason: "The " + serviceLabel(service) + " playlist was not found (deleted?); sync for this mapping is paused", Cause: err, BlocksService: false, PauseMapping: "playlist_missing"}
	case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
		return &DeferredError{Reason: "Interrupted; will retry", Cause: err, BlocksService: true}
	}
	return nil
}

func (e *Executor) deferItem(itemID, reason string) (model.SyncItems, error) {
	now := time.Now().Unix()
	if _, err := e.db.Exec(
		`UPDATE sync_items SET status = 'pending', error_message = ?, updated = ? WHERE id = ?`,
		truncateExecutorError(reason), now, itemID,
	); err != nil {
		return model.SyncItems{}, err
	}
	return e.reloadItem(itemID)
}

// pauseMapping stops auto-apply for a mapping until its next successful analysis.
func (e *Executor) pauseMapping(mappingID, status, message string) {
	_, err := e.db.Exec(`UPDATE mappings SET last_sync_status = ?, last_sync_error = ?, updated = ? WHERE id = ?`,
		status, message, time.Now().Unix(), mappingID)
	if err != nil {
		e.logger.Error().Err(err).Str("mapping_id", mappingID).Msg("failed to pause mapping")
	}
}

// ResetInterrupted puts items left 'running' by a crash or restart back in the queue.
func (e *Executor) ResetInterrupted(ctx context.Context) (int64, error) {
	res, err := e.db.ExecContext(ctx,
		`UPDATE sync_items SET status = 'pending', error_message = 'Interrupted; will retry', updated = ? WHERE status = 'running'`,
		time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func serviceLabel(service string) string {
	if service == "youtube" {
		return "YouTube"
	}
	return "Spotify"
}

func googleReasons(err error) (int, []string) {
	var gerr *googleapi.Error
	if !errors.As(err, &gerr) {
		return 0, nil
	}
	reasons := make([]string, 0, len(gerr.Errors))
	for _, item := range gerr.Errors {
		reasons = append(reasons, item.Reason)
	}
	return gerr.Code, reasons
}

func hasReason(err error, code int, want ...string) bool {
	got, reasons := googleReasons(err)
	if got != code {
		return false
	}
	for _, r := range reasons {
		for _, w := range want {
			if r == w {
				return true
			}
		}
	}
	return false
}

// isQuotaExceeded: the daily quota is used up (not a per-minute limit).
func isQuotaExceeded(err error) bool {
	return hasReason(err, 403, "quotaExceeded", "dailyLimitExceeded")
}

// isRateLimited: a short-term limit; the call can be retried soon.
func isRateLimited(err error) bool {
	if hasReason(err, 403, "rateLimitExceeded", "userRateLimitExceeded") {
		return true
	}
	if code, _ := googleReasons(err); code == 429 {
		return true
	}
	var serr spotifyapi.Error
	return errors.As(err, &serr) && serr.Status == 429
}

func isPlaylistFull(err error) bool {
	return hasReason(err, 403, "playlistContainsMaximumNumberOfVideos")
}

func isPlaylistNotFound(err error) bool {
	return hasReason(err, 404, "playlistNotFound")
}

// cachedSpotify reads destination playlists through the shared cache and records adds.
type cachedSpotify struct {
	spotifyPlatform
	cache *PlaylistCache
}

func (c *cachedSpotify) ListPlaylistTracks(ctx context.Context, playlistID string) ([]matching.Track, error) {
	if tracks, _, ok := c.cache.Get(ctx, "spotify", playlistID); ok {
		return tracks, nil
	}
	tracks, err := c.spotifyPlatform.ListPlaylistTracks(ctx, playlistID)
	if err == nil {
		c.cache.Put(ctx, "spotify", playlistID, tracks)
	}
	return tracks, err
}

func (c *cachedSpotify) AddTrackToPlaylist(ctx context.Context, playlistID, trackID string) error {
	if err := c.spotifyPlatform.AddTrackToPlaylist(ctx, playlistID, trackID); err != nil {
		return err
	}
	c.cache.Append("spotify", playlistID, matching.Track{Platform: matching.PlatformSpotify, ID: trackID})
	return nil
}

// cachedYouTube reads destination playlists through the shared cache and records adds,
// including in the mapping's expected itemCount so the next probe doesn't mistake Spotube's
// own inserts for a change.
type cachedYouTube struct {
	youtubePlatform
	cache *PlaylistCache
	db    *sql.DB
}

func (c *cachedYouTube) ListPlaylistTracks(ctx context.Context, playlistID string) ([]matching.Track, error) {
	if tracks, _, ok := c.cache.Get(ctx, "youtube", playlistID); ok {
		return tracks, nil
	}
	tracks, err := c.youtubePlatform.ListPlaylistTracks(ctx, playlistID)
	if err == nil {
		c.cache.Put(ctx, "youtube", playlistID, tracks)
	}
	return tracks, err
}

func (c *cachedYouTube) AddVideoToPlaylist(ctx context.Context, playlistID, videoID string) error {
	if err := c.youtubePlatform.AddVideoToPlaylist(ctx, playlistID, videoID); err != nil {
		return err
	}
	c.cache.Append("youtube", playlistID, matching.Track{Platform: matching.PlatformYouTube, ID: videoID})
	if c.db != nil {
		_, _ = c.db.ExecContext(ctx,
			`UPDATE mappings SET youtube_item_count = youtube_item_count + 1 WHERE youtube_playlist_id = ? AND youtube_item_count IS NOT NULL`,
			playlistID)
	}
	return nil
}
