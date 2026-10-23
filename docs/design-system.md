# Spotube design system

Spotube sits between Spotify and YouTube, so its colours are theirs: green, red and ink,
softened to pastels on warm light grey. It is set entirely in a monospace face (JetBrains Mono)
on a faint dot grid, like a terminal or graph paper: dense, aligned and quiet, with colour
reserved for meaning. To see the components live, run the frontend and open `/design`.

## Palette

Tokens are defined in `frontend/src/index.css` (`@theme`) and used as Tailwind classes such as
`bg-ink`, `text-spotify-700` and `bg-youtube-50`. The theme also overrides Tailwind's `white`
and `gray` scale, so existing `bg-white` and `text-gray-*` classes pick up the warm greys.

| Token | Hex | Use |
|---|---|---|
| `canvas` | `#f2f2ef` | Page background, with a 22px dot grid |
| `white` | `#fbfbfa` | Card surface (never pure white) |
| `gray-200` / `gray-500` | `#e1e1dc` / `#6f6f6a` | Hairline borders / secondary text |
| `ink` | `#1c1c1a` | Primary buttons, headings, the active nav item and tab text |
| `ink-700` / `ink-500` | `#3a3a37` / `#6f6f6a` | Hover on ink, muted ink |
| `spotify` | `#5fd892` | Spotify mark, success dots, fills |
| `spotify-700` | `#1d7745` | Green **text** (AA contrast on the card surface) |
| `spotify-100` / `-50` | `#cdeed9` / `#e7f6ec` | Pill and notice borders / backgrounds |
| `youtube` | `#ff6b7d` | YouTube mark, error dots, fills |
| `youtube-700` | `#b02a3c` | Red **text** and destructive buttons (AA contrast) |
| `youtube-100` / `-50` | `#fad4d9` / `#fcedef` | Pill and notice borders / backgrounds |
| `amber-400` / `-700` | `#f3cb63` / `#85590a` | "Needs you": review and pending (dot / text) |
| `sky-50` / `-700` | `#e8f1fa` / `#2b5d8c` | Running, informational notices |

**Type and shape**

- One family: `font-sans` and `font-mono` are both JetBrains Mono, with slashed zero. The root
  size is 14px because monospace runs wide.
- Page titles are prefixed with a grey `/` and sit above a hairline rule. Section titles and
  field labels are 11px uppercase with wide tracking.
- Corners are fine: 3–6px (`rounded-md` on controls, `rounded-lg` on cards and dialogs). No
  drop shadows on cards or buttons; borders do the work.
- Numbers use `tabular-nums` so columns line up.

**Rules**

- **Ink is the only primary colour.** Green and red stand for the two services and for
  outcomes, never for decoration.
- **Green** means synced, connected or backed up. **Red** means YouTube, a failure or a
  destructive action. **Amber** means a decision is waiting.
- Pastel fills (`spotify`, `youtube`) are for marks, dots and bars. Text uses the `-700` shades.
- The header is light, with a 3px rule along the top running green → amber → red: Spotify into
  YouTube. Navigation is lowercase; the active item is an ink chip.
- A status bar is pinned to the bottom of every page: API health, review / pending / failed
  counts, the YouTube quota bar and the number of backups (with failures).
- The logo is two arrows on ink, green going out and red coming back (`SpotubeMark`,
  `public/favicon.svg`).

## Components (`frontend/src/components/ui`)

| Component | Use |
|---|---|
| `Page` | Page title, description, actions and width |
| `Card`, `SectionTitle` | Light bordered surface; uppercase section label with an optional link |
| `Button` | `primary` (ink), `secondary` (outlined), `ghost`, `danger` (red), `spotify` (green); `size="sm"`; `loading`; `icon` |
| `Tabs` | Segmented control for filters: a grey track with the active segment raised on the card surface |
| `Pill`, `QueueStatusPill` | Status labels. Queue statuses: Needs review and Pending (amber), Running (sky), Synced (green), Failed (red), Skipped (grey) |
| `CountBadge` | Amber count in the nav and on tabs |
| `ScoreBadge` | Match % (green ≥ 85, amber ≥ 60, grey below). With a breakdown, a click explains the score |
| `TrackRow`, `TrackArt` | A track or video: artwork (square for Spotify, 16:9 for YouTube), title linking to the service, artist, channel, album, length, trailing actions |
| `Notice` | Inline message: `success`, `warning`, `error` (announced as an alert), `info` |
| `EmptyState` | Icon, title, help text and an optional action for empty lists |
| `Dialog`, `Drawer` | Modal for short tasks such as the track picker; right-hand panel for details. Escape or a click outside closes them |
| `Pagination` | Count and previous/next |
| `ServiceIcon`, `SpotifyLogo`, `YoutubeLogo`, `SpotubeMark` | Brand marks |

`TrackArt` falls back to the video's YouTube thumbnail (built from its ID, no quota) when no
artwork is stored.

## Feature components (`frontend/src/components`)

| Component | Use |
|---|---|
| `QuotaPanel` | Today's YouTube budget: used / limit, a bar (ink used, sky scheduled checks still to come, pale green free, amber kept for you), what the queue can use, queued work and when it finishes. Dashboard and Settings |
| `QuotaEstimate` | Units and days a proposed pairing or mirror needs, with a warning past 5,000 videos |
| `ScheduleFields`, `MappingOptions` | Interval select, start time, day toggles (`aria-pressed`) with Every day / Weekdays / Weekends presets, and a one-line description; options add the sync toggles and a note slot for the recommendation |
| `DeleteMappingDialog` | Delete a mapping; checkboxes (off by default) to also delete either playlist; per-playlist results |

The Settings page uses a switch (`role="switch"`, ink when on) for on/off settings and a
number field with **Save** for the rest. A setting's source is a pill: grey **default** or
**.env**, green **saved**, amber **until restart**.

Class helpers live in `frontend/src/lib/format.ts`: `buttonClass` (for links styled as
buttons), `linkClass`, `sectionLinkClass`, `inputClass`, `formatDuration`, `trackURL` and
`youtubeThumbnail`. Schedule labels (`describeSchedule`, `intervalLabel`, `nextRunLabel`,
`daysLabelFromNow`) are in `frontend/src/lib/schedule.ts`.

## Writing

- Name things the way the user sees them: "Needs review", "Not on YouTube", "Forget",
  "Use this".
- Say what happens and what it costs: "Each YouTube search uses 101 of the 10,000 daily quota
  units."
- Confirm results in a sentence: "Added “Fearless” to YT Worship. Spotube will remember this
  match."
