import { Link } from '@tanstack/react-router'
import type { DashboardStats } from '../lib/api/index'
import { Card } from './ui'

type QueueKey = 'pending' | 'done' | 'skipped' | 'error'

const tiles: Array<{ key: QueueKey; label: string; hint: string; accent: string }> = [
  { key: 'pending', label: 'Pending', hint: 'Waiting for the next sync', accent: 'bg-amber-400' },
  { key: 'done', label: 'Synced', hint: 'Added to the other side', accent: 'bg-green-500' },
  { key: 'skipped', label: 'Skipped', hint: 'Already there or removed', accent: 'bg-gray-300' },
  { key: 'error', label: 'Failed', hint: 'No confident match or API error', accent: 'bg-red-500' },
]

function Tile({ children, status }: { children: React.ReactNode; status?: QueueKey }) {
  const body = <Card className="h-full p-4 transition-colors hover:border-gray-300">{children}</Card>
  if (!status) return body
  return (
    <Link to="/sync-queue" search={{ status }} className="block">
      {body}
    </Link>
  )
}

/** Queue counters (each links to the filtered queue) plus today's estimated YouTube quota use. */
export function DashboardStatsCards({ stats, isLoading }: { stats?: DashboardStats; isLoading: boolean }) {
  const value = (n: number | undefined) =>
    isLoading ? (
      <span className="inline-block h-7 w-10 animate-pulse rounded bg-gray-200" role="status" aria-label="Loading" />
    ) : (
      (n ?? 0).toLocaleString()
    )

  const quota = stats?.youtube_quota
  const quotaPct = quota && quota.limit > 0 ? Math.min(100, Math.round((quota.used / quota.limit) * 100)) : 0

  return (
    <div className="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-5">
      {tiles.map((tile) => (
        <Tile key={tile.key} status={tile.key}>
          <div className="flex items-center gap-2 text-sm font-medium text-gray-600">
            <span className={`h-2 w-2 rounded-full ${tile.accent}`} />
            {tile.label}
          </div>
          <div className="mt-2 text-2xl font-semibold tabular-nums text-gray-900">{value(stats?.queue[tile.key])}</div>
          <p className="mt-1 text-xs text-gray-500">{tile.hint}</p>
        </Tile>
      ))}
      <Tile>
        <div className="text-sm font-medium text-gray-600">YouTube quota today</div>
        <div className="mt-2 text-2xl font-semibold tabular-nums text-gray-900">
          {value(quota?.used)}
          <span className="text-sm font-normal text-gray-400"> / {(quota?.limit ?? 10000).toLocaleString()}</span>
        </div>
        <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-gray-100" aria-label={`${quotaPct}% of daily quota`}>
          <div
            className={`h-full rounded-full ${quotaPct > 80 ? 'bg-red-500' : 'bg-indigo-500'}`}
            style={{ width: `${quotaPct}%` }}
          />
        </div>
        <p className="mt-1 text-xs text-gray-500">Estimate · searches cost 100</p>
      </Tile>
    </div>
  )
}
