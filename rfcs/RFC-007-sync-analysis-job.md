# RFC-007: Sync Analysis Job (Scheduled Detection)

**Status:** Draft  
**Branch:** `rfc/007-sync-analysis`  
**Depends On:**
* RFC-006 (playlist mappings collection)
* RFC-004/005 (OAuth tokens & playlist fetch)

---

## 1. Goal

Create a scheduled PocketBase job that routinely inspects each mapping, compares track lists between Spotify and YouTube, and generates **work queue items** (`sync_items` collection) describing the actions needed to reconcile differences.  Actual execution of those diffs happens in RFC-008; this RFC is analysis-only.

## 2. Background & Context

For every mapping we need to know:
* **Additions** – tracks present in source but missing in target.
* **Removals** – tracks no longer present.
* **Renames** – playlist title drift if `sync_name=true`.

We isolate expensive diff logic into a background job (`analysis`) so the execution worker can be simpler and rate-limit friendly.

PocketBase offers a [Go Job Scheduler](https://pocketbase.io/docs/go-jobs-scheduling/) we can leverage.

## 3. Technical Design

### 3.1 New Collection: `sync_items`
Migration: `go run cmd/server migrate create "create_sync_items_collection"`
| field | type | notes |
|-------|------|-------|
| `mapping_id` | `relation` → `mappings` | required |
| `service` | `select` (`spotify`, `youtube`) | which platform this action targets |
| `action` | `select` (`add_track`, `remove_track`, `rename_playlist`) | required |
| `payload` | `json` | arbitrary action data |
| `status` | `select` (`pending`, `running`, `done`, `error`, `skipped`) | default `pending` |
| `attempts` | `number` | default 0 |
| `last_error` | `text` | nullable |

Rules: only server hooks can list/view.

### 3.2 Analysis Job Registration
File `backend/internal/jobs/analysis.go`
```go
package jobs

import (
  "context"
  "time"
  "github.com/pocketbase/pocketbase"
  "github.com/pocketbase/pocketbase/jobs"
)

func RegisterAnalysis(app *pocketbase.PocketBase) {
  jobs.NewScheduler(app).Every(1 * time.Minute).JobFn(func(ctx context.Context) error {
      return AnalyseMappings(app, ctx)
  })
}
```
`pbapp.SetupApp()` registers `jobs.RegisterAnalysis(app)`.

### 3.3 AnalyseMappings Algorithm (Bidirectional)
1. Query all mapping records.
2. For each mapping if `now > nextRunAt` (derived from `interval_minutes`) proceed.
3. Fetch track lists for **both** services.
4. Compute union & differences:
   * `toAddOnSpotify   = youtubeTracks – spotifyTracks`
   * `toAddOnYouTube   = spotifyTracks – youtubeTracks`
   * Removals handled only if we decide to prune; for v1 we **don't** remove – only additive.
5. Enqueue `add_track` items with appropriate `service` field.  For name sync:
   * If titles differ and `sync_name=true`, enqueue two `rename_playlist` items – one for each service whose title differs from the chosen _canonical_ title (first non-empty or Youtube by default).

Payload example now:
```json
{"track_id":"3uFJaLSkF7z6Ds"}
```
(service stored separately)

### 3.4 Caching & Performance
* Track lists cached in memory within job run.
* Spotify & YouTube calls already authenticated via helpers from earlier RFCs.

### 3.5 Testing Strategy

#### Backend (Go)
* Use PocketBase test harness + httpmock for Spotify/YouTube endpoints.
* Create fake mapping with simple 3-track diff; run `AnalyseMappings`; assert `sync_items` records produced with correct payload.
* Time manipulation: inject `timeNow` function for deterministic tests.

### 3.6 Checklist
- [ ] **A1** Migration for `sync_items` collection.
- [ ] **A2** Add `last_analysis_at` & `next_analysis_at` fields to `mappings` collection.
- [ ] **A3** Implement analysis job & scheduler registration.
- [ ] **A4** Helper functions to fetch track lists + caching.
- [ ] **A5** Backend tests for diff logic & record creation.
- [ ] **A6** README: document analysis job cadence & env var to tune interval.

## 4. Definition of Done
* Every minute scheduler runs; mappings with elapsed interval generate `sync_items`.
* Diff logic correct for additions/removals/renames.
* Tests pass.

## 5. Implementation Notes / Summary
* We do not yet touch the target services – just enqueue.
* Payload example for `add_track`:
```json
{"track_id":"3uFJaLSkF7z6Ds"}
```
* RFC-008 will consume queue respecting rate limits and retries.

## Resources & References
* PocketBase Job Scheduler – https://pocketbase.io/docs/go-jobs-scheduling/
* Spotify Get Playlist Tracks – https://developer.spotify.com/documentation/web-api/reference/get-playlists-tracks
* YouTube PlaylistItems List – https://developers.google.com/youtube/v3/docs/playlistItems/list
* lo library (set helpers) – https://github.com/samber/lo
* httpmock – https://github.com/jarcoal/httpmock

---

*End of RFC-007* 