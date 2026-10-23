import { useId, useState } from 'react'
import type { ScoreBreakdown } from '../../lib/api/tracks'

// Mirrors matching.MatchThreshold on the backend.
export const MATCH_THRESHOLD = 0.85

function tone(value: number) {
  if (value >= MATCH_THRESHOLD) return 'bg-spotify-50 text-spotify-700 ring-spotify/40'
  if (value >= 0.6) return 'bg-amber-50 text-amber-800 ring-amber-200'
  return 'bg-gray-100 text-gray-600 ring-gray-200'
}

function pct(v: number) {
  return `${Math.round(v * 100)}%`
}

/**
 * Match score as a percentage, coloured by confidence (green ≥ 85%, the auto-match bar).
 * With a breakdown, clicking it explains the score: title and artist similarity, length
 * difference, and each bonus or penalty.
 */
export function ScoreBadge({ value, breakdown }: { value: number; breakdown?: ScoreBreakdown | null }) {
  const [open, setOpen] = useState(false)
  const id = useId()
  const badge = `inline-flex items-center rounded-md px-1.5 py-0.5 text-xs font-semibold tabular-nums ring-1 ring-inset ${tone(value)}`
  if (!breakdown) return <span className={badge} title="Match score">{pct(value)}</span>

  return (
    <span className="relative inline-block">
      <button
        type="button"
        className={`${badge} cursor-help`}
        aria-expanded={open}
        aria-controls={id}
        onClick={(e) => {
          e.stopPropagation()
          setOpen(!open)
        }}
        onBlur={() => setOpen(false)}
        title="Why this score?"
      >
        {pct(value)}
      </button>
      {open && (
        <span
          id={id}
          role="tooltip"
          className="absolute right-0 top-full z-20 mt-1 block w-60 rounded-lg bg-ink p-3 text-left text-xs font-normal text-gray-200 shadow-xl"
        >
          <span className="mb-1.5 block font-semibold text-white">Match {pct(breakdown.total)}</span>
          <Row label="Title" value={pct(breakdown.title)} />
          <Row label="Artist" value={breakdown.artist_known ? pct(breakdown.artist) : 'unknown'} />
          {breakdown.duration_diff_sec != null && (
            <Row label="Length difference" value={`${breakdown.duration_diff_sec}s`} />
          )}
          {breakdown.adjustments?.map((a) => (
            <Row
              key={a.reason}
              label={a.reason}
              value={`${a.delta > 0 ? '+' : ''}${Math.round(a.delta * 100)}`}
              tone={a.delta > 0 ? 'text-spotify' : 'text-youtube'}
            />
          ))}
          <span className="mt-1.5 block border-t border-white/10 pt-1.5 text-gray-400">
            {pct(MATCH_THRESHOLD)} or more is matched automatically.
          </span>
        </span>
      )}
    </span>
  )
}

function Row({ label, value, tone = 'text-white' }: { label: string; value: string; tone?: string }) {
  return (
    <span className="flex justify-between gap-3 py-0.5">
      <span className="first-letter:uppercase">{label}</span>
      <span className={`tabular-nums ${tone}`}>{value}</span>
    </span>
  )
}
