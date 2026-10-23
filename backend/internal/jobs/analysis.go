package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/samber/lo"
	spotify "github.com/zmb3/spotify/v2"

	"github.com/manlikeabro/spotube/internal/activitylogger"
	"github.com/manlikeabro/spotube/internal/auth"
	"github.com/manlikeabro/spotube/internal/backups"
	"github.com/manlikeabro/spotube/internal/db/model"
	"github.com/manlikeabro/spotube/internal/db/table"
	"github.com/manlikeabro/spotube/internal/matching"
	"github.com/manlikeabro/spotube/internal/platform"
	"github.com/manlikeabro/spotube/internal/quota"
	"github.com/manlikeabro/spotube/internal/schedule"
	"github.com/manlikeabro/spotube/internal/settings"
	"github.com/manlikeabro/spotube/internal/synccontext"
	"github.com/manlikeabro/spotube/internal/tracklinks"
)

// AnalysisJob is the sync runner shared by the scheduler, Sync now and new mappings: it
// analyses mapped playlists (queueing what is missing on each side) and applies the queue.
// One instance owns the per-mapping locks, so analysis and applying never overlap for a
// mapping however they were started.
type AnalysisJob struct {
	db             *sql.DB
	logger         zerolog.Logger
	activityLogger *activitylogger.Logger
	clientFactory  *auth.ClientFactory
	// backups records a free snapshot of backed-up playlists that analysis already fetched.
	backups *backups.Store
	reader  playlistReader
	cache   *PlaylistCache
	budget  *quota.Budget
	// settings switches background analysis and auto-apply at runtime; nil means both on.
	settings *settings.Store

	// applier applies queued items; nil disables applying.
	applier    *Executor
	applyLimit int

	mu        sync.Mutex
	busy      map[string]bool
	lastPrune time.Time
	// queued holds first syncs waiting for the background worker, which runs them one at a
	// time (mirroring 50 playlists must not start 50 syncs at once).
	queued        []model.Mappings
	queuedIDs     map[string]bool
	workerRunning bool
}

func NewAnalysisJob(deps JobDeps, clientFactory *auth.ClientFactory) *AnalysisJob {
	cache := deps.Cache
	if cache == nil {
		cache = NewPlaylistCache(deps.DB)
	}
	return &AnalysisJob{
		db:             deps.DB,
		logger:         deps.Logger,
		activityLogger: deps.ActivityLogger,
		clientFactory:  clientFactory,
		backups:        &backups.Store{DB: deps.DB},
		reader:         &liveReader{factory: clientFactory},
		cache:          cache,
		budget:         deps.Budget,
		settings:       deps.Settings,
		busy:           map[string]bool{},
		queuedIDs:      map[string]bool{},
	}
}

// EnableAutoApply sets the executor that applies queued items; limit caps items per Sync now.
func (j *AnalysisJob) EnableAutoApply(executor *Executor, limit int) {
	if limit <= 0 {
		limit = defaultApplyLimit
	}
	j.applier = executor
	j.applyLimit = limit
}

// defaultApplyLimit caps items Sync now applies for one mapping; the background drain takes
// the rest within the quota.
const defaultApplyLimit = 25

// defaultDrainLimit is the per-tick cap when no settings are configured.
const defaultDrainLimit = 100

// syncNowFreshness: Sync now re-reads a YouTube playlist whose last full read is older than
// this, even when the probe shows no change.
const syncNowFreshness = 10 * time.Minute

func (j *AnalysisJob) workersEnabled() bool {
	return j.settings == nil || j.settings.Bool(settings.SyncWorkersEnabled)
}

func (j *AnalysisJob) autoApplyEnabled() bool {
	return j.applier != nil && (j.settings == nil || j.settings.Bool(settings.AutoApplyEnabled))
}

// ErrSyncBusy means the mapping is already being synced.
var ErrSyncBusy = errors.New("this mapping is already syncing")

// lock takes a mapping; ok is false when it is busy.
func (j *AnalysisJob) lock(mappingID string) (func(), bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.busy[mappingID] {
		return nil, false
	}
	j.busy[mappingID] = true
	return func() {
		j.mu.Lock()
		delete(j.busy, mappingID)
		j.mu.Unlock()
	}, true
}

