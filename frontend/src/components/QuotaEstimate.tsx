import { useQuery } from '@tanstack/react-query'
import { Gauge } from 'lucide-react'
import { quotaAPI, type EstimatePair } from '../lib/api/index'
import { daysLabelFromNow } from '../lib/schedule'
import { Notice } from './ui'

const n = (v: number) => v.toLocaleString()

/** What copying the proposed playlists costs in YouTube quota, and when the queue finishes it. */
export function QuotaEstimate({ pairs }: { pairs: EstimatePair[] }) {
  const estimate = useQuery({
    queryKey: ['quota-estimate', pairs],
    queryFn: () => quotaAPI.estimate(pairs),
    enabled: pairs.length > 0,
    staleTime: 60_000,
  })
  if (pairs.length === 0) return null
  if (estimate.isLoading) {
    return <p className="text-xs text-gray-500">Estimating YouTube quota…</p>
  }
  const e = estimate.data
  if (!e) return null

  const parts = [
    e.search_units && `${n(e.search_units)} searching (${n(e.work.to_youtube)} songs)`,
    e.insert_units && `${n(e.insert_units)} adding`,
    e.read_units && `${n(e.read_units)} reading`,
    e.create_units && `${n(e.create_units)} creating`,
  ].filter(Boolean)

  return (
    <div className="space-y-2" aria-label="Quota estimate">
      <div className="flex items-start gap-2.5 rounded-md border border-gray-200 bg-gray-50 px-3.5 py-2.5 text-sm text-gray-700">
        <Gauge className="mt-0.5 h-4 w-4 shrink-0 text-gray-400" aria-hidden="true" />
        <div className="min-w-0 space-y-0.5">
          {e.units === 0 ? (
            <p>No YouTube quota needed{e.work.to_spotify ? `; ${n(e.work.to_spotify)} songs to add on Spotify (free)` : ''}.</p>
          ) : (
            <p>
              {e.upper_bound ? 'Up to ' : 'About '}
              <span className="font-semibold text-ink">{n(e.units)}</span> YouTube units
              {parts.length > 0 && <span className="text-gray-500"> ({parts.join(', ')})</span>}. Done{' '}
              <span className="font-semibold text-ink">{daysLabelFromNow(e.days)}</span>
              {e.backlog_units > 0 && <> behind {n(e.backlog_units)} units already queued</>}.
            </p>
          )}
          <p className="text-xs text-gray-500">
            The queue gets about {n(e.daily_capacity)} units a day after scheduled checks and the amount kept for you.
            {e.upper_bound && ' Songs already on both sides cost nothing, so the real cost is likely lower.'}
            {e.exact && e.work.to_youtube_linked > 0 && ` ${n(e.work.to_youtube_linked)} songs are already matched (no search).`}
          </p>
        </div>
      </div>
      {e.over_youtube_limit && (
        <Notice tone="warning">
          YouTube playlists hold at most 5,000 videos. Spotube adds until the playlist is full, then pauses YouTube adds for
          this mapping.
        </Notice>
      )}
    </div>
  )
}
