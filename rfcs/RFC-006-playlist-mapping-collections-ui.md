# RFC-006: Playlist Mapping Collections & UI

**Status:** Draft  
**Branch:** `rfc/006-playlist-mapping`  
**Depends On:**
* RFC-004 (Spotify OAuth – playlists endpoint)
* RFC-005 (YouTube OAuth – playlists endpoint)

---

## 1. Goal

Implement the data model, backend endpoints, and frontend screens that let the user **map** a Spotify playlist to a YouTube playlist (bi-directional), specify sync options, and save the configuration for later scheduled jobs.

## 2. Background & Context

The PRD specifies "Playlist Mapping Management" as a core MVP feature.  A _mapping_ is a join entity:
```
Spotify Playlist ↔ YouTube Playlist
```
with two boolean options:
* `sync_name` – mirror title changes?
* `sync_tracks` – mirror track list?

Later RFC-007/008 will read these mappings to queue sync jobs.  This RFC focuses on CRUD UI + REST.

## 3. Technical Design

### 3.1 New Collection: `mappings`
*Create migration via*:
```bash
cd backend && go run cmd/server migrate create "create_migrations_collection"
```
This generates `pb_migrations/<timestamp>_create_mappings_collection.go`.  Edit the Up/Down functions as follows:

| field | type | notes |
|-------|------|-------|
| `spotify_playlist_id` | `text` | required |
| `youtube_playlist_id` | `text` | required |
| `spotify_playlist_name` | `text` | cached for list UI |
| `youtube_playlist_name` | `text` | cached |
| `sync_name` | `bool` | default true |
| `sync_tracks` | `bool` | default true |
| `interval_minutes` | `number` | default 60, min 5 |

Collection rules:
```
list/view/create/update/delete -> @request.auth.id != ""   # only authenticated owner (single-user)
```
Later when multi-user introduced, we'll add owner field.

### 3.2 Backend Helpers
* **No custom routes needed for CRUD** – PocketBase collection API suffices.
* **Hook** on `BeforeCreate` & `BeforeUpdate` to validate:
  * playlist IDs exist in the token owner's accounts (call Spotify / YouTube list endpoints).  To avoid latency, we'll _warn_ in UI but hooks only ensure non-empty.
  * `interval_minutes ≥ 5`.
* **AfterCreate/Update**: populate cached playlist names via Spotify / YouTube HTTP calls in background goroutine to prevent UX delay.
* Index on `(spotify_playlist_id, youtube_playlist_id)` unique to prevent duplicates.

### 3.3 Frontend UI

#### 3.3.1 Routes
```
src/routes/_authenticated/mappings/
 ├── index.lazy.tsx      # list view
 ├── new.lazy.tsx        # creation wizard
 └── $mappingId/edit.lazy.tsx
```

#### 3.3.2 Data Fetching
* Use **TanStack Query**; cache key `['mappings']` hitting `/api/collections/mappings/records`.
* For playlist selectors call `/api/spotify/playlists` & `/api/youtube/playlists` (already proxied).

#### 3.3.3 Creation Wizard UX
1. **Step 1** – choose Spotify playlist (select list, searchable).
2. **Step 2** – choose YouTube playlist **or** "Create new on YouTube" toggle.
   * If creating new, backend will create via YouTube API (deferred to future RFC) – for now wizard disables that option (greyed "coming soon").
3. **Step 3** – options form: `Sync name?` `Sync tracks?` `Interval` (slider 5-720 min).
4. **Review & Save** – POST record → on success, navigate to list.

Components: Shadcn/ui `Select`, `Switch`, `Input`, `Button`, `Alert`, and `DataTable` for list.

### 3.4 Validation
* Zod schema in FE matches PB schema.
* Error toast on duplicate mapping (409 from PB unique index).

### 3.5 Testing Strategy

#### Backend
* Go tests using PocketBase test harness:
  * Create mapping record – expect 201.
  * Duplicate mapping – expect 409.
  * Interval <5 – expect 400.
* Use `httpmock` to stub Spotify/YouTube name-fetch in AfterCreate.

#### Frontend
* **Vitest** + **MSW**:
  * Mock `/api/spotify/playlists` & `/api/youtube/playlists` with sample payloads.
  * Unit test wizard steps input validation.
* **Playwright E2E** with MSW:
  * Full happy-path: create mapping; list shows row.
  * Duplicate creation shows error.

### 3.6 Dependencies
* None new (uses existing stacks: TanStack Query, Zod, MSW, Playwright, httpmock).

### 3.7 Checklist
- [ ] **M1** Migration for `mappings` collection with fields & unique index.
- [ ] **M2** Add hooks for validation & name-caching.
- [ ] **M3** FE list & wizard pages.
- [ ] **M4** Backend tests (duplicate/validation).
- [ ] **M5** FE Vitest + Playwright tests with MSW.
- [ ] **M6** README update (mapping feature docs).

## 4. Definition of Done
* Authenticated user can create, edit, delete mappings in UI.
* Duplicate pairs rejected.
* List shows cached playlist names.
* All tests pass.

## 5. Implementation Notes / Summary
* Cached names prevent extra API hits in list page. They will be refreshed by sync job (RFC-007) anyway.
* Creating brand-new playlist on opposite service is deferred – UI hides feature until RFC adds backend support.
* Drag-and-drop mapping was considered but two selects provide clearer UX for first iteration.

## Resources & References
* PocketBase collection rules – https://pocketbase.io/docs/collections/#rules-filters
* TanStack Query – https://tanstack.com/query/latest
* Zod – https://github.com/colinhacks/zod
* MSW – https://mswjs.io/
* Playwright – https://playwright.dev/
* httpmock – https://github.com/jarcoal/httpmock

---

*End of RFC-006* 