// IsSyncing reports whether a mapping is being analysed or applied, or waiting to be.
func (j *AnalysisJob) IsSyncing(mappingID string) bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.busy[mappingID] || j.queuedIDs[mappingID]
}

// Run is one scheduler tick: analyse mappings that are due (when background sync is on), then
// apply queued items across all mappings while the quota allows (when auto-apply is on).
func (j *AnalysisJob) Run(ctx context.Context) error {
	if time.Since(j.lastPrune) > time.Hour {
		j.pruneActivityLogs()
		j.lastPrune = time.Now()
	}

	if j.workersEnabled() {
		mappings, err := j.dueMappings(time.Now())
		if err != nil {
			j.logger.Error().Err(err).Msg("failed to get mappings for analysis")
			return err
		}
		if len(mappings) > 0 {
			j.logger.Info().Int("count", len(mappings)).Msg("scheduled sync starting")
		}
		readCtx := quota.WithClass(ctx, quota.Read)
		for _, mapping := range mappings {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			unlock, ok := j.lock(lo.FromPtr(mapping.ID))
			if !ok {
				continue // Sync now has it; the next tick picks it up
			}
			j.syncOne(readCtx, mapping, true, 0)
			unlock()
		}
	}

	if j.autoApplyEnabled() {
		j.drain(ctx)
	}
	return nil
}

// drain applies queued items across mappings, renames and linked songs first, taking turns.
func (j *AnalysisJob) drain(ctx context.Context) {
	limit := defaultDrainLimit
	if j.settings != nil {
		limit = j.settings.Int(settings.DrainItemsPerTick)
	}
	result, err := j.applier.Drain(quota.WithClass(ctx, quota.Drain), limit, j.lock)
	for mappingID, r := range result.PerMapping {
		if r.Attempted > 0 {
			j.activityLogger.RecordInfo(r.Summary(), mappingID, "executor")
		}
	}
	if err != nil && !errors.Is(err, quota.ErrBudget) {
		j.logger.Warn().Err(err).Msg("apply pass stopped early")
	}
	if result.Attempted > 0 || result.Deferred > 0 {
		j.logger.Info().Int("attempted", result.Attempted).Int("done", result.Done).
			Int("deferred", result.Deferred).Msg("apply pass finished")
	}
}

// SyncMapping is Sync now: analyse the mapping (re-reading YouTube unless it was read in the
// last few minutes) and apply part of its queue. It does not move the scheduled next run.
func (j *AnalysisJob) SyncMapping(ctx context.Context, mapping model.Mappings) error {
	unlock, ok := j.lock(lo.FromPtr(mapping.ID))
	if !ok {
		return ErrSyncBusy
	}
	defer unlock()
	j.syncOne(quota.WithClass(ctx, quota.Interactive), mapping, false, syncNowFreshness)
	return nil
}

// StartSync queues a Sync now in the background (the first sync of a new mapping). Queued
// syncs run one at a time. It returns false when the mapping is already queued or syncing.
func (j *AnalysisJob) StartSync(mapping model.Mappings) bool {
	id := lo.FromPtr(mapping.ID)
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.busy[id] || j.queuedIDs[id] {
		return false
	}
	j.queued = append(j.queued, mapping)
	j.queuedIDs[id] = true
	if !j.workerRunning {
		j.workerRunning = true
		go j.syncWorker()
	}
	return true
}

func (j *AnalysisJob) syncWorker() {
	for {
		j.mu.Lock()
		if len(j.queued) == 0 {
			j.workerRunning = false
			j.mu.Unlock()
			return
		}
		mapping := j.queued[0]
		j.queued = j.queued[1:]
		id := lo.FromPtr(mapping.ID)
		j.mu.Unlock()

		unlock, ok := j.lock(id)
		j.mu.Lock()
		delete(j.queuedIDs, id)
		j.mu.Unlock()
		if !ok {
			continue // already syncing
		}
		j.syncOne(quota.WithClass(context.Background(), quota.Interactive), mapping, false, syncNowFreshness)
		unlock()
	}
}

