# RFC-012: Playlist backups (point-in-time snapshots and restore)

- **Status:** Accepted, implemented on `fix/revive-sync-engine`
- **Date:** 2026-10-23
- **Scope:** new `backups` package, snapshot scheduler, Backups page, restore actions

## 1. Problem

Spotube writes to real playlists. If a playlist is damaged, there is no way to see what it
held last week or to get it back. Damage can come from a bad sync, a wrong match, a mistaken
bulk delete in the Spotify or YouTube app, or a shared playlist edited by someone else. We
want:

1. **Point-in-time copies** of chosen playlists: the track list, in order, with each track's
   ID on its own service. Where Spotube knows it, also store the ID on the other service.
2. **Opt-in per playlist, on either service.** A playlist does not need to be in a mapping.
3. **Restore**:
   - a new playlist rebuilt from a snapshot;
   - the missing tracks put back into the original playlist;
   - a copy on the other service, using known track links;
   - an export file.
4. **Cheap and safe within API limits.**
   - Spotify reads are free (rate-limited only).
   - A YouTube playlist read costs 2 units per 50 tracks.
   - YouTube writes cost 50 units per track.

## 2. When to take a snapshot

| Option | Pros | Cons |
|---|---|---|
| A. Only during sync analysis | Free: analysis already fetched the playlist | Only covers mapped playlists; no snapshot if sync is off or failing |
| B. Only on mapping creation | Simple | One snapshot, never updated |
| C. Own schedule per playlist | Works for any playlist; predictable | Extra API reads (small) |
| D. Before every write | Captures the exact pre-change state | Many near-identical copies; extra reads in the hot path |
| **E. C + A, deduplicated (chosen)** | Every chosen playlist is covered on its own schedule; mapped playlists also get a free snapshot on every sync; identical states are stored once | Two entry points to keep consistent |

**Chosen: E.**
- Each backup has an interval (default 24 hours). A background ticker, running every
  10 minutes even when sync workers are off, snapshots the backups that are due.
- When sync analysis fetches a playlist that has a backup, it records the tracks it already
  has, at no API cost. Analysis runs before the executor writes anything, so the latest
  snapshot is always the state before Spotube's latest changes. This covers most of option D.
- "Back up now" and adding a backup take a snapshot straight away.

**Deduplication.**
- A snapshot is stored only when the playlist changed (a SHA-256 of the name plus the
  ordered track IDs).
- When nothing changed, the latest snapshot's `confirmed_at` moves forward instead. Each
  snapshot therefore means "the playlist looked like this from `created` until at least
  `confirmed_at`".
- So one month of daily checks on a stable playlist is one row, not 30.

**Retention.**
- Keep the newest *N* distinct snapshots per backup (default 30, editable).
- Because only changes are stored, 30 snapshots usually span months.
- Deleting a backup deletes its snapshots.

## 3. What a snapshot stores

```
playlist_backups    one row per opted-in playlist
  id, service, playlist_id, playlist_name, enabled, interval_hours, keep_snapshots,
  last_checked_at, last_error, created, updated
  UNIQUE(service, playlist_id)

playlist_snapshots  one row per distinct state
  id, backup_id → playlist_backups (cascade), playlist_name, track_count, content_hash,
  source ('scheduled' | 'manual' | 'sync' | 'initial'), created, confirmed_at,
  tracks_json
```

`tracks_json` is an ordered array:

```json
{ "position": 0, "id": "<native id>", "title": "…", "artist": "…", "raw_title": "…",
  "channel": "…", "duration_ms": 215000, "spotify_id": "…", "youtube_id": "…" }
```

- `spotify_id` and `youtube_id` are copied from `track_links` when the snapshot is taken.
  That means a snapshot can rebuild the playlist on the other service even if the links
  change later.
- The tracks are stored as JSON, not in a child table. Snapshots are always read and written
  whole, never queried per track. A 500-track playlist is about 100 KB.

## 4. Restore

`POST /api/backups/snapshots/:id/restore` with a `mode`:

| Mode | What it does | YouTube cost |
|---|---|---|
| `new` | Creates a playlist (**private** on YouTube; Spotify ignores the flag and lists it as public) named "<name> (restored YYYY-MM-DD)" and adds the tracks in order | 50 + 50 per track |
| `missing` | Adds back tracks that are in the snapshot but no longer in the original playlist (appended). Never removes anything | 2 per 50 to read + 50 per track |
| `new` on the other service | As `new`, using the counterpart IDs stored in the snapshot. Tracks without a known counterpart are listed, not searched | as above |

**Guards and results:**
- YouTube restores are refused (HTTP 429, with the estimate) when the estimated cost exceeds
  the quota left today.
- Spotify adds go in batches of 100.
- The response lists what was added, what was skipped and what had no counterpart.
- Each restore writes an activity log entry.

**Export** (`GET /api/backups/snapshots/:id/export?format=json|csv`) downloads the snapshot,
so the data survives even if the database is lost.

**Not doing (yet):**
- An exact rollback that removes tracks added since the snapshot. Spotube is additive by
  design, and a removal mistake is worse than a duplicate. `missing` mode plus a manual
  cleanup covers the corruption case.
- Searching for tracks with no counterpart during a cross-service restore. That can cost
  100 units per track; the review queue is the right tool for it.

## 5. API

| Method & path | Purpose |
|---|---|
| `GET /api/backups` | Backups with snapshot count and latest snapshot |
| `POST /api/backups` | Add `{service, playlist_id, playlist_name?, interval_hours?, keep_snapshots?}`; takes the first snapshot |
| `PATCH /api/backups/:id` | `enabled`, `interval_hours`, `keep_snapshots` |
| `DELETE /api/backups/:id` | Stop backing up and delete its snapshots |
| `POST /api/backups/:id/run` | Back up now |
| `GET /api/backups/:id/snapshots` | Timeline, newest first, with +added / −removed against the previous snapshot |
| `GET /api/backups/snapshots/:id` | One snapshot with tracks and its diff |
| `GET /api/backups/snapshots/:id/export` | JSON or CSV download |
| `POST /api/backups/snapshots/:id/restore` | `{mode: new\|missing, service?}` |

## 6. UI

**Backups** in the nav (`/backups`):
- Backed-up playlists from both services: last change, last check, snapshot count, interval,
  on/off, **Back up now**.
- **Add playlist** opens a picker with tabs for Spotify and YouTube. Playlists already backed
  up are marked.

**Backup detail** (`/backups/:id`):
- A snapshot timeline with change counts.
- The selected snapshot's track list. Tracks added and removed since the previous snapshot are
  highlighted, and each track shows its IDs on both services.
- **Restore** (choose a mode; shows the estimated quota cost) and **Export**.

## 7. Testing

**Unit tests:**
- dedupe (`confirmed_at` moves forward, no new row);
- retention pruning;
- diff;
- counterpart IDs taken from links;
- the analysis hook only records playlists that have a backup;
- restore in all modes against fake writers, including the quota refusal.

**Handler tests** cover the API.

**Live:** back up one Spotify and one YouTube playlist, check that the first snapshot and a
no-change re-run are deduplicated, export, and restore the Spotify backup into a new
playlist. Spotify writes are free.
