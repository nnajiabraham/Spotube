# RFC-010: Logging & Status Dashboard

**Status:** Draft  
**Branch:** `rfc/010-logging-dashboard`  
**Depends On:**
* RFC-007 (including 7b, 7c) & RFC-008(including 8b) (Jobs producing data to visualize)
* RFC-009 (including 9b) (Blacklist producing data to visualize)

---

## 1. Goal

Provide the user with a real-time status dashboard that visualizes job runs, per-mapping status, sync queue health, and a tail-able log of important events. This makes the system's state observable and debuggable.

## 2. Background & Context

With analysis and execution jobs running in the background, the user needs visibility into what the application is doing. A dashboard with high-level stats and a detailed log viewer are standard for self-hosted services. This RFC focuses on collecting and presenting this data.

## 3. Technical Design

### 3.1 New Collection: `logs`
Create via:
```bash
cd backend && go run cmd/server migrate create "create_logs_collection"
```
Fields:
| field | type | notes |
|-------|------|-------|
| `level` | `select` (`info`, `warn`, `error`) | required |
| `message` | `text` | required, max 1024 chars |
| `mapping_id`| `relation` → `mappings` | optional, links log to a specific sync |
| `job_type` | `select` (`analysis`, `execution`, `system`) | required |

*   **TTL**: This collection should be configured to automatically delete records older than a configurable number of days (e.g., 7 days) to prevent the database from growing indefinitely. PocketBase doesn't have native TTL, so this will be a simple scheduled job that runs daily.
*   **Rules**: Read-only for authenticated users. Only the system (via hooks) can create entries.

### 3.2 Backend Implementation

#### 3.2.1 Logging Service
A new helper `logger.Log(level, message, mappingID, jobType)` will be created in a `backend/internal/logger` package. It will write to both Zerolog (for console output) and the `logs` PocketBase collection.

#### 3.2.2 Job Integration
The `analysis` and `executor` jobs will be modified to call the new logging service at key points:
*   `analysis`: "Starting analysis for X mappings", "Found Y diffs for mapping Z", "Analysis complete".
*   `executor`: "Processing item X", "Successfully added track Y", "Error processing item X: [reason]".

#### 3.2.3 Dashboard Stats Endpoint
A new route `/api/admin/dashboard/stats` will be created. It will return an aggregated JSON object:
```json
{
  "mappings": { "total": 5 },
  "queue": { "pending": 12, "errors": 2, "skipped": 1 },
  "recent_runs": [
    { "timestamp": "...", "job_type": "analysis", "status": "success" }
  ]
}
```
This data will be aggregated via direct DAO queries for performance.

### 3.3 Frontend UI

#### 3.3.1 Dashboard Page (`/`)
The main dashboard page will be updated to display real-time status cards using the data from `/api/admin/dashboard/stats`.
*   **Cards**: "Mappings Configured", "Queue - Pending", "Queue - Errors", "Queue - Skipped".
*   Each card will be a link to the relevant management page (e.g., `/mappings`, `/logs?level=error`).
*   **TanStack Query**: Data will be fetched with a `refetchInterval` of 60 seconds to provide a near real-time view.

#### 3.3.2 Logs Page (`/logs`)
A new route at `/logs` will display the contents of the `logs` collection in a virtualized table (e.g., using TanStack Table).
*   **Columns**: Timestamp, Level (with colored badge), Job Type, Mapping, Message.
*   **Filtering**: UI controls to filter by `level` and `job_type`.

### 3.4 Testing Strategy

#### Backend
*   **Unit Tests**: Verify that the `logger` service correctly writes to both Zerolog and the `logs` collection.
*   **Integration Tests**:
    *   Run the `AnalyseMappings` job and assert that the expected log entries are created.
    *   Test the `/api/admin/dashboard/stats` endpoint with a known set of data and verify the aggregated counts are correct.

#### Frontend
*   **MSW**: Mock handlers for `/api/admin/dashboard/stats` and `/api/collections/logs/records`.
*   **Vitest**: Test that the dashboard cards render the correct numbers from the mocked stats.
*   **Playwright**: Full E2E test to navigate to the dashboard, view stats, navigate to the logs page, and filter the log entries.

## 4. Dependencies
*   **Frontend**: `@tanstack/react-table` for the virtualized log viewer.

## 5. Checklist
- [ ] **L1** Migration for `logs` collection.
- [ ] **L2** Create `logger` service and integrate with existing jobs.
- [ ] **L3** Implement `/api/admin/dashboard/stats` endpoint.
- [ ] **L4** Implement daily TTL job to clean up old logs.
- [ ] **L5** FE: Implement dashboard status cards.
- [ ] **L6** FE: Implement logs page with filtering.
- [ ] **L7** Backend tests for logger and stats endpoint.
- [ ] **L8** Frontend tests for dashboard and logs page with MSW.

## 6. Definition of Done
*   The dashboard displays accurate, near real-time stats about the system's health.
*   The logs page provides a filterable view of system events.
*   Log records are automatically purged after the configured TTL.
*   All tests pass.

## Resources & References
*   PocketBase Go Records API – https://pocketbase.io/docs/go-records/
*   TanStack Table – https://tanstack.com/table/v8
*   MSW – https://mswjs.io/
*   Playwright – https://playwright.dev/

---

*End of RFC-010* 