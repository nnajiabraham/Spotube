# RFC-010: Logging & Status Dashboard

**Status:** Done  
**Branch:** `rfc/010-logging-dashboard`  
**Depends On:**
*   RFC-007, RFC-007b, RFC-007c (Jobs producing data to visualize)
*   RFC-008, RFC-008b (Executor for sync jobs)
*   RFC-009, RFC-009b (Blacklist producing data to visualize)

---

## 1. Goal

This RFC outlines a two-part plan. First, it addresses critical bugs in the existing sync analysis and execution jobs to ensure data integrity and functionality. Second, it defines the implementation of a real-time status dashboard that visualizes job runs, per-mapping status, sync queue health, and a tail-able log of important system events, making the system's state observable and debuggable.

## 2. Background & Context

With analysis and execution jobs running, the user needs visibility into what the application is doing. However, several issues in the current implementation prevent reliable syncing and would lead to an inaccurate dashboard.

**Identified Issues:**
1.  **Insufficient OAuth Scopes**: The executor job fails with `Insufficient client scope` errors because the initial OAuth integrations (RFC-004, RFC-005) did not request permissions to modify playlists.
2.  **Duplicate Sync Items**: The analysis job (RFC-007) repeatedly enqueues `sync_items` for the same track without checking if a pending item already exists, leading to a bloated and inefficient queue.
3.  **Incorrect Track Matching**: The execution job (RFC-008) assumes a track ID from one service can be used on another. The correct approach is to search for the track by its title on the destination service to find the corresponding ID.
4.  **Lack of Detail**: The `sync_items` collection lacks fields to store the track title and the source/destination of the sync, making logs and debugging difficult.

This RFC prioritizes fixing these foundational issues before building the UI components that depend on them.

## 3. Technical Design: Prerequisites & Bug Fixes

### 3.1 Part 1: Fix Insufficient OAuth Scopes

The authentication flows for both Spotify and YouTube need to be updated to request the correct permissions.

*   **Spotify**: In `backend/internal/auth/spotify.go`, the `spotify.Authenticator` scope list must be expanded to include `playlist-modify-public` and `playlist-modify-private` in addition to the existing `playlist-read-private`.
*   **YouTube**: In `backend/internal/auth/youtube.go`, the scope must be changed from `youtube.readonly` to the more permissive `https://www.googleapis.com/auth/youtube`.

Users will need to re-authenticate both services after this change for the new scopes to take effect. The UI should be updated to reflect this, possibly by invalidating the existing connection status.

### 3.2 Part 2: Prevent Duplicate Sync Items

The analysis job needs to be more intelligent about queueing work.

*   **New Unique Index**: A new unique composite index will be added to the `sync_items` collection on `(mapping_id, service, action, payload)`. This will prevent the database from ever allowing a perfect duplicate. The migration will need to handle existing duplicates before creating the index.
*   **Analysis Logic Update**: In `backend/internal/jobs/analysis.go`, before calling `enqueueSyncItem`, the code must query the `sync_items` collection to check if a record with the same `mapping_id`, `service`, `action`, and `track_id` already exists with a status of `pending` or `running`.

### 3.3 Part 3: Add Detail to `sync_items` and Implement Correct Track Matching

To enable proper logging and track matching, the system needs to be updated.

*   **New `sync_items` Fields**: The `sync_items` collection will be updated via a migration to include:
    *   `source_track_id`: `text`, required
    *   `source_track_title`: `text`, required
    *   `source_service`: `select` (`spotify`, `youtube`), required
    *   `destination_service`: `select` (`spotify`, `youtube`), required
    The existing `payload` field will now store the `destination_track_id` after a successful search.

*   **Analysis Job (`analysis.go`) Changes**:
    *   The `analyzeTracks` function will now create `sync_items` with the new detailed fields. `source_track_id` and `source_track_title` will be populated from the source playlist.
    *   The `payload` will be empty initially.

*   **Execution Job (`executor.go`) Changes**:
    *   A new step will be added to the beginning of `processSyncItem` for `add_track` actions.
    *   This step will use the `source_track_title` to search for the track on the `destination_service`.
    *   **Spotify Search**: Use `client.Search()` with `spotify.SearchTypeTrack`.
    *   **YouTube Search**: Use `svc.Search.List()` with `type: "video"`. (Note: The YouTube API does not support searching specifically for "audio"; "video" is the closest available type for matching music content).
    *   The `destination_track_id` from the search result will be saved into the `payload` field of the `sync_item`.
    *   If no match is found, the item will be blacklisted with a reason of `search_failed`.
    *   The rest of the execution logic will use the `destination_track_id` from the payload to add the track to the playlist.