// syncOne analyses a mapping and, for Sync now, applies its queue; the caller holds the lock.
// maxAge forces a fresh YouTube read when the cached one is older (0: the size-based default).
func (j *AnalysisJob) syncOne(ctx context.Context, mapping model.Mappings, scheduled bool, maxAge time.Duration) {
	mappingID := lo.FromPtr(mapping.ID)

	outcome, err := j.analyzeMapping(ctx, mapping, maxAge)
	if err == nil && !scheduled && j.autoApplyEnabled() && mapping.SyncTracks != 0 {
		// Applying counts as background work even when the user asked: a big first copy must
		// not eat the quota kept for searches.
		batch, applyErr := j.applier.ExecutePending(quota.WithClass(ctx, quota.Drain), mappingID, j.applyLimit)
		if batch.Attempted > 0 {
			j.activityLogger.RecordInfo(batch.Summary(), mappingID, "executor")
		}
		var deferred *DeferredError
		if applyErr != nil && !errors.As(applyErr, &deferred) && !errors.Is(applyErr, quota.ErrBudget) {
			err = applyErr
		}
	}

	if err != nil && ctx.Err() != nil {
		// Interrupted (e.g. the server is stopping): not the mapping's fault. Leave its status
		// and next run as they were so it runs again after the restart.
		j.logger.Info().Str("mapping_id", mappingID).Msg("mapping sync interrupted")
		return
	}
	status, errMsg := outcome.status, outcome.message
	if err != nil {
		status, errMsg = "error", truncateExecutorError(err.Error())
		switch {
		case errors.Is(err, auth.ErrReconnectRequired) || errors.Is(err, auth.ErrNoTokenFound):
			status = "reconnect_required"
		case errors.Is(err, platform.ErrPlaylistNotFound):
			status = "playlist_missing"
		case errors.Is(err, quota.ErrBudget):
			status = "waiting_quota"
		}
		if status != "waiting_quota" {
			j.logger.Error().Err(err).Str("mapping_id", mappingID).Msg("mapping sync failed")
		}
		// A failure that repeats every run (e.g. a revoked token) is already shown on the
		// mapping; only log it to the activity feed when it first appears or changes.
		if errMsg != lo.FromPtr(mapping.LastSyncError) && status != "waiting_quota" {
			j.activityLogger.RecordError("Sync failed: "+err.Error(), mappingID, "analysis")
		}
	}
	var next *time.Time
	if scheduled {
		n := ScheduleOf(mapping).Next(time.Now())
		// A run refused for quota tries again when the quota allows, if that is sooner than
		// the next slot (a daily mapping shouldn't lose a day).
		var qe *quota.Error
		if errors.As(err, &qe) && !qe.RetryAt.IsZero() && qe.RetryAt.Before(n) {
			n = qe.RetryAt
		}
		next = &n
	}
	// Stamp the run even on failure so a broken mapping waits for its next run instead of
	// retrying (and logging an error) on every cron tick.
	if stampErr := j.recordSyncResult(mappingID, status, errMsg, next); stampErr != nil {
		j.logger.Error().Err(stampErr).Str("mapping_id", mappingID).Msg("failed to record sync result")
	}
}

// ScheduleOf returns a mapping's schedule.
func ScheduleOf(m model.Mappings) schedule.Schedule {
	return schedule.Schedule{
		IntervalMinutes: int(m.IntervalMinutes),
		Start:           m.ScheduleStart,
		Days:            int(m.ScheduleDays),
		TZ:              lo.FromPtr(m.ScheduleTz),
	}
}

