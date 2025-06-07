# RFC-009: Conflict & Blacklist Handling

**Status:** Draft  
**Branch:** `rfc/009-blacklist-conflict`  
**Depends On:**
* RFC-007 (analysis queue) & RFC-008 (execution worker)

---

## 1. Goal

Provide mechanisms to exclude problematic tracks from sync (user-managed blacklist) and surface conflicts (e.g., track not found on target service) with UI tools to resolve them.  The execution job will consult this blacklist and mark items accordingly; the dashboard will expose management pages.

## 2. Background & Context

During cross-service sync we'll hit unmatchable content: regional restrictions, private uploads, mismatched ISRCs, etc.  We need to:
1. Record failures with **reason** and **counter**.
2. Allow the user to permanently skip ('blacklist') that track for this mapping (or globally).
3. Allow un-blacklisting to retry later.

## 3. Technical Design

### 3.1 New Collection: `blacklist`
Create via:
```bash
cd backend && go run cmd/server migrate create "create_blacklist_collection"
```
Fields:
| field | type | notes |
|-------|------|-------|
| `mapping_id` | `relation` → `mappings` | nullable (nil = global) |
| `service` | `select` (`spotify`, `youtube`) | required |
| `track_id` | `text` | required |
| `reason` | `text` | required |
| `skip_counter` | `number` | default 1 |
| `last_skipped_at` | `date` | auto update |

Unique composite on `(mapping_id, service, track_id)`.
Rules: authenticated only.  No public access.

### 3.2 Updates to `sync_items` Handling
* When executor encounters non-fatal 404/400 indicating "track not found", it will:
  * Create or update blacklist record (`skip_counter++`).
  * Mark item `skipped`.
* Analysis job must consult blacklist when generating `add_track` items, ensuring we do not enqueue blacklisted tracks again (unless blacklist entry deleted).

### 3.3 Frontend UI

#### 3.3.1 Dashboard Indicators
* Sync queue widget increments **Skipped** badge.
* Clicking opens **Conflict Resolver Modal** showing the first 20 skipped items grouped by mapping.

#### 3.3.2 Blacklist Management Page
Route: `/mappings/$mappingId/blacklist`
* Table columns: Service badge, Track title (fetched lazily), Reason, Skip count, Last skipped.
* Actions: **Un-blacklist** button – deletes record which allows future analysis to retry.

#### 3.3.3 Components
* Shadcn/ui `Badge`, `Table`, `Dialog`, `Button`, `Tooltip`.
* Use TanStack Query with key `['blacklist', mappingId]`.

### 3.4 Executor Changes
* Map API error codes:
  * Spotify 404 track → `reason="not_found"`.
  * YouTube 403 forbidden → `reason="forbidden"`.
* Configurable retry attempts (default 3). After exceeding attempts, auto-blacklist.

### 3.5 Testing Strategy

#### Backend
* `httpmock` returns 404 for add_track → expect blacklist record created.
* Analysis job skips blacklisted track.
* Executor un-blacklisting (delete record) then runs → item processed successfully.

#### Frontend
* **MSW** mocks blacklist endpoints.
* Vitest component tests for resolver modal.
* Playwright flow: open blacklist, un-blacklist, verify queue item processed (simulate via MSW state).

### 3.6 Checklist
- [ ] **B1** Migration for `blacklist` collection + unique index.
- [ ] **B2** Modify analysis loop to filter blacklisted tracks.
- [ ] **B3** Modify executor to create/update blacklist on unrecoverable errors.
- [ ] **B4** FE blacklist pages & modal.
- [ ] **B5** Backend tests (blacklist creation, filter logic).
- [ ] **B6** FE tests with MSW & Playwright.
- [ ] **B7** README section describing blacklist.

## 4. Definition of Done
* Blacklisted tracks not retried.
* UI lists and can un-blacklist; sync picks them up next analysis.
* Tests pass.

## Resources & References
* PocketBase collections – https://pocketbase.io/docs/go-collections/
* Spotify error codes – https://developer.spotify.com/documentation/web-api
* YouTube error codes – https://developers.google.com/youtube/v3/docs/errors
* MSW – https://mswjs.io/
* Playwright – https://playwright.dev/

---

*End of RFC-009* 