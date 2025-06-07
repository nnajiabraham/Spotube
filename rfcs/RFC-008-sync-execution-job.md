# RFC-008: Sync Execution Job (Worker Processing Queue)

**Status:** Draft  
**Branch:** `rfc/008-sync-execution`  
**Depends On:**
* RFC-007 (sync_items queue filled by analysis)
* OAuth helpers (RFC-004 Spotify, RFC-005 YouTube)

---

## 1. Goal

Implement a background worker that consumes `sync_items` records, performs the necessary Spotify / YouTube API mutations, handles retry with exponential back-off, and marks items `done` or `error`.  This will complete the end-to-end automatic bi-directional sync cycle.

## 2. Background & Context

The analysis job now produces `sync_items` such as:
```
{ mapping_id, service:"spotify", action:"add_track", payload:{"track_id":"abc"}, status:"pending" }
```
Execution must:
1. De-queue pending items respecting per-service API rate-limits.
2. Call the appropriate platform API.
3. Persist success/failure.
4. Re-enqueue (status=`error`) with nextAttemptAt timestamp for retry.

PocketBase jobs can be long-running goroutines; but to avoid blocking analysis we register a **continuous worker** listening every 5 s.

## 3. Technical Design

### 3.1 sync_items Additional Fields
Add via migration:
| field | type | default |
|-------|------|---------|
| `next_attempt_at` | `date` | `created` |
| `attempt_backoff_secs` | `number` | 30 |

Back-off formula: `attempt_backoff_secs = min( 2^attempts * 30 , 3600 )` (capped 1 h).

### 3.2 Worker Registration
`backend/internal/jobs/executor.go`
```go
func RegisterExecutor(app *pocketbase.PocketBase) {
  jobs.NewScheduler(app).Every(5 * time.Second).JobFn(func(ctx context.Context) error {
     return ProcessQueue(app, ctx)
  })
}
```
Called from `pbapp.SetupApp()` **after** analysis registration.

### 3.3 ProcessQueue Logic
1. Query up to `BATCH_SIZE=50` items where `status="pending" AND next_attempt_at <= now` order by `created`.
2. For each item spawn goroutine limited by `workerPool := semaphore.NewWeighted(MAX_CONCURRENCY)` (e.g., 5).
3. Execute handler determined by `service + action`:
   * `spotify:add_track`  → `spotifyClient.AddTracksToPlaylist()`
   * `youtube:add_track`  → `youtubeService.PlaylistItems.Insert()`
   * `spotify:rename_playlist` → `spotifyClient.ChangePlaylistName()`
   * ... (removal actions reserved for later RFC).
4. On **success**: set `status="done"`, increment `attempts`.
5. On **rate-limit (HTTP 429)**: leave status `pending`, compute back-off to `next_attempt_at`.
6. On **other error (5xx/4xx)**: set `status="error"`, store `last_error` (truncated 512 chars).  Scheduler will pick up but require manual reset.

### 3.4 Rate-Limit Awareness
Spotify: 50 req/s global bucket (with bursts) – we conservatively process 10 per second.
YouTube: Quota cost – add track is 50 units; daily limit 10 000.  Worker tracks day-bucket counts via `sync.Map` with reset at UTC midnight; when exhausted marks items `skipped` with `last_error="quota"`.

### 3.5 Idempotency & Conflict
* Additions may already exist; APIs return 400/409 – treat as success and mark `done`.
* Renames that already match target title treated as success.

### 3.6 Makefile Target
```makefile
# run analysis+executor continuously (dev)
backend-workers:
	cd backend && PORT=8090 air -c .air.workers.toml
```
`workers` config reloads when `.go` files in `internal/jobs` change.

### 3.7 Admin UI / Logging
* Create `/api/admin/sync-queue` protected endpoint returning aggregate counts (`pending`, `running`, `error`).  Frontend dashboard will visualise in RFC-010.
* Use Zerolog; errors forwarded to Sentry when ENV=prod (RFC-012).

## 4. Testing Strategy

### Backend
* Use PocketBase test harness + **httpmock**:
  * Mock successful `add_track` call (200) ⇒ item becomes `done`.
  * Mock 429 ⇒ item remains `pending`, backoff doubled.
  * Mock fatal 400 ⇒ item `error`.
* Test batch selection respects `next_attempt_at`.
* Test quota counter for YouTube: supply > daily limit and expect `skipped` status.

### Frontend
No UI changes in this RFC (visualisation deferred).  Playwright smoke test will be added in RFC-010.

## 5. Dependencies
* `golang.org/x/sync/semaphore` – goroutine pool
* `github.com/jarcoal/httpmock` – tests

## 6. Checklist
- [ ] **E1** Migration: add `next_attempt_at` & `attempt_backoff_secs` fields to `sync_items`.
- [ ] **E2** Implement executor job, rate-limit handling, back-off.
- [ ] **E3** Add YouTube quota tracker helper.
- [ ] **E4** Tests covering success, back-off, quota, fatal error.
- [ ] **E5** Add Makefile `backend-workers` target.
- [ ] **E6** README worker section & ENV vars (e.g., `WORKER_CONCURRENCY`).

## 7. Definition of Done
* Worker processes pending items, successfully adding tracks & renames.
* Rate-limit and back-off logic proven by tests.
* Sync queue shrinks over time during steady-state.

## Resources & References
* Spotify Add Tracks – https://developer.spotify.com/documentation/web-api/reference/add-tracks-to-playlist
* YouTube PlaylistItems.Insert – https://developers.google.com/youtube/v3/docs/playlistItems/insert
* PocketBase Job Scheduler – https://pocketbase.io/docs/go-jobs-scheduling/
* httpmock – https://github.com/jarcoal/httpmock

---

*End of RFC-008* 