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
| **Schedule** | When a mapping is checked: every N minutes from a start time on chosen days, in your time zone (daily = once at the start time). |
| **Quota budget** | How much of today's 10,000 YouTube units each kind of work may use, so copying never starves scheduled checks or your own searches. |
| **Setting** | An app-wide option on the Settings page. A value set in `backend/.env` is used at startup; changing it on the page lasts until the next restart. Other settings are saved in the database. |
| **Snapshot** | The playlist's name and tracks, in order, at one point in time, with each track's Spotify and YouTube IDs where Spotube knows both. A new snapshot is saved only when something changed. |

## 2. Workflows

### 2.1 First run (setup)

1. Open `http://127.0.0.1:5173`. With no OAuth credentials, every page redirects to **Setup**.
2. Enter the Spotify and Google client ID and secret (or put them in `backend/.env`).
3. On the **Dashboard**, connect Spotify, then YouTube. Each opens the provider's consent page
   and returns to the Dashboard with a "connected" message.

### 2.2 Pair two playlists

**Mappings → Pair playlists**: pick one playlist from each account, choose whether to sync
names and tracks, and set the schedule.

- **Schedule.** Spotube recommends one as soon as both playlists are picked: how often by size
  (small playlists every 30 minutes, very large ones daily), less often while all your mappings'
  daily checks would use more than 15% of the YouTube quota, and a start time spaced away from
  your other mappings. Change the interval, start time or days to override it; **Use
  recommended** goes back. A warning appears when even a daily check is over that share.
- **Estimate.** Below the form: the YouTube units the copy needs (searching, adding, reading,
  creating) and when it will be done behind the work already queued. For a Spotify playlist
  copied to a new YouTube playlist it is near-exact (songs Spotube already matched don't need
  a search); otherwise it says "Up to". Over 5,000 videos it warns that YouTube playlists stop
  there.
- **Start syncing now** (default from Settings) runs the first sync in the background straight
  away; off, it waits for the first scheduled run. The mapping shows **Syncing** meanwhile.

- A playlist can be in only one mapping; already-paired playlists are greyed out (the API
  returns 409 for them).
- **Only on one side?** Choose **New {service} playlist** at the top of the other list instead.
  Spotube creates it (named after the source playlist unless you type a name), pairs it, and
  the first sync copies the tracks across. Creating a YouTube playlist costs 50 quota units and
  is private; Spotify ignores the private flag for apps, so a new Spotify playlist is public.
  Videos with no confident match (talks, mixes) go to Review as usual.

### 2.2a Mirror several playlists

**Mappings → Mirror playlists** (or **Mirror several** on the pairing page): choose a direction
(Spotify → YouTube or YouTube → Spotify), tick playlists (search and **Select all** help), and
**Create … copies**. Each playlist becomes its own mapping with a new playlist on the other side
and its own recommended, spaced schedule.

- Playlists already in a mapping are listed last, greyed out with **Already paired**; mappings
  are strictly one-to-one. **Same name on …** flags a playlist whose name already exists on the
  other side, in case pairing them is what you want.
- The estimate covers all selected playlists.
- Results list each playlist: **created**, **already paired**, **waiting for quota** (not
  enough YouTube quota left today to create the playlist; those stay ticked so you can retry
  after the reset) or **failed** with the reason. Nothing is half-created: if a mapping can't be
  saved, the new playlist is deleted again.

### 2.2b Delete a mapping

The trash icon opens a dialog. Both playlists are kept by default; tick **Also delete the
Spotify playlist** (unfollows it, which for someone else's playlist only removes it from your
library) and/or **Also delete the YouTube playlist** (permanent, 50 units). The mapping is
deleted first; if a playlist can't be deleted the dialog says why.

### 2.3 A sync run

A run starts when a mapping's next run time comes and **Background sync** is on, or straight
away with **Sync now**.

```
1. Read Spotify in full                         free
   Probe YouTube                                1 unit: title and video count
   Read YouTube in full only if the count changed, nothing is cached, or the cache is stale
                                                ~1.5 units per 50 videos (durations only for unmatched videos)
2. Rename the YouTube playlist if needed        50 units, only when names differ and sync_name is on
3. For each track on each side:
     a. linked?  counterpart present → nothing to do
                 marked "not on <service>" → nothing to do, never searched
     b. otherwise fuzzy-match against the other playlist (score ≥ 85% → present, and remembered as a link)
     c. otherwise queue an `add` sync item
4. Stamp the mapping with last sync time, status and next run
   Sync now also applies this mapping's queue straight away
```

Spotube's own additions don't count as changes. A cached read is "stale" after a day per 1,000
videos (at most a week); Sync now re-reads unless the last read is under 10 minutes old.

**Applying the queue.** With **Apply changes automatically** on, every minute Spotube applies
up to 100 queued items across all mappings: renames first, then songs it already knows on the
other side (50 units each), then the rest, taking turns between mappings so one huge playlist
doesn't hold up the others. It stops YouTube work when the quota budget says so; Spotify work
continues. This runs even when Background sync is off.

**Applying one add item:**