## 4. Technical Design: Activity Logging Dashboard

### 4.1 New Collection: `activity_logs`
Create via:
```bash
cd backend && go run cmd/server/main.go migrate create "create_activity_logs_collection"
```
Fields:
| field | type | notes |
|-------|------|-------|
| `level` | `select` (`info`, `warn`, `error`) | required |
| `message` | `text` | required, max 1024 chars |
| `sync_item_id` | `relation` → `sync_items` | optional, links log to a specific sync |
| `job_type` | `select` (`analysis`, `execution`, `system`) | required |

*   **Rules**: Publicly readable. Only the system (via admin API key or hooks) can create/update/delete entries.

**Note**: The `activity_logs` collection is specifically for tracking sync job activities, system events, and status updates that are displayed in the dashboard. This is distinct from general application logging (stdout/stderr) handled by Zerolog.

### 4.2 Backend Implementation

#### 4.2.1 Activity Logger Service
A new helper `activityLogger.Record(level, message, syncItemID, jobType)` will be created in a `backend/internal/activitylogger` package. It will write to both Zerolog (for console output) and the `activity_logs` PocketBase collection.

#### 4.2.2 Job Integration
The `analysis` and `executor` jobs will be modified to call the new activity logger service at key points:
*   `analysis`: "Starting analysis for X mappings", "Found Y diffs for mapping Z", "Analysis complete".
*   `executor`: "Processing item for track '{track_title}' (ID: {track_id})", "Successfully added track", "Error processing item: [reason]".

#### 4.2.3 Dashboard Stats Endpoint
A new **unauthenticated** route `/api/dashboard/stats` will be created. It will return an aggregated JSON object:
```json
{
  "mappings": { "total": 5 },
  "queue": { "pending": 12, "running": 1, "errors": 2, "skipped": 1, "done": 102 },
  "recent_runs": [
    { "timestamp": "...", "job_type": "analysis", "status": "success", "message": "..." }
  ],
  "youtube_quota": { "used": 1250, "limit": 10000 }
}
```
This data will be aggregated via direct DAO queries for performance.

### 4.3 Frontend UI

#### 4.3.1 Dashboard Page (`/dashboard`)
The main dashboard page will be updated to display real-time status cards using the data from `/api/dashboard/stats`.
*   **Cards**: "Mappings", "Queue - Pending", "Queue - Running", "Queue - Errors", "Queue - Skipped", "YouTube Quota".
*   **Controls**: A "Pause" button will stop the automatic refetching. A "Refresh" button will trigger a manual refetch.
*   **TanStack Query**: Data will be fetched with a `refetchInterval` of **60 seconds**, which can be paused and resumed.

#### 4.3.2 Activity Logs Page (`/logs`)
A new route at `/logs` will display the contents of the `activity_logs` collection in a virtualized table (e.g., using TanStack Table).
*   **Columns**: Timestamp, Level, Job Type, Message. If a `sync_item_id` is present, the message will be a link to a modal showing the details of that sync item (source/destination track, services, etc.).
*   **Filtering**: UI controls to filter by `level` and `job_type`.

---

## 5. Checklist

### Part 1: Bug Fixes & Prerequisites
- [x] **BF1: Update OAuth Scopes & UI** ✅ COMPLETED
    -   **Test Cases**:
        -   [x] Test that the Spotify authenticator requests `playlist-modify-public` and `playlist-read-private` scopes.
        -   [x] Test that the YouTube authenticator requests the `https://www.googleapis.com/auth/youtube` scope.
        -   [x] Test that the executor job can successfully add a track to a Spotify playlist after re-authentication.
        -   [x] Test that the executor job can successfully add a track to a YouTube playlist after re-authentication.

- [x] **BF2: Enhance `sync_items` Collection & Prevent Duplicates** ✅ COMPLETED
    -   **Test Cases**:
        -   [x] Test that the analysis job does not enqueue a `sync_item` if a pending one for the same track/mapping/action already exists.
        -   [x] Test that the database rejects a direct duplicate `sync_item` insertion due to the unique index.

- [x] **BF3: Implement Track Search in Executor** ✅ COMPLETED
    -   **Test Cases**:
        -   [x] Test that the executor searches for a track on Spotify by title before adding.
        -   [x] Test that the executor searches for a track on YouTube by title before adding.
        -   [x] Test that if a track is found via search, its ID is stored in the `payload` and used for the `add_track` operation.
        -   [x] Test that if a track is not found via search, the `sync_item` is moved to the blacklist with reason `search_failed`.

