# Spotube app guide

What Spotube does, how each workflow runs, and what every page shows. This is the reference for
how the app is meant to behave. If the code disagrees, one of them is wrong. Configuration and
commands are in the [README](../README.md); colours and components are in
[design-system.md](./design-system.md).

## 1. Concepts

| Term | Meaning |
|---|---|
| **Mapping** | A Spotify playlist paired with a YouTube playlist. Syncing is per mapping. |
| **Sync run** | One pass over a mapping: read both playlists, queue what's missing, apply the queue. |
| **Sync item** | One queued change: add a track to one side, or rename the YouTube playlist. |
| **Match score** | How alike two tracks are, 0–100%. **85%** or more counts as the same recording. |
| **Track link** | Spotube's memory that a Spotify track and a YouTube video are the same song, or that a song is **not on** a service. Links are global: they apply to every mapping. |
| **Review** | Where a track waits when Spotube searched but wasn't confident. Only you can settle it. |
| **Blocked track** | A track you never want synced for one mapping. |
| **Backup** | A playlist on either service whose track list Spotube saves over time. Independent of mappings: any playlist can be backed up. |
| **Snapshot** | The playlist's name and tracks, in order, at one point in time, with each track's Spotify and YouTube IDs where Spotube knows both. A new snapshot is saved only when something changed. |

## 2. Workflows

### 2.1 First run (setup)

1. Open `http://127.0.0.1:5173`. With no OAuth credentials, every page redirects to **Setup**.
2. Enter the Spotify and Google client ID and secret (or put them in `backend/.env`).
3. On the **Dashboard**, connect Spotify, then YouTube. Each opens the provider's consent page
   and returns to the Dashboard with a "connected" message.

### 2.2 Pair two playlists

**Mappings → Pair playlists**: pick one playlist from each account, choose whether to sync
names and tracks, and set the interval (5 minutes to once a day). The mapping syncs on its
first run.

- A playlist can be in only one mapping; already-paired playlists are greyed out (the API
  returns 409 for them).
- **Only on one side?** Choose **New {service} playlist** at the top of the other list instead.
  Spotube creates it (named after the source playlist unless you type a name), pairs it, and
  the first sync copies the tracks across. Creating a YouTube playlist costs 50 quota units and
  is private; Spotify ignores the private flag for apps, so a new Spotify playlist is public.
  Videos with no confident match (talks, mixes) go to Review as usual.

### 2.3 A sync run

A run starts on the cron schedule, when a mapping's interval has elapsed and
`SYNC_WORKERS_ENABLED=true`, or straight away with **Sync now**.

```
1. Read both playlists                          Spotify: free · YouTube: 1 unit per 50 videos (+1 for durations)
2. Rename the YouTube playlist if needed        50 units, only when names differ and sync_name is on
3. For each track on each side:
     a. linked?  counterpart present → nothing to do
                 marked "not on <service>" → nothing to do, never searched
     b. otherwise fuzzy-match against the other playlist (score ≥ 85% → present, and remembered as a link)
     c. otherwise queue an `add` sync item
4. Apply the queue (auto-apply, up to 25 items per mapping)
5. Stamp the mapping with last sync time and status
```

**Applying one add item:**

```
linked to a known counterpart?   → add it directly (no search)
marked not on the destination?   → skip ("not on YouTube")
already in the destination?      → skip, and remember the pair
search the destination           Spotify: free · YouTube: 101 units
best result ≥ 85%                → add it (YouTube: 50 units) and remember the pair
nothing confident                → status "Needs review" (never retried automatically)
API error                        → status "Failed", retried up to 3 times
```

Because pairs are remembered by ID, a track Spotube copied is never copied back, and a song is
added to a playlist at most once, even if its title differs on the two services.

### 2.4 Review a track

When a search finds nothing confident, the item goes to **Review** (nav badge, dashboard tile).
Each card shows the source track beside the closest candidate and its score. You can:

| Action | What happens | Cost |
|---|---|---|
| **Use this** | Remembers the pair and adds the track now | YouTube add: 50 units |
| **Find on {service} → Search** | In-app search; results scored against the source, best first | YouTube: 101 per search; repeat within 30 min is free |
| **Find on {service} → Paste a link** | Resolves a URL, URI or ID, then **Use this** | YouTube: 1 unit |
| **Not on {service}** | Remembers that the song isn't there; skips it in every mapping; never searched again | none |
| **⋯ → Search again automatically** | Back to pending and run now | YouTube: 101 units |
| **⋯ → Don't sync this track** | Blocks the track for this mapping only | none |

A decision settles the same song in every mapping. Picking a match moves that song's other
review items back to pending; they are then added from the link without a search.

### 2.5 Undo a decision

- **Matches → Forget** removes a link. On the next sync, Spotube decides that song afresh. It
  never removes tracks from playlists.
- **Mappings → Blocked tracks → Unblock** lets a blocked track sync again.

### 2.6 Reconnect an account

If a provider revokes a token, the mapping shows **Reconnect needed** and the Dashboard
account card offers **Reconnect**.

### 2.7 Back up a playlist

1. **Backups → Add playlist** (or **Back up** on a Settings playlist page) picks any playlist
   from either account. Spotube reads it straight away and saves the first snapshot.
2. After that, the playlist is checked:
   - **on its schedule** (every 6 h to weekly, daily by default). A background check runs every
     10 minutes, whether or not sync workers are on (`BACKUPS_ENABLED`);
   - **whenever a sync reads it**, if the playlist is in a mapping. This is free, because the
     sync already fetched the tracks;
   - **when you press Back up now**.
3. If the name and track order are unchanged, the latest snapshot is marked "unchanged until"
   that check and nothing is added. Otherwise a new snapshot is saved, with what was added and
   removed. The oldest snapshots past the keep limit (30 by default) are deleted.
4. A YouTube check costs about 2 quota units per 50 videos. Spotify is free.

### 2.8 Restore a snapshot

Open a backup, pick a snapshot and choose **Restore…**:

- **Put back missing tracks** adds the snapshot's tracks that are no longer in the original
  playlist, at the end. Nothing is removed or reordered.
- **New playlist on the same service** creates a copy with every track in order. On YouTube it
  is private. Spotify currently ignores the private flag for apps, so the copy shows as public.
- **New playlist on the other service** uses the counterpart IDs from Matches. Tracks without
  a known counterpart are listed afterwards instead of being searched for.

On YouTube a restore costs 50 units per track added (plus 50 for a new playlist). The dialog
shows the estimate and refuses if it would exceed the quota left today. Each snapshot can also
be downloaded as **JSON** (complete, for scripts) or **CSV**.

## 3. Pages

The header is light, with a green → red rule along the top. Its nav reads dashboard ·
mappings · review · queue · matches · backups · activity. A status bar along the bottom
shows API health, review / pending / failed counts, today's YouTube quota and the number of
backups.

### Dashboard `/dashboard`
- **Accounts**: Spotify and YouTube connection state, with Playlists and Reconnect buttons.
- **Sync queue tiles**:
  - **Needs review** (amber when there is something to review; links to Review);
  - **Pending**, **Synced**, **Skipped** and **Failed**, each linking to the filtered queue;
  - **YouTube quota today**: measured units against 10,000; resets at midnight Pacific.
- **Paired playlists**: each mapping with its interval, last sync status and **Sync now**.
- **Recent activity**: the latest sync, executor and error messages.

### Mappings `/mappings`, `/mappings/new`, `/mappings/:id/edit`, `/mappings/:id/blacklist`
- **List**: playlist names with links to each service, options, last sync status and time,
  **Sync now**, **Edit**, **Blocked tracks** and **Delete**.
- **New** (`/mappings/new`): pick a Spotify playlist and a YouTube playlist, or create one side
  from the other, plus sync options.
- **Edit**: sync names, sync tracks and interval.
- **Blocked tracks**: tracks never synced for this mapping, with **Unblock**.

### Review `/review`
- Filters: direction (both, to YouTube, to Spotify) and playlist pair. These live in the URL
  (`?to=youtube&mapping=…`).
