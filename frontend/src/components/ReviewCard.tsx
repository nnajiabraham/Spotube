import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ArrowRight, Ban, Ellipsis, EyeOff, RotateCw, Search } from 'lucide-react'
import { syncItemsAPI, type ResolveAction, type SyncItemDetails } from '../lib/api/sync-items'
import { blacklistAPI } from '../lib/api/blacklist'
import { YOUTUBE_SEARCH_COST, type SearchHit } from '../lib/api/tracks'
import { serviceName } from '../lib/format'
import { invalidateQueue, sourceTrackOf } from '../lib/review'
import { Button, Card, Dialog, ScoreBadge, ServiceIcon, TrackRow } from './ui'
import { TrackPicker } from './TrackPicker'

export type ReviewOutcome = { ok: boolean; text: string }

/**
 * One track waiting for a decision: what Spotube looked for, its closest find, and every way
 * to settle it. Decisions are remembered, so the same song is never asked about twice.
 */
export function ReviewCard({ item, onOutcome }: { item: SyncItemDetails; onOutcome: (o: ReviewOutcome) => void }) {
  const queryClient = useQueryClient()
  const [picking, setPicking] = useState(false)
  const [moreOpen, setMoreOpen] = useState(false)
  const dest = item.destination_service
  const destName = serviceName(dest)
  const source = sourceTrackOf(item)
  const title = source.title || 'Track'
  const best = [...(item.last_run?.candidates ?? [])].sort((a, b) => b.score - a.score)[0]

  const resolve = useMutation({
    mutationFn: (body: ResolveAction) => syncItemsAPI.resolve(item.id, body),
    onSuccess: (result, body) => {
      setPicking(false)
      if (result.status === 'done') onOutcome({ ok: true, text: `Added “${title}” to ${result.destination_playlist_name || destName}. Spotube will remember this match.` })
      else if (body.action === 'unavailable') onOutcome({ ok: true, text: `“${title}” marked as not on ${destName}. It won't be searched again.` })
      else if (result.status === 'review') onOutcome({ ok: false, text: `Still no confident match for “${title}”: ${result.error_message || 'try searching yourself'}.` })
      else if (result.status === 'skipped') onOutcome({ ok: true, text: `“${title}”: ${result.error_message || 'skipped'}.` })
      else onOutcome({ ok: false, text: `“${title}”: ${result.error_message || result.execution_log || 'failed'}` })
    },
    onError: (err: Error) => onOutcome({ ok: false, text: err.message }),
    onSettled: () => invalidateQueue(queryClient),
  })

  const block = useMutation({
    mutationFn: () =>
      blacklistAPI.create({
        mapping_id: item.mapping_id,
        service: item.service,
        track_id: item.track_id ?? '',
        reason: 'Chosen in Review',
      }),
    onSuccess: () => onOutcome({ ok: true, text: `“${title}” won't be synced to ${item.destination_playlist_name || destName}.` }),
    onError: (err: Error) => onOutcome({ ok: false, text: err.message }),
    onSettled: () => invalidateQueue(queryClient),
  })

  const busy = resolve.isPending || block.isPending
  const pickedId = resolve.isPending && resolve.variables?.action === 'match' ? resolve.variables.target_id : null
  const use = (hit: SearchHit) => resolve.mutate({ action: 'match', target_id: hit.id })

  return (
    <Card className="overflow-hidden">
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1 border-b border-gray-100 bg-gray-50/60 px-4 py-2 text-xs text-gray-500">
        <ServiceIcon service={item.source_service} className="h-3.5 w-3.5" />
        <span className="max-w-[14rem] truncate">{item.source_playlist_name}</span>
        <ArrowRight className="h-3 w-3" />
        <ServiceIcon service={dest} className="h-3.5 w-3.5" />
        <span className="max-w-[14rem] truncate">{item.destination_playlist_name}</span>
        {item.error_message && <span className="ml-auto text-amber-800">{item.error_message}</span>}
      </div>

      <div className="grid gap-px bg-gray-100 md:grid-cols-2">
        <div className="bg-white p-2">
          <p className="px-3 pt-1 text-[11px] font-semibold uppercase tracking-wide text-gray-400">
            On {serviceName(item.source_service)}
          </p>
          <TrackRow service={item.source_service} track={source} />
        </div>
        <div className="bg-white p-2">
          <p className="px-3 pt-1 text-[11px] font-semibold uppercase tracking-wide text-gray-400">Closest on {destName}</p>
          {best ? (
            <TrackRow
              service={dest}
              track={best}
              trailing={
                <>
                  <ScoreBadge value={best.score} breakdown={best.breakdown} />
                  <Button size="sm" variant="primary" loading={pickedId === best.id} disabled={busy} onClick={() => use(best)}>
                    Use this
                  </Button>
                </>
              }
            />
          ) : (
            <p className="px-3 py-4 text-sm text-gray-500">Nothing came close.</p>
          )}
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-2 border-t border-gray-100 px-4 py-3">
        <Button variant="secondary" size="sm" icon={<Search className="h-3.5 w-3.5" />} disabled={busy} onClick={() => setPicking(true)}>
          Find on {destName}
        </Button>
        <Button
          variant="secondary"
          size="sm"
          icon={<EyeOff className="h-3.5 w-3.5" />}
          loading={resolve.isPending && resolve.variables?.action === 'unavailable'}
          disabled={busy}
          onClick={() => resolve.mutate({ action: 'unavailable' })}
          title={`Remember that this song isn't on ${destName}; it won't be searched again in any playlist`}
        >
          Not on {destName}
        </Button>
        <div className="relative ml-auto">
          <Button variant="ghost" size="sm" aria-expanded={moreOpen} aria-label="More actions" onClick={() => setMoreOpen(!moreOpen)}>
            <Ellipsis className="h-4 w-4" />
          </Button>
          {moreOpen && (
            <div className="absolute right-0 z-10 mt-1 w-64 rounded-lg border border-gray-200 bg-white p-1 shadow-lg" onMouseLeave={() => setMoreOpen(false)}>
              <MenuItem
                icon={<RotateCw className="h-4 w-4" />}
                disabled={busy}
                onClick={() => {
                  setMoreOpen(false)
                  resolve.mutate({ action: 'retry' })
                }}
                hint={dest === 'youtube' ? `Uses ${YOUTUBE_SEARCH_COST} quota units` : 'Free'}
              >
                Search again automatically
              </MenuItem>
              <MenuItem
                icon={<Ban className="h-4 w-4" />}
                disabled={busy}
                onClick={() => {
                  setMoreOpen(false)
                  block.mutate()
                }}
                hint="Only for this playlist pair"
              >
                Don't sync this track
              </MenuItem>
            </div>
          )}
        </div>
      </div>

      {picking && (
        <Dialog
          size="lg"
          title={`Find “${title}” on ${destName}`}
          subtitle={source.artist}
          onClose={() => !resolve.isPending && setPicking(false)}
        >
          <TrackPicker item={item} onPick={use} picking={pickedId} />
        </Dialog>
      )}
    </Card>
  )
}

function MenuItem({
  icon,
  children,
  hint,
  onClick,
  disabled,
}: {
  icon: React.ReactNode
  children: React.ReactNode
  hint?: string
  onClick: () => void
  disabled?: boolean
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="flex w-full items-start gap-2.5 rounded-md px-3 py-2 text-left text-sm text-ink hover:bg-gray-100 disabled:opacity-50"
    >
      <span className="mt-0.5 text-gray-500">{icon}</span>
      <span>
        {children}
        {hint && <span className="block text-xs text-gray-500">{hint}</span>}
      </span>
    </button>
  )
}