// dueMappings returns mappings whose next run has come. A mapping without one (created before
// schedules, or never run) is due when its schedule has a run since its last analysis.
func (j *AnalysisJob) dueMappings(now time.Time) ([]model.Mappings, error) {
	var mappings []model.Mappings
	if err := table.Mappings.SELECT(table.Mappings.AllColumns).Query(j.db, &mappings); err != nil {
		return nil, err
	}
	var due []model.Mappings
	for _, m := range mappings {
		if m.SyncTracks == 0 && m.SyncName == 0 {
			continue
		}
		switch {
		case m.NextRunAt != nil:
			if int64(*m.NextRunAt) <= now.Unix() {
				due = append(due, m)
			}
		case m.LastAnalysisAt == nil:
			due = append(due, m)
		default:
			if !ScheduleOf(m).Next(time.Unix(int64(*m.LastAnalysisAt), 0)).After(now) {
				due = append(due, m)
			}
		}
	}
	return due, nil
}

// DailyReadUnits estimates the YouTube units scheduled syncs use per day (0 while background
// sync is off), for the quota budget and schedule advice.
func (j *AnalysisJob) DailyReadUnits() int {
	if !j.workersEnabled() {
		return 0
	}
	var mappings []model.Mappings
	if err := table.Mappings.SELECT(table.Mappings.AllColumns).Query(j.db, &mappings); err != nil {
		return 0
	}
	total := 0
	for _, m := range mappings {
		if m.SyncTracks == 0 && m.SyncName == 0 {
			continue
		}
		total += schedule.DailyReadUnits(ScheduleOf(m), int(max(m.YoutubeTracksCount, m.SpotifyTracksCount)))
	}
	return total
}

// analysisOutcome is the status a successful analysis leaves on the mapping.
type analysisOutcome struct {
	status, message string
}

// youtubePlaylistLimit is the most videos a YouTube playlist can hold.
const youtubePlaylistLimit = 5000