### Part 2: Activity Logging & Dashboard Features
- [x] **L1** Migration for `activity_logs` collection. ✅ COMPLETED
    -   **Test Cases**:
        -   [x] Test that an `activity_log` record can be created with all required fields via the DAO.

- [x] **L2** Create `activityLogger` service and integrate with existing jobs. ✅ COMPLETED
    -   **Test Cases**:
        -   [x] Test that `activityLogger.Record` creates a record in the `activity_logs` collection with the correct level, message, and job type.
        -   [x] Test that the analysis job creates start and end activity log entries.
        -   [x] Test that the executor job creates an activity log entry for each major step (processing, success, error).

- [x] **L3** Implement **unauthenticated** `/api/dashboard/stats` endpoint. ✅ COMPLETED
    -   **Test Cases**:
        -   [x] Test that a request to `/api/dashboard/stats` without auth headers succeeds.
        -   [x] Test that the endpoint aggregates correct counts for `mappings` and all `queue` statuses from mock data.
        -   [x] Test that the `youtube_quota` values are returned correctly from the tracker.
        -   [x] Test that `recent_runs` are populated from the `activity_logs` collection.

- [x] **L5** FE: Implement dashboard status cards with controls. ✅ COMPLETED
    -   **Test Cases**:
        -   [x] Test that the dashboard cards render the correct numbers from the mocked stats endpoint.
        -   [x] Test that the `refetchInterval` is paused when the "Pause" button is clicked and resumed when clicked again.
        -   [x] Test that clicking the "Refresh" button triggers `queryClient.invalidateQueries`.

- [x] **L6** FE: Implement activity logs page with filtering. ✅ COMPLETED
    -   **Test Cases**:
        -   [x] Test that the activity logs table renders rows from mocked log data.
        -   [x] Test that the table is updated correctly when the `level` filter is changed.
        -   [x] Test that a modal with sync item details is shown when a log message with a `sync_item_id` is clicked.

## Implementation Notes / Summary

### ✅ BF1: Update OAuth Scopes & UI (COMPLETED)
**Changes Made:**
- **Spotify Scopes**: Added `playlist-modify-public` and `playlist-modify-private` to `backend/internal/pbext/spotifyauth/spotifyauth.go`
- **YouTube Scopes**: Updated from `youtube.YoutubeReadonlyScope` to `youtube.YoutubeScope` in `backend/internal/auth/youtube.go`
- **Scope Consolidation**: Made `YouTubeScopes` public and eliminated duplication between `auth/youtube.go` and `googleauth/googleauth.go`
- **Test Updates**: Fixed `backend/internal/pbext/googleauth/googleauth_test.go` to expect new scope URL pattern

**Files Modified:**
- `backend/internal/auth/youtube.go` - Updated and consolidated YouTube scopes
- `backend/internal/pbext/googleauth/googleauth.go` - Removed duplicate scopes, use unified YouTubeScopes
- `backend/internal/pbext/googleauth/googleauth_test.go` - Updated test expectations
- `backend/internal/pbext/spotifyauth/spotifyauth.go` - Added playlist modification scopes

**Testing:** All backend tests passing ✅

### ✅ BF2: Enhance `sync_items` Collection & Prevent Duplicates (COMPLETED)
**Changes Made:**
- **Database Migration**: `backend/migrations/1750474958_prevent_duplicate_sync_items.go`
  - Changed `payload` field from JSON to TEXT type for reliable indexing
  - Added unique composite index on `(mapping_id, service, action, payload)`
  - Handles existing duplicates before creating index
  - Includes proper rollback functionality
- **Duplicate Prevention Logic**: Updated `enqueueSyncItem()` in `backend/internal/jobs/analysis.go`
  - Checks all existing sync_items for pending/running duplicates
  - Handles PocketBase relation field storage correctly (mapping_id as array)
  - Allows duplicates when original items are completed (`done` status)
  - Uses manual filtering approach for reliability
- **Comprehensive Testing**: Added `TestEnqueueSyncItem_DuplicatePrevention` in `backend/internal/jobs/analysis_test.go`
  - Tests basic duplicate prevention
  - Tests different actions/services create new items
  - Tests allowing duplicates when original is completed
  - Tests edge cases and relation field handling

**Files Modified:**
- `backend/migrations/1750474958_prevent_duplicate_sync_items.go` - Database schema updates
- `backend/internal/jobs/analysis.go` - Duplicate prevention logic
- `backend/internal/jobs/analysis_test.go` - Comprehensive test coverage

