# RFC-011: Track matching, review queue, and track links

- **Status:** Accepted, implemented on `fix/revive-sync-engine`
- **Date:** 2026-10-23
- **Scope:** backend matching + sync engine, new review/override UI, design system refresh

## 1. Problems

Observed while syncing the `Testingspotubee` pair live:

| # | Problem | Example |
|---|---------|---------|
| P1 | **Same title, different song.** Title-only similarity is high for unrelated songs that share a title. | "Fearless — Fearless Worship" vs "Fearless — Jasmine Murray" scored 0.65. Correctly rejected, but nothing stopped a near-miss from passing. |
| P2 | **Noisy YouTube titles.** Uploads carry "(Official Video)", "Lyrics", "x Collab", "| Live at …", channel suffixes like "VEVO" or "- Topic". | "Umbrella x Moses Bliss, …" needed a special noise rule. |
| P3 | **Wrong versions.** Live, remix, cover, sped-up, karaoke and 1-hour loops look like the original song by title. | Sermon and "Sunday Service" videos showed up as candidates for a worship song. |
| P4 | **Repeated failed searches.** A track that isn't on the other service is searched again on every retry. A YouTube search costs **100 of 10,000** daily units. | "Fearless" was searched 3 times (300 units) for the same no-match. |
| P5 | **No memory of what was synced.** After copying Spotify track S to YouTube video Y, the next run compares Y back to Spotify by fuzzy metadata. If that fuzzy match fails, Y is queued to be copied *back* to Spotify, and a different release of the same song can be added as a duplicate. | The reverse direction depends on fuzzy matching that already failed once. |
| P6 | **No way to fix a miss.** When matching fails the only options are "retry" and "never sync". The user cannot say "this is the right track" or "this song is not on Spotify". | — |

## 2. Goals

1. Fewer wrong matches and fewer misses, without new infrastructure.
2. Never search twice for something we already know the answer to.
3. Every synced track has exactly one counterpart on the other service, which prevents duplicates in both directions.
4. Failed matches land in a **review queue** where the user can pick the right track (searched in the app, chosen from earlier candidates, or pasted as a link) or mark it **not available** on that service.
5. Stay well inside API limits. YouTube quota is the binding constraint.

## 3. Matching alternatives considered

### A. Better deterministic scoring (chosen)

Add signals to the existing normalized title/artist comparison:

- **Duration.** Spotify returns `duration_ms` on every track. YouTube durations come from `videos.list` (`contentDetails.duration`), which costs **1 unit per 50 videos**. A song and its official upload are usually within a few seconds of each other. A 45-minute sermon or a 1-hour loop is not. This is the strongest signal for P1 and P3.
- **Version keywords.** Penalize live, remix, cover, acoustic, instrumental, karaoke, sped up, slowed, reverb, nightcore, 8D, extended or loop when one side has the keyword and the other doesn't.
- **Channel trust.** YouTube Music "Artist - Topic" channels and channels named after the artist are official uploads, so they get a small bonus.
- **Character-level similarity.** Blend token Jaccard with trigram similarity so spelling variants, missing apostrophes and transliterations still score well.
- **Artist containment.** Score the source artist against the candidate's parsed artist, channel and raw title together. This handles "Title - Artist" vs "Artist - Title" uploads.
- **Better searches.** YouTube searches are restricted to the Music category (`videoCategoryId=10`). Spotify uses field filters (`track:"…" artist:"…"`) first, then falls back to free text.
- **Explainable scores.** Every candidate stores a breakdown (title, artist, duration difference, penalties, bonuses) so the UI can show *why* it was or wasn't chosen.

Cost: about 1 extra YouTube unit per search and per 50 playlist videos listed. No new dependencies.

### B. Embedding similarity (fastembed): not now

