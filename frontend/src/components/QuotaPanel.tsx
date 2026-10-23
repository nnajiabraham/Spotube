import { useQuery } from '@tanstack/react-query'
import { quotaAPI } from '../lib/api/index'
import { daysLabelFromNow } from '../lib/schedule'
import { Card } from './ui'

const n = (v: number) => v.toLocaleString()

/** Today's YouTube quota budget: who may use what, and when the queue is done. */
export function QuotaPanel({ className = '' }: { className?: string }) {
  const { data: q } = useQuery({
    queryKey: ['quota'],
    queryFn: quotaAPI.status,
    refetchInterval: 60_000,
  })
  if (!q) return null

  const pct = (v: number) => `${Math.max(0, Math.min(100, (v / Math.max(q.limit, 1)) * 100))}%`
  const resets = new Date(q.reset_at * 1000).toLocaleTimeString(undefined, {
    hour: '2-digit',
    minute: '2-digit',
  })
  const queued = q.backlog.youtube_unlinked + q.backlog.youtube_linked + q.backlog.youtube_renames

  return (
    <Card className={`p-4 ${className}`}>
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <p className="text-sm text-ink">
          <span className="text-2xl font-medium tabular-nums">{n(q.used)}</span>
          <span className="text-gray-400"> / {n(q.limit)} units used today</span>
        </p>
        <p className="text-xs text-gray-500">resets {resets}</p>
      </div>
      {/* used | expected scheduled reads | kept for you | free for the queue */}
      <div className="mt-2 flex h-2 overflow-hidden rounded-full bg-spotify-100" aria-hidden="true">
        <span className="h-full bg-ink" style={{ width: pct(q.used) }} />
        <span className="h-full bg-sky-300" style={{ width: pct(q.reads_still_expected) }} />
        <span className="ml-auto h-full bg-amber-300" style={{ width: pct(q.reserve) }} />
      </div>
      <dl className="mt-3 grid grid-cols-2 gap-x-4 gap-y-1.5 text-xs sm:grid-cols-4">
        <div>
          <dt className="text-gray-400">
            <span className="mr-1 inline-block h-2 w-2 rounded-full bg-spotify" />
            Queue can use
          </dt>
          <dd className="tabular-nums text-ink">{n(q.background_available)}</dd>
        </div>
        <div>
          <dt className="text-gray-400">
            <span className="mr-1 inline-block h-2 w-2 rounded-full bg-sky-300" />
            Scheduled checks
          </dt>
          <dd className="tabular-nums text-ink">
            {n(q.daily_reads)}/day ({n(q.reads_still_expected)} to come)
          </dd>
        </div>
        <div>
          <dt className="text-gray-400">
            <span className="mr-1 inline-block h-2 w-2 rounded-full bg-amber-300" />
            Kept for you
          </dt>
          <dd className="tabular-nums text-ink">{n(q.reserve)}</dd>
        </div>
        <div>
          <dt className="text-gray-400">Queued for YouTube</dt>
          <dd className="tabular-nums text-ink">
            {n(queued)} ({n(q.backlog.units)} units)
          </dd>
        </div>
      </dl>
      <p className="mt-3 text-xs text-gray-500">
        {q.exhausted
          ? 'YouTube reported the quota used up; Spotube checks again shortly and waits for the reset if needed.'
          : queued === 0
            ? 'Nothing waiting for YouTube.'
            : `At about ${n(q.daily_capacity)} units a day for the queue, it finishes ${daysLabelFromNow(q.days_to_drain)}.`}
      </p>
    </Card>
  )
}
