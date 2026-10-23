# RFC-013: Quota budget, schedules, runtime settings and bulk pairing

- **Status:** Accepted and implemented (revised after design review, see §7)
- **Date:** 2026-10-23
- **Scope:** `internal/quota`, `internal/schedule`, `internal/settings` (all new), the sync
  scheduler, analysis and executor, the YouTube transport, mappings API and pages, a Settings
  page, a Mirror page, the dashboard quota panel

## 1. Problem

The user will pair many playlists with thousands of songs each, and some songs exist on only
one service. Before this change:

- **YouTube quota is the bottleneck, and nothing planned for it.** The Data API gives 10,000
  units per day (resets at midnight Pacific). Adding a song Spotube has not matched before
  costs about 151 units (search 100 + videos.list 1 + insert 50); a known song costs 50. So
  roughly 66 new songs per day can reach YouTube. A 2,000-song Spotify playlist takes about a
  month to copy.
- **Reads were not free either, and they grow with playlist size.** Every analysis read both
  playlists in full: a YouTube read is about 1.5 units per 50 videos (items + durations). Ten
  2,000-video mappings analysed hourly would spend more than the whole quota on reads alone.
- **Running out was handled badly.** A `quotaExceeded` answer marked the item `error` and
  counted an attempt. After 3 such days an item was never retried, though nothing was wrong.
- **No spreading.** The first mapping that was due took up to 25 items per run, regardless of
  other mappings, the user's in-app searches, or how much quota was left.
- **No estimate.** Pairing a big playlist gave no idea of the cost or how long it would take.
- **Schedules collided.** Every mapping ran "every N minutes" from its last run, with no set
  time or days and no advice on what to pick.
- **Configuration was only in `.env`** and needed a restart.

Also in scope (from review of the previous change):

- Mappings showed `tracks_count = max(spotify, youtube)`, e.g. "3 tracks" for a mapping whose
  Spotify side was empty.
- Deleting a mapping could not delete its playlists.
- YouTube artwork was blank on Review cards.
- The activity feed logged "Analysis completed, found 0 sync items" on every run.
- The user wants to choose "sync now" when creating a mapping.
- "Mirror": pick many playlists on one service and create their counterparts in one go, as
  individual 1:1 mappings. A playlist already used in a mapping must be flagged and refused.

Out of scope: syncing removals (the user ruled it out: sync and backups have different
purposes), notifications, app login, Docker, dark mode.

## 2. YouTube cost model

| Operation | Units | Notes |
|---|---|---|
| `search.list` | 100 | plus `videos.list` (1) for durations: **101** per search |
| `playlistItems.insert` | 50 | one track |
| `playlists.insert` / `delete` / `update` | 50 | create, delete, rename |
| `playlistItems.list` | 1 | per 50 items |
| `videos.list` | 1 | per 50 IDs (durations) |
| `playlists.list` | 1 | names and `itemCount`: the change probe |

A full read of `n` videos costs `pages + ceil(pages/2)` where `pages = ceil(n/50)`
(`quota.FullReadUnits`): items, plus durations for the videos that still need them. A YouTube
playlist holds at most **5,000** videos.

Spotify has no daily quota, only a rolling rate limit (HTTP 429 with `Retry-After`), which the
client backs off from.

## 3. Design

```
       ┌──────────────── scheduler tick (every minute) ─────────────────┐
       │ 1. if "background sync" is on: due mappings (next_run_at ≤ now) │
       │    → probe YouTube (1 unit) → full read only if it changed      │
       │ 2. if "apply automatically" is on: drain the queue across all   │
       │    mappings, round-robin, while the budget allows               │
       └─────────────────────────────────────────────────────────────────┘
                 │ every YouTube request, from anywhere, goes through
                 ▼
       apiusage.Transport ── charges quota.Budget before sending
                             refunds requests that never reached Google
                             and 403 quota refusals
```

### 3.1 The budget (`internal/quota`)

One process-wide `Budget` decides every YouTube call. It is enforced in the HTTP transport
behind `ClientFactory.GetYouTubeService`, so analysis, the executor, search, playlist creation,
backups and restores are all covered without each caller remembering to ask.

- **Counting.** Usage is an in-memory counter, charged *before* the request is sent and seeded
  from `api_usage` at startup and whenever the quota day (Pacific) changes. `api_usage` is
  still written per call, for persistence and the dashboard. Charging up front means concurrent
  callers cannot overshoot together, and there is no window where a finished call is counted
  nowhere.