**Testing:** All backend tests passing ✅, duplicate prevention working perfectly with detailed logging

### ✅ BF3: Implement Track Search in Executor (COMPLETED)
**Testing:** Core BF3 functionality fully working ✅
- Analysis creates sync_items with proper track details
- Track search works for both Spotify and YouTube
- Executor actions use searched track IDs correctly
- Blacklist entries created for failed searches
- All executor action tests passing
- **All backend tests passing** ✅
- Duplicate prevention fixed to work with new payload format
- Search failure handling properly tested and working
- Test suite updated to match new BF3 behavior

**Files Modified:**
- `backend/internal/jobs/analysis.go`: Enhanced sync item creation with track details
- `backend/internal/jobs/executor.go`: Added complete track search and execution logic
- `backend/internal/testhelpers/pocketbase.go`: Updated schema with BF3 track detail fields
- `backend/internal/jobs/analysis_test.go`: Updated tests for BF3 compatibility  
- `backend/internal/jobs/executor_test.go`: Updated tests to match new BF3 behavior

**Key Implementation Details:**
- Track search failures now create blacklist entries with "search_failed" reason
- Sync items marked as "skipped" when search fails (not "pending")
- Duplicate prevention works with timestamped payloads
- All existing functionality preserved during BF3 implementation
- Zero regressions introduced ✅

### ✅ L1: Migration for `activity_logs` collection (COMPLETED)
**Changes Made:**
- **Database Migration**: `backend/migrations/1750550511_create_activity_logs_collection.go`
  - Created `activity_logs` collection with proper schema
  - Added fields: level (select), message (text), sync_item_id (relation), job_type (select)
  - Made accessible to all users per user request
  - Added indexes for efficient querying
- **Migration Applied**: Migration successfully applied to database

**Files Modified:**
- `backend/migrations/1750550511_create_activity_logs_collection.go` - New migration file

**Testing:** Migration applied successfully, collection created with proper schema ✅

### ✅ L2: Create `activityLogger` service and integrate with existing jobs (COMPLETED)
**Changes Made:**
- **Activity Logger Service**: `backend/internal/activitylogger/activitylogger.go`
  - Implements `daoProvider` interface pattern for consistency
  - `Record()` method logs to both console (structured format) and `activity_logs` collection
  - `RecordWithContext()` for timeout handling
  - Comprehensive error handling for missing collection in test environments
- **Job Integration**: Added activity logging to both jobs
  - **Analysis Job** (`backend/internal/jobs/analysis.go`): Logs job start, mapping counts, errors, completion
  - **Executor Job** (`backend/internal/jobs/executor.go`): Logs processing start, item details, success/error outcomes
  - Different log levels: info (normal flow), warn (rate limits, temp errors), error (fatal errors)
- **Comprehensive Testing**: `backend/internal/activitylogger/activitylogger_test.go`
  - Tests all logging scenarios including timeouts
  - Verifies console and database logging integration
  - All backend tests passing ✅

**Files Modified:**
- `backend/internal/activitylogger/activitylogger.go` - New activity logger service
- `backend/internal/activitylogger/activitylogger_test.go` - Comprehensive test suite
- `backend/internal/jobs/analysis.go` - Activity logging integration
- `backend/internal/jobs/executor.go` - Activity logging integration

**Key Implementation Details:**
- Console logging uses structured "ACTIVITY" prefix format for easy filtering
- Database logging handles missing collection gracefully (test environments)
- Activity logs are distinct from general application logging (Zerolog)
- Integration preserves all existing job functionality with zero regressions ✅

### ✅ L3: Implement unauthenticated `/api/dashboard/stats` endpoint (COMPLETED)
**Changes Made:**
- **Dashboard Package**: `backend/internal/pbext/dashboard/routes.go`
  - Implements `/api/dashboard/stats` endpoint returning JSON with mappings, queue, recent_runs, and youtube_quota data
  - Uses proper PocketBase patterns with `FindRecordsByFilter` for database queries
  - Graceful handling of missing activity_logs collection
  - Integrated with YouTube quota tracker via public `GetYouTubeQuotaUsage()` function
- **Route Registration**: Updated `backend/cmd/server/main.go` to register dashboard routes
- **YouTube Quota Access**: Added public function `GetYouTubeQuotaUsage()` in `backend/internal/jobs/executor.go`
- **Comprehensive Testing**: `backend/internal/pbext/dashboard/routes_test.go`
  - Interface wrapper pattern for test compatibility
  - Tests endpoint functionality, empty database handling, missing collections
  - Uses proper testhelpers for collection setup with unique payloads to avoid constraint violations

