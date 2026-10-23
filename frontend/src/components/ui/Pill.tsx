import type { ReactNode } from 'react'

const pillTones = {
  gray: 'bg-gray-100 text-gray-700 ring-gray-200',
  green: 'bg-spotify-50 text-spotify-700 ring-spotify/40',
  yellow: 'bg-amber-50 text-amber-800 ring-amber-200',
  red: 'bg-youtube-50 text-youtube-700 ring-youtube/30',
  ink: 'bg-ink text-white ring-ink',
  blue: 'bg-sky-50 text-sky-700 ring-sky-200',
}

export type PillTone = keyof typeof pillTones

export function Pill({ tone = 'gray', children, title }: { tone?: PillTone; children: ReactNode; title?: string }) {
  return (
    <span
      title={title}
      className={`inline-flex items-center gap-1 whitespace-nowrap rounded-full px-2 py-0.5 text-xs font-medium ring-1 ring-inset ${pillTones[tone]}`}
    >
      {children}
    </span>
  )
}

// Amber means "needs you": pending work and items waiting for review.
const queueStatusTone: Record<string, PillTone> = {
  pending: 'yellow',
  running: 'blue',
  done: 'green',
  error: 'red',
  skipped: 'gray',
  review: 'yellow',
}

const queueStatusLabel: Record<string, string> = {
  pending: 'Pending',
  running: 'Running',
  done: 'Synced',
  error: 'Failed',
  skipped: 'Skipped',
  review: 'Needs review',
}

export function QueueStatusPill({ status }: { status: string }) {
  return <Pill tone={queueStatusTone[status] ?? 'gray'}>{queueStatusLabel[status] ?? status}</Pill>
}

/** Small count bubble for nav items and tabs. */
export function CountBadge({ count, tone = 'yellow' }: { count?: number; tone?: 'yellow' | 'gray' }) {
  if (!count) return null
  return (
    <span
      className={`ml-1.5 inline-flex min-w-5 items-center justify-center rounded-full px-1.5 text-[11px] font-semibold tabular-nums leading-5 ${
        tone === 'yellow' ? 'bg-amber-400 text-ink' : 'bg-gray-200 text-gray-700'
      }`}
    >
      {count > 99 ? '99+' : count}
    </span>
  )
}
