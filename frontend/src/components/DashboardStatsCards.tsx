import { Link } from '@tanstack/react-router'
import type { DashboardStats } from '../lib/api/index'
import { Card } from './ui'

type QueueKey = 'pending' | 'done' | 'skipped' | 'error'

const tiles: Array<{ key: QueueKey; label: string; hint: string; accent: string }> = [
  { key: 'pending', label: 'Pending', hint: 'Waiting for the next sync', accent: 'bg-amber-400' },
  { key: 'done', label: 'Synced', hint: 'Added to the other side', accent: 'bg-spotify' },
  { key: 'skipped', label: 'Skipped', hint: 'Already there, blocked or unavailable', accent: 'bg-gray-300' },
  { key: 'error', label: 'Failed', hint: 'API errors; retried automatically', accent: 'bg-youtube' },
]

/** Queue counters (each links to the filtered queue), the review count and today's YouTube quota. */
export function DashboardStatsCards({ stats, isLoading }: { stats?: DashboardStats; isLoading: boolean }) {
  const value = (n: number | undefined) =>
    isLoading ? (
      <span className="inline-block h-7 w-10 animate-pulse rounded bg-gray-200" role="status" aria-label="Loading" />
    ) : (
      (n ?? 0).toLocaleString()
    )

  const review = stats?.queue.review ?? 0
  const quota = stats?.youtube_quota
  const quotaPct = quota && quota.limit > 0 ? Math.min(100, Math.round((quota.used / quota.limit) * 100)) : 0

  return (
    <div className="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-6">
      <Link to="/review" className="block">
        <Card
          className={`h-full p-4 transition-colors ${
            review > 0 ? 'border-amber-300 bg-amber-50 hover:border-amber-400' : 'hover:border-gray-300'
          }`}
        >
          <div className="flex items-center gap-2 text-sm font-medium text-gray-700">
            <span className="h-2 w-2 rounded-full bg-amber-400" />
            Needs review
          </div>
          <div className="mt-2 text-2xl font-semibold tabular-nums text-ink">{value(stats?.queue.review)}</div>
          <p className="mt-1 text-xs text-gray-600">{review > 0 ? 'Pick the right track →' : 'No decisions waiting'}</p>
        </Card>
      </Link>
      {tiles.map((tile) => (
        <Link key={tile.key} to="/sync-queue" search={{ status: tile.key }} className="block">
          <Card className="h-full p-4 transition-colors hover:border-gray-300">
            <div className="flex items-center gap-2 text-sm font-medium text-gray-600">
              <span className={`h-2 w-2 rounded-full ${tile.accent}`} />
              {tile.label}
            </div>
            <div className="mt-2 text-2xl font-semibold tabular-nums text-ink">{value(stats?.queue[tile.key])}</div>
            <p className="mt-1 text-xs text-gray-500">{tile.hint}</p>
          </Card>
        </Link>
      ))}
      <Card className="h-full p-4">
        <div className="text-sm font-medium text-gray-600">YouTube quota today</div>
        <div className="mt-2 text-2xl font-semibold tabular-nums text-ink">
          {value(quota?.used)}
          <span className="text-sm font-normal text-gray-400"> / {(quota?.limit ?? 10000).toLocaleString()}</span>
        </div>
        <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-gray-100" aria-label={`${quotaPct}% of daily quota`}>
          <div
            className={`h-full rounded-full ${quotaPct > 80 ? 'bg-youtube' : quotaPct > 50 ? 'bg-amber-400' : 'bg-spotify'}`}
            style={{ width: `${quotaPct}%` }}
          />
        </div>
        <p className="mt-1 text-xs text-gray-500">Measured · resets midnight PT</p>
      </Card>
    </div>
  )
}
