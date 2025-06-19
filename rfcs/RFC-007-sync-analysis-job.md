# RFC-007: Sync Analysis Job (Scheduled Detection)

**Status:** Completed  
**Branch:** `rfc/007-sync-analysis`  
**Depends On:**
* RFC-006 (playlist mappings collection)
* RFC-004/005 (OAuth tokens & playlist fetch)

---

## 1. Goal

Create a scheduled PocketBase job that routinely inspects each mapping, compares track lists between Spotify and YouTube, and generates **work queue items** (`sync_items` collection) describing the actions needed to reconcile differences.  Actual execution of those diffs happens in RFC-008; this RFC is analysis-only.

## 2. Background & Context

For every mapping we need to know:
* **Additions** – tracks that exist in one platform but are missing from the other platform, in both directions (Spotify to YouTube and YouTube to Spotify)
* **Removals** – tracks no longer present.
* **Renames** – playlist title drift if `sync_name=true`.

We isolate expensive diff logic into a background job (`analysis`) so the execution worker can be simpler and rate-limit friendly.

PocketBase offers a [Go Job Scheduler](https://pocketbase.io/docs/go-jobs-scheduling/) we can leverage.

## 3. Technical Design

### 3.1 New Collection: `sync_items` (This is just a draft, might not be the best DB design for storing this kind of data, figure out a better way)
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
- [X] **A1** Migration for `sync_items` collection.
- [X] **A2** Add `last_analysis_at` & `next_analysis_at` fields to `mappings` collection.
- [X] **A3** Implement analysis job & scheduler registration.
- [X] **A4** Helper functions to fetch track lists + caching.
- [X] **A5** Backend tests for diff logic & record creation.
- [X] **A6** README: document analysis job cadence & env var to tune interval.

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

**A1 COMPLETED** - Created sync_items collection migration:
* Generated migration file `backend/migrations/1750298622_create_sync_items_collection.go` using PocketBase CLI
* Implemented collection schema with all required fields:
  - `mapping_id` (relation to mappings) - required, with cascade delete
  - `service` (select: spotify, youtube) - required 
  - `action` (select: add_track, remove_track, rename_playlist) - required
  - `payload` (json) - optional, for arbitrary action data
  - `status` (select: pending, running, done, error, skipped) - required
  - `attempts` (number, min: 0) - required
  - `last_error` (text) - optional
* Added database indexes for optimal query performance:
  - `idx_sync_items_mapping_id` - for filtering by mapping
  - `idx_sync_items_status` - for filtering by processing status
  - `idx_sync_items_service` - for filtering by target service
* Collection rules set to `null` (only server hooks can access, as specified in RFC)
* Migration successfully applied, creating sync_items table in SQLite database

**A2 COMPLETED** - Added analysis timing fields to mappings collection:
* Generated migration file `backend/migrations/1750298769_add_analysis_fields_to_mappings.go`
* Added two new date fields to mappings collection:
  - `last_analysis_at` (date) - tracks when the mapping was last analyzed
  - `next_analysis_at` (date) - tracks when the mapping should be analyzed next
* Both fields are optional (nullable) to handle new mappings that haven't been analyzed yet
* Migration includes proper rollback logic to remove fields if needed
* Migration successfully applied, extending mappings table schema

**A3 COMPLETED** - Implemented analysis job & scheduler registration:
* Created `backend/internal/jobs/analysis.go` module with complete analysis logic
* Used PocketBase's `tools/cron` package for job scheduling (cron expression: `*/1 * * * *` - runs every minute)
* Implemented core analysis functions:
  - `RegisterAnalysis()` - registers and starts the cron job
  - `AnalyseMappings()` - main analysis logic for all mappings
  - `shouldAnalyzeMapping()` - timing-based mapping filter using next_analysis_at
  - `analyzeMapping()` - per-mapping analysis logic
  - `analyzeTracks()` - bidirectional track diff using samber/lo set operations
  - `analyzePlaylistNames()` - playlist name sync analysis
  - `enqueueSyncItem()` - creates sync_items records for work queue
  - `updateMappingAnalysisTime()` - updates analysis timestamps
* Added placeholder functions `fetchSpotifyTracks()` and `fetchYouTubeTracks()` (marked TODO for A4)
* Registered analysis job in `backend/cmd/server/main.go` with `jobs.RegisterAnalysis(app)`
* Added `github.com/samber/lo` dependency for set operations
* Backend compiles successfully and job scheduler is ready for deployment

**A4 COMPLETED** - Implemented helper functions to fetch track lists:
* Replaced placeholder functions with real API implementations:
  - `fetchSpotifyTracks()` - fetches playlist tracks using Spotify Web API
  - `fetchYouTubeTracks()` - fetches playlist items using YouTube Data API v3
* Created job-specific OAuth helpers (background job compatible):
  - `getSpotifyClientForJob()` - gets authenticated Spotify client without Echo context
  - `getYouTubeServiceForJob()` - gets authenticated YouTube service without Echo context
* Both helpers include automatic token refresh logic with 30-second expiry buffer
* Track data structure includes ID and title for diff comparison
* Added proper imports: `github.com/zmb3/spotify/v2`, `google.golang.org/api/youtube/v3`
* Error handling for authentication failures and API call failures
* Logging for track fetch operations (shows count of tracks fetched per playlist)
* Backend compiles successfully with real track fetching functionality

**A5 COMPLETED** - Backend tests for diff logic & record creation:
* ✅ **ALL TESTS PASS** - Created comprehensive integration test suite with 100% pass rate
* Proper PocketBase integration testing following `googleauth_test.go` patterns:
  - Uses `tests.NewTestApp()` for real PocketBase database operations
  - Creates actual collections (`mappings`, `sync_items`, `oauth_tokens`) with proper schema
  - Tests real functions from `analysis.go` with full database integration
  - HTTP mocking with `httpmock` for Spotify/YouTube API calls
* **Complete test coverage:**
  - `TestAnalyseMappings_Integration` - Full end-to-end analysis workflow ✅
  - `TestShouldAnalyzeMapping_WithPocketBaseRecord` - Timing logic with real records ✅  
  - `TestAnalyzeMapping_NoSyncItems` - Edge case with identical playlists ✅
  - `TestEnqueueSyncItem_Integration` - Sync items creation and retrieval ✅
  - `TestUpdateMappingAnalysisTime_Integration` - Timestamp handling ✅
  - Plus all logic unit tests (5 test functions, 15 subtests) ✅
* **Real integration issues discovered and fixed:**
  - PocketBase filter expression requirements (`id != ''` vs empty string)
  - PocketBase date format handling (`2006-01-02 15:04:05.000Z` vs RFC3339)
  - PocketBase relation fields stored as `[]string` arrays
  - HTTP mocking configuration for background jobs
  - Timezone handling in date comparisons (UTC vs local time)
* **Validates core RFC-007 requirements:** ✅ Analysis job runs, creates sync items, updates timestamps
* **Tests prove proper PocketBase integration:** Real database operations, not mocked/isolated logic

**A6 COMPLETED** - README: document analysis job cadence & configuration:
* Added comprehensive "Sync Analysis & Processing" section to README
* Documented two-phase sync approach (Analysis RFC-007 → Execution RFC-008)
* Explained analysis job schedule: every minute cron with per-mapping intervals
* Detailed analysis process: fetch → diff → work queue → timestamp update
* Documented work item structure and status tracking in `sync_items` collection
* Clarified configuration: no env vars needed, timing controlled via `interval_minutes` UI setting
* Added monitoring guidance: logs, admin UI, timestamp health checks
* Updated existing references to point to new sync documentation section

## Resources & References
* PocketBase Job Scheduler – https://pocketbase.io/docs/go-jobs-scheduling/
* Spotify Get Playlist Tracks – https://developer.spotify.com/documentation/web-api/reference/get-playlists-tracks
* YouTube PlaylistItems List – https://developers.google.com/youtube/v3/docs/playlistItems/list
* lo library (set helpers) – https://github.com/samber/lo
* httpmock – https://github.com/jarcoal/httpmock

---

*End of RFC-007* 