func (j *AnalysisJob) analyzeMapping(ctx context.Context, mapping model.Mappings, maxAge time.Duration) (analysisOutcome, error) {
	mappingID := lo.FromPtr(mapping.ID)
	outcome := analysisOutcome{status: "ok"}
	if mapping.SyncTracks == 0 && mapping.SyncName == 0 {
		return outcome, nil
	}
	j.logger.Debug().Str("mapping_id", mappingID).Msg("analyzing mapping")

	// Spotify reads are free: read the whole playlist every time.
	var sp []matching.Track
	var err error
	if mapping.SyncTracks != 0 {
		if sp, err = j.reader.SpotifyTracks(ctx, mapping.SpotifyPlaylistID); err != nil {
			return outcome, err
		}
		j.cache.Put(ctx, "spotify", mapping.SpotifyPlaylistID, sp)
	}
	if mapping.SyncName != 0 {
		name, err := j.reader.SpotifyName(ctx, mapping.SpotifyPlaylistID)
		if err != nil {
			return outcome, err
		}
		mapping.SpotifyPlaylistName = stringPtr(name)
	}

	// YouTube: a 1-unit probe, then a full read only if the playlist changed.
	title, itemCount, err := j.reader.YouTubeInfo(ctx, mapping.YoutubePlaylistID)
	if err != nil {
		return outcome, err
	}
	if mapping.SyncName != 0 && title != "" {
		mapping.YoutubePlaylistName = stringPtr(title)
	}

	var links *tracklinks.Index
	var yt []matching.Track
	freshYouTube := false
	if mapping.SyncTracks != 0 {
		if links, err = (&tracklinks.Store{DB: j.db}).LoadIndex(ctx); err != nil {
			return outcome, err
		}
		cached, readAt, ok := j.cache.Get(ctx, "youtube", mapping.YoutubePlaylistID)
		if maxAge <= 0 {
			maxAge = schedule.FullReadEvery(itemCount)
		}
		changed := mapping.YoutubeItemCount == nil || int(*mapping.YoutubeItemCount) != itemCount
		if !ok || changed || time.Since(readAt) > maxAge {
			if j.budget != nil {
				if err := j.budget.Allows(quota.ClassOf(ctx), schedule.FullReadUnits(itemCount)); err != nil {
					return outcome, err
				}
			}
			needDuration := func(id string) bool { _, linked := links.For("youtube", id); return !linked }
			if yt, err = j.reader.YouTubeTracks(ctx, mapping.YoutubePlaylistID, needDuration); err != nil {
				return outcome, err
			}
			j.cache.Put(ctx, "youtube", mapping.YoutubePlaylistID, yt)
			freshYouTube = true
		} else {
			yt = cached
			j.logger.Debug().Str("mapping_id", mappingID).Int("items", itemCount).Msg("youtube playlist unchanged; using cached read")
		}
	}

	if mapping.SyncName != 0 {
		if _, err := j.db.Exec("UPDATE mappings SET spotify_playlist_name = ?, youtube_playlist_name = ? WHERE id = ?",
			mapping.SpotifyPlaylistName, mapping.YoutubePlaylistName, mappingID); err != nil {
			return outcome, err
		}
	}

	spotifyTracks, youtubeTracks := toTrackInfos(sp), toTrackInfos(yt)
	result := j.generateSyncItems(mapping, spotifyTracks, youtubeTracks, links)
	if err := j.rememberPairs(ctx, result.pairs); err != nil {
		return outcome, err
	}
	if mapping.SyncTracks != 0 {
		// Analysis runs before anything is applied, so this is the state before Spotube's changes.
		j.observeBackup(ctx, "spotify", mapping.SpotifyPlaylistID, lo.FromPtr(mapping.SpotifyPlaylistName), sp)
		if freshYouTube {
			j.observeBackup(ctx, "youtube", mapping.YoutubePlaylistID, lo.FromPtr(mapping.YoutubePlaylistName), yt)
		}
	}

	inserted := 0
	if len(result.items) > 0 {
		inserted, err = j.insertSyncItems(ctx, result.items)
		if err != nil {
			return outcome, err
		}
	}
	if mapping.SyncTracks != 0 {
		if err := j.reconcileQueuedAdds(ctx, mappingID, result.items, spotifyTracks, youtubeTracks, result.unavailable); err != nil {
			return outcome, err
		}
	}

	if err := j.updateMappingCounts(ctx, mappingID, len(sp), itemCount, freshYouTube); err != nil {
		return outcome, err
	}
	if itemCount >= youtubePlaylistLimit {
		outcome = analysisOutcome{status: "playlist_full", message: "The YouTube playlist is full (5,000 videos); new tracks can't be added."}
	}

	alreadyQueued := len(result.items) - inserted
	j.logger.Info().
		Str("mapping_id", mappingID).
		Bool("youtube_read", freshYouTube).
		Int("generated", len(result.items)).
		Int("inserted", inserted).
		Int("matched_skipped", result.matchedSkipped).
		Msg("mapping analysis completed")
	// Only runs that found something reach the activity feed.
	if inserted > 0 {
		msg := "Found " + strconv.Itoa(inserted) + " tracks to sync"
		if alreadyQueued > 0 {
			msg += " (" + strconv.Itoa(alreadyQueued) + " already queued)"
		}
		summary := synccontext.Envelope{
			Version: synccontext.Version,
			Kind:    "analysis_run",
			Data: synccontext.AnalysisRun{
				MappingID:      mappingID,
				SpotifyCount:   len(sp),
				YoutubeCount:   len(yt),
				QueuedAdds:     inserted,
				MatchedSkipped: result.matchedSkipped,
				AlreadyQueued:  alreadyQueued,
			},
		}
		_ = j.activityLogger.RecordWithDetails("info", msg, mappingID, "analysis", "", summary)
	}
	return outcome, nil
}

// playlistReader is what analysis reads from the services (faked in tests).
type playlistReader interface {
	SpotifyTracks(ctx context.Context, playlistID string) ([]matching.Track, error)
	SpotifyName(ctx context.Context, playlistID string) (string, error)
	// YouTubeInfo is the 1-unit probe: title and itemCount.
	YouTubeInfo(ctx context.Context, playlistID string) (string, int, error)
	YouTubeTracks(ctx context.Context, playlistID string, needDuration func(string) bool) ([]matching.Track, error)
}

type liveReader struct{ factory *auth.ClientFactory }