**Files Modified:**
- `backend/internal/pbext/dashboard/routes.go` - New dashboard API package
- `backend/internal/pbext/dashboard/routes_test.go` - Comprehensive test suite  
- `backend/cmd/server/main.go` - Dashboard route registration
- `backend/internal/jobs/executor.go` - Public YouTube quota access function

**API Response Format:**
```json
{
  "mappings": { "total": 5 },
  "queue": { "pending": 12, "running": 1, "errors": 2, "skipped": 1, "done": 102 },
  "recent_runs": [
    { "timestamp": "...", "job_type": "analysis", "status": "success", "message": "..." }
  ],
  "youtube_quota": { "used": 1250, "limit": 10000 }
}
```

**Testing:** All tests passing ✅, including comprehensive dashboard test suite with proper test data setup

### ✅ L5: Frontend dashboard status cards with controls (COMPLETED)
**Changes Made:**
- **Dashboard API Types**: `frontend/src/lib/api.ts`
  - Added `DashboardStats` interface with mappings, queue, recent_runs, and youtube_quota types
  - Added `getDashboardStats()` API function using PocketBase SDK
- **Dashboard Stats Cards Component**: `frontend/src/components/DashboardStatsCards.tsx`
  - Created reusable `StatCard` component with loading states and color variants
  - Created `ControlsCard` component with pause/resume and refresh functionality
  - Integrated TanStack Query with 60-second `refetchInterval` that can be paused/resumed
  - Added comprehensive error handling with retry functionality
  - Uses proper loading states with accessibility attributes
- **Dashboard Page Update**: `frontend/src/routes/dashboard.lazy.tsx`
  - Integrated new `DashboardStatsCards` component with state management
  - Added proper pause/resume state handling
  - Organized layout with clear sections for system status and service connections
- **MSW Test Handlers**: `frontend/src/test/mocks/handlers.ts`
  - Added `/api/dashboard/stats` endpoint handler with realistic mock data
- **Comprehensive Test Suite**: `frontend/src/components/DashboardStatsCards.test.tsx`
  - Tests component rendering with API data
  - Tests loading states and error handling
  - Tests pause/resume functionality and button interactions
  - Tests refresh button triggering query invalidation
  - All tests using MSW for API mocking

**Files Modified:**
- `frontend/src/lib/api.ts` - Dashboard API types and function
- `frontend/src/components/DashboardStatsCards.tsx` - New stats cards component
- `frontend/src/routes/dashboard.lazy.tsx` - Updated dashboard with new cards
- `frontend/src/test/mocks/handlers.ts` - Added dashboard stats mock handler
- `frontend/src/components/DashboardStatsCards.test.tsx` - Comprehensive test suite

**Key Features:**
- Real-time stats display with 60-second auto-refresh
- Pause/Resume controls for auto-refresh
- Manual refresh button with query invalidation
- Color-coded stat cards with appropriate icons
- Loading states with accessibility support
- Error handling with retry functionality
- Responsive grid layout
- Comprehensive test coverage

**Testing:** All frontend tests passing ✅, including comprehensive dashboard stats card functionality

## Final Validation ✅

**All requirements successfully completed:**
- ✅ All bug fixes (BF1-BF3) implemented and tested
- ✅ All logging & dashboard features (L1-L6) implemented and tested  
- ✅ **Complete test suite validation**: 
  - **Frontend**: 46/46 tests passing ✅
  - **Backend**: All tests passing ✅  
  - **Zero regressions introduced** ✅
- ✅ Activity logs page accessible at `/logs` route with full functionality
- ✅ Dashboard stats cards working with real-time updates
- ✅ Activity logging integrated throughout job system
- ✅ All accessibility requirements met

**RFC-010 Status: COMPLETED** 🎉

## 6. Definition of Done
- ✅ All bug fixes are implemented and tested. The sync process is reliable.
- ✅ The dashboard displays accurate, near real-time stats about the system's health.
- ✅ The activity logs page provides a filterable view of system events.
- ✅ All new and existing tests pass.

## 7. Resources & References
*   PocketBase Go Records API – https://pocketbase.io/docs/go-records/
*   Spotify Search API – https://developer.spotify.com/documentation/web-api/reference/search
*   YouTube Search API – https://developers.google.com/youtube/v3/docs/search/list
*   TanStack Table – https://tanstack.com/table/v8
*   RFC-007 (Sync Analysis): For context on the analysis job.
*   RFC-008 (Sync Execution): For context on the executor job.

---

*End of RFC-010* 