- **Refunds.** A request that never reached Google (network error, cancelled context) and a
  403 `quotaExceeded`/`dailyLimitExceeded` answer are refunded, so the counter does not
  overstate usage once quota runs out.
- **Classes**, passed in the request context (`quota.WithClass`):

  | Class | Used by | May use up to |
  |---|---|---|
  | `Interactive` (default) | search, Sync now's read, creating/deleting playlists, restore | limit − 50 (safety margin) |
  | `Read` | scheduled analysis and backup reads | limit − interactive reserve |
  | `Drain` | applying the queue (also after Sync now) | limit − reserve − scheduled reads still expected today |

  "Reads still expected" is today's estimated scheduled read units (`DailyReadUnits` summed over
  mappings, §3.6) times the share of the quota day still to come. This keeps the drain from
  spending the whole quota at midnight and starving the evening's scheduled checks.
- **Whole-job pre-checks.** Multi-call jobs ask `Budget.Allows(class, units)` first: a full
  read checks `FullReadUnits(n)`, an add for an unmatched song checks 151 (search + insert), a
  linked add 50, an in-app search 101. A job that cannot finish does not start, so a search is
  never paid for and then wasted on a refused insert.
- **Exhausted latch.** A Google `quotaExceeded`/`dailyLimitExceeded` latches "exhausted" until
  `min(now + 30 min, reset)`: Google's count can differ from ours (another app on the same
  project; resets that are not exactly at midnight), so it is re-checked instead of trusted all
  day. Per-minute `rateLimitExceeded`/`userRateLimitExceeded` do not latch; the item is
  deferred and retried.
- `Status()` (used by `GET /api/quota`): used, limit, reserve, daily scheduled reads and the
  part still to come, available (interactive), background available (drain), exhausted, retry
  time, reset time, backlog and days to drain.
- The limit and reserve are settings (§3.7) read on every decision, so changes apply at once.

### 3.2 Executor: defer, don't fail

- A budget refusal, a Google quota error, a rate limit (429 or `rateLimitExceeded`) or a
  cancelled context is a **deferral**: the item goes back to `pending` with its attempt count
  unchanged and a message such as "Waiting for YouTube quota". YouTube work stops for the rest
  of the pass; Spotify items keep going.
- **Playlist full** (`playlistContainsMaximumNumberOfVideos`): the item is skipped with a
  reason, and YouTube adds for that mapping pause (status `playlist_full`). Spotify adds for it
  continue.
- **Playlist missing** (404): the mapping's items stop for the pass and the mapping shows
  `playlist_missing`.
- **409 counts as success only when the reason says the video is already there**, not for any
  409 (YouTube also answers 409 for conflicting writes).
- **The match is remembered before the insert.** A search result is stored as a track link
  first, so if the insert is then refused, the next attempt costs 50 units, not 151.
- **Retries back off:** an `error` item waits `15 min × 4^(attempt−1)` before the next try
  (15 min, then 1 h), at most 3 attempts.
- **Startup:** items left `running` by a crash or shutdown go back to `pending`.
- A migration resets items that failed only because of quota (`quotaExceeded`,
  `dailyLimitExceeded`, "exceeded your quota") to `pending` with `attempt_count = 0`.

### 3.3 Analysis: probe first, read YouTube only on change

Most runs find nothing new.

- **Spotify is always read in full.** It is free, and its `snapshot_id` changes on Spotube's
  own adds, so it would not save anything.