func (r *liveReader) SpotifyTracks(ctx context.Context, id string) ([]matching.Track, error) {
	client, err := r.factory.GetSpotifyClient(ctx)
	if err != nil {
		return nil, err
	}
	tracks, err := platform.SpotifyPlaylistTracks(ctx, client, id)
	var serr spotify.Error
	if errors.As(err, &serr) && serr.Status == 404 {
		return nil, platform.ErrPlaylistNotFound
	}
	return tracks, err
}

func (r *liveReader) SpotifyName(ctx context.Context, id string) (string, error) {
	client, err := r.factory.GetSpotifyClient(ctx)
	if err != nil {
		return "", err
	}
	return platform.SpotifyPlaylistName(ctx, client, id)
}

func (r *liveReader) YouTubeInfo(ctx context.Context, id string) (string, int, error) {
	service, err := r.factory.GetYouTubeService(ctx)
	if err != nil {
		return "", 0, err
	}
	return platform.YouTubePlaylistInfo(ctx, service, id)
}

func (r *liveReader) YouTubeTracks(ctx context.Context, id string, needDuration func(string) bool) ([]matching.Track, error) {
	service, err := r.factory.GetYouTubeService(ctx)
	if err != nil {
		return nil, err
	}
	return platform.YouTubePlaylistTracksFor(ctx, service, id, needDuration)
}

// observeBackup snapshots a backed-up playlist from tracks already fetched. A failure is
// logged and never fails the sync.
func (j *AnalysisJob) observeBackup(ctx context.Context, service, playlistID, name string, tracks []matching.Track) {
	if j.backups == nil {
		return
	}
	created, err := j.backups.Observe(ctx, service, playlistID, name, tracks)
	if err != nil {
		j.logger.Warn().Err(err).Str("service", service).Str("playlist_id", playlistID).Msg("backup snapshot during sync failed")
		return
	}
	if created {
		j.logger.Info().Str("service", service).Str("playlist_id", playlistID).Int("tracks", len(tracks)).Msg("playlist backup updated during sync")
	}
}

// rememberPairs links tracks already present on both sides, so later runs (and other
// mappings holding the same songs) decide presence by ID. Existing links are kept.
func (j *AnalysisJob) rememberPairs(ctx context.Context, pairs []matchedPair) error {
	store := &tracklinks.Store{DB: j.db}
	for _, p := range pairs {
		if err := store.LinkAuto(ctx, p.spotifyID, p.youtubeID, p.title, p.artist, p.score); err != nil {
			return err
		}
	}
	return nil
}

func toTrackInfos(tracks []matching.Track) []TrackInfo {
	out := make([]TrackInfo, 0, len(tracks))
	for _, t := range tracks {
		info := TrackInfo{ID: t.ID, Title: t.Title, Artist: t.Artist, DurationMs: t.DurationMs}
		if t.Platform == matching.PlatformYouTube {
			// YouTube titles are re-parsed from the raw upload title together with the channel.
			info = TrackInfo{ID: t.ID, Title: t.RawTitle, RawTitle: t.RawTitle, Channel: t.Channel, DurationMs: t.DurationMs}
		}
		out = append(out, info)
	}
	return out
}

type syncItemsGenerateResult struct {
	items          []model.SyncItems
	matchedSkipped int
	// unavailable holds "service|sourceTrackID" keys for songs marked as not on service.
	unavailable map[string]bool
	// pairs are confident matches between the two playlists, remembered as track links.
	pairs []matchedPair
}

type matchedPair struct {
	spotifyID, youtubeID, title, artist string
	score                               float64
}