[fastembed](https://github.com/qdrant/fastembed) runs small ONNX embedding models (for example `bge-small-en-v1.5`, about 130 MB, or `all-MiniLM-L6-v2`, about 90 MB) on the CPU. The backend is Go, so the options are:

| Option | What it takes |
|---|---|
| `fastembed-go` (Go port) | CGO plus the ONNX Runtime shared library, installed per OS; first run downloads the model. Makes `go build`, CI and the Docker image more complex. |
| Python fastembed sidecar | A second runtime and process to install, start and monitor. |
| Hosted embeddings API | Network dependency, cost, and sends track data to a third party. |

More importantly, embeddings solve the wrong problem here. They measure *semantic* closeness, but our failures are *different songs with near-identical text* ("Fearless" by two artists) and *noise tokens*. An embedding model rates "Fearless — Jasmine Murray" and "Fearless — Fearless Worship" as very similar, which is the exact mistake to avoid. Duration and exact artist agreement separate them; cosine similarity does not.

**Decision:** don't add embeddings. Scoring sits behind one function (`matching.ScoreCandidate`), so an embedding tie-breaker can be added later if a class of misses shows up that A cannot fix (for example cross-language titles). This RFC is the place to revisit that.

### C. Other options noted

- **ISRC.** Spotify exposes ISRC codes; the YouTube Data API does not. Useful only for Spotify↔Spotify. Rejected.
- **Unofficial YouTube Music API (ytmusicapi).** Its "songs" search is excellent and costs no quota, but it is unofficial, Python-only, cookie-authenticated and against YouTube's terms. Rejected.
- **LLM judge for borderline scores.** Possible later for the 0.6–0.85 band. Needs an API key and adds cost and latency. Deferred, since human review covers it for now.

## 4. Track links (memory of synced pairs)

New table `track_links`, global and shared by all mappings:

| column | meaning |
|---|---|
| `spotify_id` | Spotify track ID, or NULL when the song is known **not to be on Spotify** |
| `youtube_id` | YouTube video ID, or NULL when the song is known **not to be on YouTube** |
| `title`, `artist` | label for the UI |
| `origin` | `auto` (the sync engine found it) or `manual` (the user chose it) |
| `score` | match score when `auto` |

Each non-NULL side is unique, so a track has at most one counterpart. A manual choice replaces any conflicting link.

**Where links are written**
- The executor copies S → Y successfully: auto link S↔Y.
- The executor finds the chosen candidate already in the destination: auto link.
- The user picks a track in review: manual link.
- The user marks "Not on YouTube/Spotify": link with the other side NULL.

**Where links are read**
- **Analysis.** Before fuzzy matching, check for a link. If the linked counterpart is in the destination playlist, the track is present, and fuzzy matching is skipped. If the link says "not on that service", nothing is queued. This fixes P5: a copied track is never copied back.
- **Executor.** If a link exists, add the linked ID directly with no search. This saves 100 units per YouTube add. If the link says "not available", the item is skipped. This fixes P4.

Links are global because whether a song exists on a service doesn't depend on which playlist pair is asking. Blocked tracks stay per mapping, because "don't sync this here" is a per-pair choice.

## 5. Review queue

New sync item status **`review`**: the engine searched and wasn't confident.

- A no-match goes to `review` instead of `error`, and auto-apply never retries it. This fixes P4.
- Real errors (network, quota, auth) stay `error` and retry up to 3 times, as before.
- A migration moves existing no-match `error` rows to `review`.

**Review page** (`/review`, with a count badge in the nav), filterable by mapping. Each card shows the source track (title, artist, duration, link), the direction, what was searched, and the best candidates from the last search with score breakdowns. Showing those candidates is free, with no new search. Actions:

| Action | Effect | API cost |
|---|---|---|
| **Use this** (on a candidate) | manual link, then add immediately | YT add 50 / Spotify free |
| **Search {service}** | in-app search dialog; pick a result | YT 101 per search / Spotify free |
| **Paste a link** | resolve a YouTube/Spotify URL or ID, then pick | YT 1 / Spotify free |
| **Not on {service}** | link with an empty side; item skipped; never searched again | none |
| **Don't sync** | blocks the track for this mapping (existing blocked tracks) | none |
| **Search again** | back to pending for a fresh automatic search | YT 101 |

**Matches page** (`/matches`): every link, filterable by Linked / Not on Spotify / Not on YouTube, with text search. Each row can be **removed**, so the engine decides again next time. Removing a link does not edit playlists.

## 6. API limits

- **Measured YouTube quota.** An HTTP transport on the Google client records the unit cost of every YouTube call (search 100, insert/update 50, list 1) per Pacific day in `api_usage`. The dashboard shows measured usage instead of an estimate from logs.
- **Quota guard.** In-app YouTube search is refused above 95% of the daily quota.
- **Server-side search cache.** Identical searches within 30 minutes are answered from memory, so re-renders and refreshes never re-spend quota.
- **Client.** Search runs only on explicit submit, never while typing. Search results use `staleTime: Infinity`. Window-focus refetch stays off.
- **Engine.** Links skip searches. `review` items are never retried automatically. Auto-apply is capped at 25 items per mapping per run.

## 7. Design system

The palette blends the two brands: **ink** black (`#0f0f0f`, both apps' dark surfaces), **Spotify green** (`#1ed760`, text-safe `#117a37`) and **YouTube red** (`#ff0033`, text-safe `#c4001f`). Rules:

- Ink for primary actions and the header.
- Green for success: synced and connected.
- Red for YouTube identity, errors and destructive actions.
- Amber for "needs you": review and pending.

Components live in `frontend/src/components/ui/` and are shown on a `/design` page. See `docs/design-system.md`.

## 8. Rollout and tests

1. Migration: `track_links`, `api_usage`, `sync_items.status` gains `review` (table rebuild), and existing no-match errors move to review.
2. Matching: scoring v2, with unit tests built from the real cases seen live (Fearless, Umbrella, Pelo Negro, Moving Train, Color Caramelo) plus synthetic live/remix/long-video cases.
3. Engine: links in analysis and executor, the review status, measured quota. Tested against fake platforms.
4. API: search, lookup, resolve, links.
5. UI: design system, Review, Matches, the updated queue, dashboard and mappings pages, and the restyled settings and setup pages.
6. Live verification on the test pair, done carefully within quota: one sync, one review resolution, one "not available", and a duplicate check of both playlists.

## 9. Out of scope (follow-ups)

- Removing tracks from playlists (removal sync).
- "Wrong match" correction that also deletes the wrongly added track from the destination playlist.
- Embedding or LLM tie-breakers (see 3B and 3C).