- One card per track. The header shows the source and destination playlists and the reason
  (e.g. "best score 0.80"). The two halves are the source track (title, artist, length, link)
  and the closest candidate (artwork, channel, length, score badge, **Use this**).
- Actions are listed in §2.4. The track picker dialog has three tabs: **Suggestions**
  (candidates from the last search, free), **Search {service}** and **Paste a link**.
- Click a score badge to see its breakdown: title %, artist %, length difference, and each
  bonus or penalty.
- Empty state: "Nothing to review", with a link to Matches.

### Sync Queue `/sync-queue`
- Status tabs: All, **Needs review**, Pending, Synced, Failed, Skipped. There is also a
  destination filter.
- Each row shows the track, direction and destination playlist, status and reason, and the
  last update. **Run now** or **Retry** appears on pending and failed rows; **Review** appears
  on review rows.
- Clicking a row opens the detail drawer: status, attempts, source link, why the track was
  queued (closest track in the playlist and its score), and every attempt with its search
  query, candidates and score breakdowns. It also has **Never sync this track**.

### Matches `/matches`
- Every remembered pair and unavailable song.
- Tabs: All, Paired, Not on YouTube, Not on Spotify. Search covers title, artist or track ID.
- Each row shows the song, the Spotify ID and YouTube ID (each linked, or "Not on …"), how the
  pair was decided ("Auto" with its score, or "You decided"), when it was last updated, and
  **Forget**.

### Activity `/logs`
- Tabs: All, Errors, Sync runs, Track copies, System. Rows have expandable details and a
  **View track** button that opens the sync item drawer. A reconnect link appears when a
  provider needs one. Pagination is 50 per page; logs older than 30 days are pruned.

### Backups `/backups`, `/backups/:id`
- **List**: each backed-up playlist with its service and schedule, track count and last change
  (+added / −removed, or "first"), snapshots kept out of the limit, when it was last checked,
  **Back up now** and **Open**. **Add playlist** opens a picker with a Spotify and a YouTube
  tab that filters the account's playlists.
- **Detail**: the playlist ID linked to the service, **Back up now** and **Delete** (deletes the
  backup and its snapshots, never the playlist). Settings: scheduled on/off, how often, how
  many snapshots to keep.
- **Snapshots** timeline, newest first, each with track count, change and source (first backup,
  scheduled, sync, manual). The selected snapshot shows its tracks with length, Spotify ID and
  YouTube ID, what changed since the one before, **JSON** / **CSV** downloads and **Restore…**.

### Settings `/settings/spotify`, `/settings/youtube`
- Every playlist in the connected account, filterable, with "⇄ synced with …" for paired ones
  and **Back up** (or the backup's state, linking to it).

### Setup `/setup`
- OAuth credentials form, shown until credentials exist.

### Design system `/design`
- Every UI component and the palette, for review. It isn't in the nav.

## 4. API limits

| Service | Limit | How Spotube stays inside it |
|---|---|---|
| YouTube Data API | 10,000 units/day (search 100, insert 50, list 1) | Remembered links skip searches; review items are never auto-retried; in-app search only runs on submit and is cached for 30 minutes; YouTube search is refused above 95% of the day's quota; usage is measured per call and shown on the Dashboard; a run stops on `quotaExceeded`. |
| Spotify Web API | Rolling rate limit (HTTP 429) | The client retries with back-off; searches are free. |

## 5. Data

| Table | Holds |
|---|---|
| `mappings` | Playlist pairs, options, last sync status |
| `sync_items` | Queued changes: `pending`, `running`, `done`, `error`, `skipped` or `review` |
| `track_links` | Song pairs (`auto` or `manual`); a NULL side means "not on that service" |
| `blacklist` | Blocked tracks per mapping |
| `activity_logs` | Sync, executor and system events with JSON details (30-day retention) |
| `api_usage` | YouTube units and calls per Pacific day |
| `playlist_backups` | Backed-up playlists: service, playlist ID, schedule, keep limit, last check and error |
| `playlist_snapshots` | One row per distinct version: name, counts, change, source, content hash, `confirmed_at`, and the tracks as JSON |
| `oauth_tokens`, `settings` | Tokens and OAuth client credentials |