// generateSyncItems queues an add for every track missing from the other playlist. A
// remembered link decides presence by ID before fuzzy matching, so a track Spotube copied is
// never copied back, and a song marked "not on <service>" is never queued.
func (j *AnalysisJob) generateSyncItems(mapping model.Mappings, spotifyTracks, youtubeTracks []TrackInfo, links *tracklinks.Index) syncItemsGenerateResult {
	var syncItems []model.SyncItems
	matchedSkipped := 0
	unavailable := map[string]bool{}
	var pairs []matchedPair
	spotifyIDs := map[string]bool{}
	for _, t := range spotifyTracks {
		spotifyIDs[t.ID] = true
	}
	youtubeIDs := map[string]bool{}
	for _, t := range youtubeTracks {
		youtubeIDs[t.ID] = true
	}
	// linkDecides reports whether a link settles the source track: present in dest, or
	// known to be unavailable there.
	linkDecides := func(sourceService, sourceID, destService string, destIDs map[string]bool) bool {
		link, ok := links.For(sourceService, sourceID)
		if !ok {
			return false
		}
		target, missing := link.Counterpart(destService)
		if missing {
			unavailable[destService+"|"+sourceID] = true
			matchedSkipped++
			return true
		}
		if destIDs[target] {
			matchedSkipped++
			return true
		}
		return false
	}
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
		return syncItemsGenerateResult{items: syncItems, unavailable: unavailable}
	}

	for _, track := range spotifyTracks {
		if linkDecides("spotify", track.ID, "youtube", youtubeIDs) {
			continue
		}
		if track.Title == "" {
			continue // an add recorded in the cache by ID only; nothing to search with
		}
		source := matching.SpotifyTrack(track.ID, track.Title, track.Artist)
		source.DurationMs = track.DurationMs
		decision := matching.FindMatch(source, youtubeMatching)
		if decision.Matched {
			matchedSkipped++
			pairs = append(pairs, matchedPair{source.ID, decision.BestMatch.ID, source.Title, source.Artist, decision.Score})
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
		if linkDecides("youtube", track.ID, "spotify", spotifyIDs) {
			continue
		}
		if track.Title == "" && track.RawTitle == "" {
			continue // an add recorded in the cache by ID only; nothing to search with
		}
		rawTitle := track.RawTitle
		if rawTitle == "" {
			rawTitle = track.Title
		}
		source := matching.YouTubeTrack(track.ID, rawTitle, track.Channel)
		source.DurationMs = track.DurationMs
		decision := matching.FindMatch(source, spotifyMatching)
		if decision.Matched {
			matchedSkipped++
			best := decision.BestMatch
			pairs = append(pairs, matchedPair{best.ID, source.ID, best.Title, best.Artist, decision.Score})
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

	return syncItemsGenerateResult{items: syncItems, matchedSkipped: matchedSkipped, unavailable: unavailable, pairs: pairs}
}

func trackInfosToYouTubeMatching(tracks []TrackInfo) []matching.Track {
	out := make([]matching.Track, 0, len(tracks))
	for _, track := range tracks {
		raw := track.RawTitle
		if raw == "" {
			raw = track.Title
		}
		t := matching.YouTubeTrack(track.ID, raw, track.Channel)
		t.DurationMs = track.DurationMs
		out = append(out, t)
	}
	return out
}

func trackInfosToSpotifyMatching(tracks []TrackInfo) []matching.Track {
	out := make([]matching.Track, 0, len(tracks))
	for _, track := range tracks {
		t := matching.SpotifyTrack(track.ID, track.Title, track.Artist)
		t.DurationMs = track.DurationMs
		out = append(out, t)
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
				// A song that left the source and came back is wanted again.
				if item.Operation == "add" {
					res, uerr := j.db.ExecContext(ctx,
						`UPDATE sync_items SET status = 'pending', attempt_count = 0, error_message = NULL,
						   analysis_context_json = ?, updated = ?
						 WHERE mapping_id = ? AND service = ? AND operation = 'add' AND track_id = ?
						   AND status = 'skipped' AND error_message = 'no longer in source playlist'`,
						item.AnalysisContextJSON, time.Now().Unix(), item.MappingID, item.Service, lo.FromPtr(item.TrackID))
					if uerr != nil {
						return inserted, uerr
					}
					if n, _ := res.RowsAffected(); n > 0 {
						inserted++
						continue
					}
				}
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

// reconcileQueuedAdds skips pending/error add items that this analysis no longer
// wants: the track is now on the destination, or it left the source playlist.
func (j *AnalysisJob) reconcileQueuedAdds(ctx context.Context, mappingID string, wanted []model.SyncItems, spotifyTracks, youtubeTracks []TrackInfo, unavailable map[string]bool) error {
	key := func(service, trackID string) string { return service + "|" + trackID }
	stillWanted := map[string]bool{}
	for _, item := range wanted {
		if item.Operation == "add" {
			stillWanted[key(item.Service, lo.FromPtr(item.TrackID))] = true
		}
	}
	// add→youtube items come from Spotify tracks and vice versa.
	inSource := map[string]bool{}
	for _, t := range spotifyTracks {
		inSource[key("youtube", t.ID)] = true
	}
	for _, t := range youtubeTracks {
		inSource[key("spotify", t.ID)] = true
	}

	rows, err := j.db.QueryContext(ctx,
		`SELECT id, service, track_id FROM sync_items
		 WHERE mapping_id = ? AND operation = 'add' AND status IN ('pending', 'error', 'review')`, mappingID)
	if err != nil {
		return err
	}
	type stale struct{ id, reason string }
	var staleItems []stale
	for rows.Next() {
		var id, service string
		var trackID sql.NullString
		if err := rows.Scan(&id, &service, &trackID); err != nil {
			rows.Close()
			return err
		}
		k := key(service, trackID.String)
		if stillWanted[k] {
			continue
		}
		reason := "already in destination playlist"
		switch {
		case !inSource[k]:
			reason = "no longer in source playlist"
		case unavailable[k]:
			reason = "not on " + serviceLabel(service)
		}
		staleItems = append(staleItems, stale{id, reason})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	now := time.Now().Unix()
	for _, s := range staleItems {
		if _, err := j.db.ExecContext(ctx,
			`UPDATE sync_items SET status = 'skipped', error_message = ?, updated = ? WHERE id = ?`,
			s.reason, now, s.id); err != nil {
			return err
		}
	}
	if len(staleItems) > 0 {
		j.logger.Info().Str("mapping_id", mappingID).Int("skipped", len(staleItems)).Msg("reconciled stale queue items")
	}
	return nil
}

// activityLogRetention is how long activity log rows are kept.
const activityLogRetention = 30 * 24 * time.Hour

// pruneActivityLogs deletes activity logs older than activityLogRetention.
func (j *AnalysisJob) pruneActivityLogs() {
	cutoff := time.Now().Add(-activityLogRetention).Unix()
	res, err := j.db.Exec("DELETE FROM activity_logs WHERE created < ?", cutoff)
	if err != nil {
		j.logger.Error().Err(err).Msg("failed to prune activity logs")
		return
	}
	if n, _ := res.RowsAffected(); n > 0 {
		j.logger.Info().Int64("deleted", n).Msg("pruned old activity logs")
	}
}

func (j *AnalysisJob) recordSyncResult(mappingID, status, errMsg string, next *time.Time) error {
	now := time.Now().Unix()
	query := "UPDATE mappings SET last_analysis_at = ?, last_sync_status = ?, last_sync_error = ?, updated = ?"
	args := []any{now, status, nullableErrorMessage(errMsg), now}
	if next != nil {
		query += ", next_run_at = ?"
		args = append(args, next.Unix())
	}
	_, err := j.db.Exec(query+" WHERE id = ?", append(args, mappingID)...)
	return err
}

// updateMappingCounts stores each side's size; after a full YouTube read it also resets the
// itemCount the next probe compares against.
func (j *AnalysisJob) updateMappingCounts(ctx context.Context, mappingID string, spotifyCount, youtubeCount int, youtubeRead bool) error {
	now := time.Now().Unix()
	query := "UPDATE mappings SET spotify_tracks_count = ?, youtube_tracks_count = ?, tracks_count = ?, updated = ?"
	args := []any{spotifyCount, youtubeCount, max(spotifyCount, youtubeCount), now}
	if youtubeRead {
		query += ", youtube_item_count = ?, last_full_read_at = ?"
		args = append(args, youtubeCount, now)
	}
	_, err := j.db.ExecContext(ctx, query+" WHERE id = ?", append(args, mappingID)...)
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
	Channel    string
	DurationMs int
}

func stringPtr(s string) *string {
	return &s
}