```
linked to a known counterpart?   → add it directly (no search)
marked not on the destination?   → skip ("not on YouTube")
already in the destination?      → skip, and remember the pair
search the destination           Spotify: free · YouTube: 101 units
best result ≥ 85%                → remember the pair, then add it (YouTube: 50 units)
nothing confident                → status "Needs review" (never retried automatically)
not enough quota / rate limited  → stays "Pending" (attempts unchanged), tried again later
YouTube playlist full (5,000)    → skipped; YouTube adds for this mapping pause ("Playlist full")
YouTube playlist gone            → mapping shows "Playlist missing"; its items wait
API error                        → status "Failed", tried up to 3 times (15 min, then 1 h apart)
```

Before starting an add, Spotube checks that the quota can cover all of it (151 units for a
search and insert), so it never pays for a search and then can't add the result.

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
mappings · review · queue · matches · backups · activity · settings. A status bar along the bottom
shows API health, review / pending / failed counts, today's YouTube quota and the number of
backups.

### Dashboard `/dashboard`
- **Accounts**: Spotify and YouTube connection state, with Playlists and Reconnect buttons.
- **Sync queue tiles**:
  - **Needs review** (amber when there is something to review; links to Review);
  - **Pending**, **Synced**, **Skipped** and **Failed**, each linking to the filtered queue;
  - **YouTube quota today**: measured units against the daily limit; resets at midnight Pacific.
- **YouTube quota today** panel: units used, a bar split into used / scheduled checks still to
  come / free / kept for you, what the queue can still use today, scheduled checks per day, the
  YouTube work queued (items and units) and when it finishes. When Google reports the quota
  used up it says Spotube is waiting.
- **Paired playlists**: each mapping with how often it runs, last sync status and **Sync now**.
- **Recent activity**: the latest sync, executor and error messages.

### Mappings `/mappings`, `/mappings/new`, `/mappings/:id/edit`, `/mappings/:id/blacklist`
- **List**: playlist names with links to each service, options, last sync status and time
  (OK, Syncing, Failed, Reconnect needed, Waiting for quota, Playlist missing, Playlist full),
  the schedule and next run, tracks on each side, and queue counts (queued, to review, failed)
  that open the Queue or Review filtered to that mapping. **Sync now**, **Edit**, **Blocked
  tracks** and **Delete** (§2.2b). **Mirror playlists** and **Pair playlists** at the top.
- **New** (`/mappings/new`): §2.2.
- **Mirror** (`/mappings/mirror`): §2.2a.
- **Edit**: sync names, sync tracks and schedule, with each side's track count and the next run.
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
  destination filter, and a mapping filter (`?mapping=…`, set from the Mappings page) shown as
  a removable chip.
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

### Settings `/settings`
- Grouped: **Sync**, **YouTube quota** (with the quota panel) and **Backups**. Switches for
  on/off settings, a number and **Save** for the rest.
- Each shows where its value comes from: **default**, **saved** (database), **.env** (set at
  startup), or **until restart** (changed here while `.env` sets it; the `.env` value returns on
  restart, and the row says so).
- Links to the playlist libraries.

### Playlist libraries `/settings/spotify`, `/settings/youtube`
- Every playlist in the connected account, filterable, with "⇄ synced with …" for paired ones
  and **Back up** (or the backup's state, linking to it).

### Setup `/setup`
- OAuth credentials form, shown until credentials exist.

### Design system `/design`
- Every UI component and the palette, for review. It isn't in the nav.

## 4. API limits

| Service | Limit | How Spotube stays inside it |
|---|---|---|
| YouTube Data API | 10,000 units/day (search 100, insert 50, list 1); setting **YouTube daily quota** | One budget gates every call (interactive / scheduled reads / queue), keeping a reserve for you and room for the day's scheduled checks; jobs are pre-checked so nothing is half-paid; quota refusals defer instead of failing and are re-checked after 30 minutes; checks probe for 1 unit and re-read only on change; remembered links skip searches; review items are never auto-retried; in-app search runs on submit, is cached for 30 minutes and is refused (429) when the quota can't cover it. |
| Spotify Web API | Rolling rate limit (HTTP 429) | The client retries with back-off; searches are free. |

## 5. Data

| Table | Holds |
|---|---|
| `mappings` | Playlist pairs (each playlist at most once), options, schedule and next run, track counts per side, the YouTube video count Spotube expects, last sync status |
| `playlist_cache` | The latest full read of each mapped playlist, so unchanged checks and restarts don't re-read |
| `app_settings` | Settings saved from the Settings page |
| `sync_items` | Queued changes: `pending`, `running`, `done`, `error`, `skipped` or `review` |
| `track_links` | Song pairs (`auto` or `manual`); a NULL side means "not on that service" |
| `blacklist` | Blocked tracks per mapping |
| `activity_logs` | Sync, executor and system events with JSON details (30-day retention) |
| `api_usage` | YouTube units and calls per Pacific day |
| `playlist_backups` | Backed-up playlists: service, playlist ID, schedule, keep limit, last check and error |
| `playlist_snapshots` | One row per distinct version: name, counts, change, source, content hash, `confirmed_at`, and the tracks as JSON |
| `oauth_tokens`, `settings` | Tokens and OAuth client credentials |
