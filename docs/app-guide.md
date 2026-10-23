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

## 2. Workflows

### 2.1 First run (setup)

1. Open `http://127.0.0.1:5173`. With no OAuth credentials, every page redirects to **Setup**.
2. Enter the Spotify and Google client ID and secret (or put them in `backend/.env`).
3. On the **Dashboard**, connect Spotify, then YouTube. Each opens the provider's consent page
   and returns to the Dashboard with a "connected" message.

### 2.2 Pair two playlists

**Mappings → New mapping**: pick one playlist from each account, choose whether to sync names
and tracks, and set the interval. The mapping syncs on its first run.

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

## 3. Pages

The header is black. Its nav reads Dashboard · Mappings · Review (count badge) · Sync Queue ·
Matches · Activity.

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
- **New**: pick a Spotify playlist and a YouTube playlist, plus sync options.
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

### Settings `/settings/spotify`, `/settings/youtube`
- Connection status for one provider, with its playlists.

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
| `oauth_tokens`, `settings` | Tokens and OAuth client credentials |