- **YouTube is probed** with `playlists.list` (1 unit): title and `itemCount`. A full read
  happens only when:
  - the count differs from `mappings.youtube_item_count`, which the executor bumps on each of
    its own inserts (so Spotube's own writes do not look like changes);
  - there is no cached read; or
  - the cached read is older than `FullReadEvery(n)`: a day per 1,000 videos, at most 7 days
    (catches a swap that keeps the count). Sync now uses 10 minutes, so it reads fresh.
- **The read is cached in SQLite** (`playlist_cache`), so a restart does not cost a full read,
  and the executor updates it as it inserts.
- **Durations only for unlinked videos.** Durations only help match unknown videos, so
  `videos.list` runs only for videos with no track link.
- **Budgeted.** A scheduled full read is `Read` class and pre-checked; if refused, the mapping
  is stamped `waiting_quota` and its next run is moved to when the quota allows (the reset or
  the latch re-check), if that is sooner than its next slot, so a daily mapping does not lose a
  day.
- **Interrupted runs** (server stopping) leave the mapping's status and next run untouched, so
  it simply runs again after the restart.
- **Quiet feed.** Analysis logs to the activity feed only when it queued something or failed
  (repeat failures with the same message are logged once). Unchanged probes log at debug level.
- A song skipped as "no longer in the source" is brought back to `pending` if it is wanted
  again.

### 3.4 Draining the queue

Analysis only queues (free); the drain applies. It runs on every tick when "apply
automatically" is on, **independently of the background-sync toggle**, so a multi-day copy
keeps going.

- Order: renames, then items whose song already has a track link (50 units), then the rest;
  within each group, round-robin across mappings (each mapping's oldest first), so one huge
  playlist cannot starve the others.
- At most `drain_items_per_tick` items per tick (setting, default 100), so a tick stays short.
- **One shared runner.** The scheduler and Sync now use the same `AnalysisJob`, with one
  per-mapping lock and one cache. A mapping being synced by one is skipped by the other.
- **No duplicate work:** the unique index on `sync_items (mapping, service, operation,
  track_id)` prevents queueing the same add twice; item claiming is atomic; mappings are 1:1
  (unique indexes on both playlist ID columns); a search result is saved as a track link, so the
  same song in another mapping costs 50, not 151.

### 3.5 Estimates and progress

- `GET /api/quota`: status (§3.1), the YouTube backlog (unlinked × 151, linked × 50, renames ×
  50) and **days to drain** at today's background capacity.
- `POST /api/quota/estimate` with proposed pairs (`spotify_tracks`, `youtube_videos`,
  `create`, optional `spotify_playlist_id`): units split into searching, adding, reading and
  creating, and the days to finish behind the existing backlog. For a Spotify playlist being
  copied, Spotube reads it (free) and counts songs that already have a YouTube link, so the
  figure is near-exact; otherwise it is marked "up to" (songs already on both sides cost
  nothing). It warns when a YouTube playlist would pass 5,000 videos.
- Mappings list: per-side track counts (`spotify_tracks_count`, `youtube_tracks_count`,
  replacing the misleading max), queue counts (pending, review, error, done, skipped),
  `next_run_at`, and `syncing` while a first sync runs.

### 3.6 Schedules and recommendations (`internal/schedule`)

One model: **every N minutes, starting at HH:MM, on days D, in time zone Z**, restarting each
day. A daily schedule is N = 1440. Stored on the mapping as `interval_minutes`,
`schedule_start`, `schedule_days` (bitmask, Sunday = 1 … Saturday = 64; 127 = every day),
`schedule_tz` (from the browser) and `next_run_at`.

- `Next(after)` walks run times day by day in the schedule's zone. Times inside a DST gap
  (02:30 on spring-forward day) run at the first valid minute after, never early.
- `next_run_at` is set after each scheduled run (not after Sync now). A mapping without one
  (existing mappings, which migrate to "from 00:00, every day") is due when a run fell since
  its last analysis; after downtime it runs once, not once per missed slot.
- **Recommendation** (`GET /api/collections/mappings/records/schedule-recommendation`):
  1. Interval by size: ≤ 300 songs → 30 min; ≤ 1,500 → 2 h; ≤ 5,000 → 6 h; larger → daily.
  2. Load: while all mappings' daily read units (`DailyReadUnits`: one probe per run plus about
     one full read a day) would exceed 15% of the daily limit, step up (30 m → 1 h → 2 h → 6 h
     → 12 h → daily). If even daily is over, it says so in a warning.
  3. Spacing: pick the start time (5-minute steps) whose runs are furthest from every other
     mapping's runs over a day; daily schedules prefer around 02:00.
- The pairing form applies the recommendation until the user edits the schedule, and offers
  "Use recommended" to go back. Interval, start time and days can always be overridden.

### 3.7 Runtime settings (`internal/settings`, Settings page)

| Key | Env var | Default |
|---|---|---|
| `sync_workers_enabled` (Background sync) | `SYNC_WORKERS_ENABLED` | false |
| `auto_apply_enabled` (Apply changes automatically) | `SYNC_EXECUTOR_AUTO_ENABLED` | true |
| `sync_new_mappings_now` | — | true |
| `drain_items_per_tick` | — | 100 (1–1,000) |
| `youtube_daily_quota` | `YOUTUBE_DAILY_QUOTA` | 10,000 |
| `youtube_interactive_reserve` | — | 500 |
| `backups_enabled` | `BACKUPS_ENABLED` | true |

Resolution, as the user asked:

- **An env var set at startup wins.** The Settings page can still change it; the change lives
  in memory until the next restart, when the `.env` value returns. The page marks these
  ".env" / "until restart" and says so.
- **Otherwise the value is stored in SQLite** (`app_settings`) and survives restarts; with
  nothing stored, the default applies. (If an env var is later removed, a stored value comes
  back. That is intended: `.env` is a startup override.)
- `GET /api/settings` lists value, default, source (`default`, `env`, `database`, `runtime`),
  limits and whether the value is env-pinned. `PATCH /api/settings` validates every value before
  applying any.
- Changes apply live: the scheduler and backup ticker always run and check their toggles on
  every tick; the budget reads the limit and reserve on every decision. The cron tick spec
  stays env-only.

### 3.8 Mapping API changes

- **1:1 only.** Unique indexes on `spotify_playlist_id` and `youtube_playlist_id`; creating a
  mapping with a playlist already paired returns 409, and every picker greys it out as
  "Already paired".
- **Create** gains the schedule fields and `sync_now` (default from `sync_new_mappings_now`):
  the first sync starts in the background on the shared runner; its read is interactive, its
  apply is `Drain` class so a big first copy cannot eat the quota kept for searches. If the
  mapping insert fails after Spotube created a playlist (e.g. a conflict), the new playlist is
  deleted again.
- **Delete** takes `delete_spotify_playlist` / `delete_youtube_playlist` (both default false):
  Spotify unfollows (for playlists you don't own, that only removes it from your library);
  YouTube deletes (50 units). The mapping is deleted first; each playlist's result is reported.
- **Mirror** (`POST /api/collections/mappings/records/mirror`): a source service and up to 100
  playlists. Each becomes its own mapping with the other side created and its own recommended,
  spaced schedule. Already-paired playlists come back `already_paired`; once YouTube quota runs
  short, the rest come back `waiting_quota` without creating anything. Partial success is
  normal; the response lists each result.
- **Search** (`/api/tracks/search` on YouTube) pre-checks 101 interactive units and answers 429
  with the reset time when refused.

### 3.9 Small fixes

- **Artwork:** YouTube thumbnails come from the video ID (`i.ytimg.com/vi/<id>/mqdefault.jpg`,
  the URL the API returns), so cards without stored artwork show it at no quota cost.
- **Track counts:** per side, as above.

## 4. Edge cases

| Case | Behaviour |
|---|---|
| Quota spent elsewhere on the same Google project | Google's refusal latches the budget for 30 minutes (or until the reset); items deferred, not failed |
| Midnight PT passes mid-drain | Budget rolls over to the new day's recorded usage; latch clears at the reset |
| Server restarts mid-day | Usage reloads from `api_usage`; playlist reads from `playlist_cache`; `running` items go back to `pending`; interrupted analyses run again |
| Sync now while the scheduler syncs the same mapping | Shared per-mapping lock; the scheduler skips it, Sync now reports it is already syncing |
| In-app search when the drain has used its share | Allowed up to the limit minus 50; background is refused first |
| In-app search when exhausted | 429 with the reset time |
| Song on neither service | One search; it lands in Review and is not searched again while it is there |
| Same song in many mappings | First search stores a link; later adds cost 50 |
| Playlist edited during a long drain | The count no longer matches → full read → reconcile skips items no longer wanted |
| YouTube playlist reaches 5,000 | Item skipped with the reason; YouTube adds for that mapping pause (`playlist_full`); estimate warns beforehand |
| YouTube playlist deleted | 404 → mapping shows `playlist_missing`; its items wait |
| User lowers the daily limit below today's usage | Background and interactive refused; UI shows 0 available |
| Env-pinned setting changed in UI, then restart | Reverts to the `.env` value (shown in UI) |
| Mirror of 50 playlists, quota for 20 creations | First 20 created; rest `waiting_quota`, still selected to retry after the reset |
| DST spring-forward with a start inside the gap | Runs at the first valid minute after |
| Spotify 429 | Client waits `Retry-After`; the item is deferred if the wait is cut short |

## 5. Testing

- `quota`: classes, reads still expected, charge/refund, latch and re-check, day rollover from
  `api_usage`, transport gating/refunds/rate limits, estimates.
- `schedule`: validation, `Next` across days, day masks, time zones and the DST gap, runs and
  reads per day, recommendation tiers, load stepping, spacing.
- `settings`: env vs database vs runtime precedence, validation, ranges, registry.
- Executor: YouTube blocked while Spotify continues, whole-job pre-checks, playlist full and
  missing, rate-limit and cancel deferrals, remember-before-insert, item-count bump, backoff,
  drain ordering, busy lock and per-tick limit, `ResetInterrupted`, 409 semantics.
- Analysis: probe skip, cache survives restart, changed count, stale cache, own inserts, durations
  only for unlinked, budget refusal, retry at reset, next run vs Sync now, missing/full status,
  re-queue, quiet logs, interrupted run.
- Handlers: schedules and `sync_now`, validation, cleanup on conflict, delete options, list
  counts and queue, recommendation, mirror, settings API, quota API.
- Frontend: schedule labels, pairing with recommendation and overrides, delete dialog, mirror
  flow (paired flagged, partial results), Settings page (sources, toggles, errors).
- Live: probe-only runs, drain across mappings with deferrals on shutdown, quota panel numbers
  against `api_usage`.

## 6. Bugs found while testing the implementation

- **Rollover used the wall clock.** The budget loaded "today's" usage with `time.Now()` rather
  than its own clock, so a rollover computed for another day read the wrong row. It now loads
  `apiusage.UsedOn(day)`.
- **DST gap ran early.** A 02:30 start on spring-forward day normalised to 01:30. It now
  shifts forward to the first valid minute.
- **Full-playlist pause stopped Spotify too.** Pauses are now per mapping *and service*.
- **A quota-refused daily run waited a whole day.** It now retries at the reset when that is
  sooner.
- **Search bypassed the budget pre-check** and could start with too little quota; it now
  pre-checks 101 units.
- **Shutdown marked the running analysis "Failed: context canceled".** Interrupted runs no
  longer record a result.

## 7. Design review: findings and responses

An independent review of the first draft (subagent, read-only) raised these; all but the last
are addressed in the design above.

| Finding | Response |
|---|---|
| Cached usage + reservation release leaves a window where units are counted nowhere; `Record` ignores DB errors | In-memory counter charged before sending, seeded from `api_usage`; DB is persistence only |
| Only the executor was gated; analysis, backups, creation, search call YouTube directly | Gate moved into the transport behind `ClientFactory`; class via context |
| Multi-call jobs need one decision; search units wasted if insert refused | `Allows` pre-checks for whole jobs; match remembered before the insert |
| Exhausted latch too blunt (rate limit vs quota; reset not exactly midnight) | Only quota reasons latch; re-checked after 30 minutes |
| Transport charged refused requests | Refunds for unsent requests and quota refusals |
| Helpers used the hard-coded 10,000 | Everything reads the `youtube_daily_quota` setting through the budget |
| Spotube's own adds would trigger full reads | Executor bumps `youtube_item_count`; Spotify always read in full (free) |
| In-memory playlist cache lost on restart | Persisted in `playlist_cache` |
| Forced 24 h re-read too expensive for big playlists | `FullReadEvery`: a day per 1,000 videos, max 7 |
| Two `AnalysisJob` instances, no shared locks | One shared runner for the scheduler and Sync now |
| Offset scheduling breaks for intervals not dividing 1,440; tz/DST unspecified | One model (every N from HH:MM on days, restarting daily) in the mapping's tz; DST handled |
| `next_run_at` after downtime / for existing mappings | Due when a run fell since the last analysis; runs once |
| Recommendation has no answer when even daily is over budget | Returns daily with a warning |
| YouTube 5,000-video cap | Skip + per-service pause + estimate warning |
| Any 409 treated as success | Only when the reason says it is already there |
| 404 playlist should pause, not burn attempts | `playlist_missing` |
| Items stuck `running` after a crash | Reset on startup |
| Retries without backoff | 15 min, then 1 h, before the 2nd and 3rd attempts |
| Skipped items never re-queued | Re-queued when wanted again |
| Quota migration must reset attempts and match `dailyLimitExceeded` | Done |
| Drain only ran with background sync on | Drain depends only on "apply automatically" |
| Mirror could leave orphan playlists | Created playlist is deleted if the mapping insert fails |
| 1:1 was check-then-insert only | Unique indexes |
| Estimate only "up to" | Near-exact for Spotify sources using existing links |
| Projected reads with `p_change`, tickets, two schedule kinds, per-mapping ETAs: over-engineered | Simplified: fixed daily read estimate, charge/refund, one schedule model, a global days-to-drain |
| Thousands of songs on only one service need a bulk "not available" action | **Not done** (not in this request). Each costs one search and waits in Review; a bulk action is a follow-up |
