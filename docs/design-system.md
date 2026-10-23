# Spotube design system

Spotube sits between Spotify and YouTube, so its look borrows from both. The palette is the
near-black surface both apps use, Spotify green and YouTube red. Everything else is neutral
grey. To see the components live, run the frontend and open `/design`.

## Palette

Tokens are defined in `frontend/src/index.css` (`@theme`) and used as Tailwind classes such as
`bg-ink`, `text-spotify-700` and `ring-youtube/30`.

| Token | Hex | Use |
|---|---|---|
| `ink` | `#0f0f0f` | Header, primary buttons, headings, active tabs |
| `ink-700` / `ink-500` | `#2e2e2e` / `#6b6b6b` | Hover on ink, muted ink |
| `spotify` | `#1ed760` | Spotify brand mark, success dots, "connect" buttons |
| `spotify-700` | `#117a37` | Green **text** on white (AA contrast) |
| `spotify-50` | `#ebfbf1` | Success backgrounds, the selected candidate |
| `youtube` | `#ff0033` | YouTube brand mark, error dots |
| `youtube-700` | `#c4001f` | Red **text** on white and destructive buttons (AA contrast) |
| `youtube-50` | `#fff0f2` | Error backgrounds |
| `amber-400` (Tailwind) | `#fbbf24` | "Needs you": review and pending |

**Rules**

- **Ink is the only primary colour.** Green and red stand for the two services and for
  outcomes, never for decoration.
- **Green** means synced or connected. **Red** means YouTube, a failure or a destructive action.
  **Amber** means a decision is waiting.
- Bright brand colours (`spotify`, `youtube`) are used for marks, dots and fills. Text uses
  the `-700` shades.
- The header has a 2px rule running from green to red: Spotify into YouTube.
- The logo is two arrows on ink, green going out and red coming back (`SpotubeMark`,
  `public/favicon.svg`).

## Components (`frontend/src/components/ui`)

| Component | Use |
|---|---|
| `Page` | Page title, description, actions and width |
| `Card`, `SectionTitle` | White bordered surface; uppercase section label with an optional link |
| `Button` | `primary` (ink), `secondary` (outlined), `ghost`, `danger` (red), `spotify` (green); `size="sm"`; `loading`; `icon` |
| `Tabs` | Segmented control for filters; the active tab is ink |
| `Pill`, `QueueStatusPill` | Status labels. Queue statuses: Needs review and Pending (amber), Running (sky), Synced (green), Failed (red), Skipped (grey) |
| `CountBadge` | Amber count in the nav and on tabs |
| `ScoreBadge` | Match % (green ≥ 85, amber ≥ 60, grey below). With a breakdown, a click explains the score |
| `TrackRow`, `TrackArt` | A track or video: artwork (square for Spotify, 16:9 for YouTube), title linking to the service, artist, channel, album, length, trailing actions |
| `Notice` | Inline message: `success`, `warning`, `error` (announced as an alert), `info` |
| `EmptyState` | Icon, title, help text and an optional action for empty lists |
| `Dialog`, `Drawer` | Modal for short tasks such as the track picker; right-hand panel for details. Escape or a click outside closes them |
| `Pagination` | Count and previous/next |
| `ServiceIcon`, `SpotifyLogo`, `YoutubeLogo`, `SpotubeMark` | Brand marks |

Class helpers live in `frontend/src/lib/format.ts`: `buttonClass` (for links styled as
buttons), `linkClass`, `sectionLinkClass`, `inputClass`, `formatDuration` and `trackURL`.

## Writing

- Name things the way the user sees them: "Needs review", "Not on YouTube", "Forget",
  "Use this".
- Say what happens and what it costs: "Each YouTube search uses 101 of the 10,000 daily quota
  units."
- Confirm results in a sentence: "Added “Fearless” to YT Worship. Spotube will remember this
  match."
