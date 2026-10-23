import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Ban, ExternalLink, X } from 'lucide-react'
import { activityLogsAPI } from '../lib/api/activity-logs'
import { blacklistAPI } from '../lib/api/blacklist'
import { isSyncItemExecutable, syncItemsAPI } from '../lib/api/sync-items'
import { Pill, QueueStatusPill, ServiceIcon } from './ui'
import { buttonClass, fullTime, serviceName, timeAgo } from '../lib/format'

// Mirrors matching.MatchThreshold on the backend.
const MATCH_THRESHOLD = 0.85

type TrackRef = { platform?: string; id?: string; title?: string; artist?: string; channel?: string }

type Candidate = TrackRef & { rank: number; score: number }

type ExecutorRun = {
  search_query?: string
  candidates?: Candidate[]
  selected?: TrackRef
  already_in_playlist?: boolean
  outcome?: string
  error?: string
}

type AnalysisDecision = { best_match?: TrackRef; score?: number; matched?: boolean }

function trackURL(platform: string | undefined, id: string | undefined): string | null {
  if (!id) return null
  if (platform === 'youtube') return `https://www.youtube.com/watch?v=${id}`
  if (platform === 'spotify') return `https://open.spotify.com/track/${id}`
  return null
}

function TrackLink({ track, fallbackPlatform }: { track: TrackRef; fallbackPlatform?: string }) {
  const url = trackURL(track.platform ?? fallbackPlatform, track.id)
  const label = track.artist ? `${track.title} — ${track.artist}` : track.title || track.id || '—'
  if (!url) return <span>{label}</span>
  return (
    <a href={url} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 text-indigo-700 hover:underline">
      {label}
      <ExternalLink className="h-3 w-3 shrink-0" />
    </a>
  )
}

function Score({ value }: { value: number }) {
  const tone = value >= MATCH_THRESHOLD ? 'text-green-700' : value >= 0.5 ? 'text-amber-700' : 'text-gray-500'
  return <span className={`tabular-nums font-medium ${tone}`}>{value.toFixed(2)}</span>
}

const outcomeTone: Record<string, 'green' | 'red' | 'gray' | 'yellow'> = {
  added: 'green',
  already_in_destination: 'gray',
  no_match: 'yellow',
  error: 'red',
}

export function SyncItemDetail({ itemId, onClose }: { itemId: string; onClose: () => void }) {
  const { data: item, isLoading, error } = useQuery({
    queryKey: ['sync-item', itemId],
    queryFn: () => syncItemsAPI.getOne(itemId),
  })
  const { data: logs } = useQuery({
    queryKey: ['executor-logs', itemId],
    queryFn: () => activityLogsAPI.getList({ sync_item_id: itemId, job_type: 'executor', per_page: 10 }),
  })

  const queryClient = useQueryClient()
  // Blacklist the source track for this mapping; the backend marks the queued item skipped.
  const block = useMutation({
    mutationFn: async () => {
      if (!item?.track_id) return
      await blacklistAPI.create({
        mapping_id: item.mapping_id,
        service: item.service,
        track_id: item.track_id,
        reason: 'Blocked from the Sync Queue',
      })
    },
    onSettled: () => {
      for (const key of ['sync-item', 'sync-items', 'executor-logs', 'dashboard-stats', 'blacklist']) {
        queryClient.invalidateQueries({ queryKey: [key] })
      }
    },
  })

  const decision = (item?.analysis_context_json as { decision?: AnalysisDecision } | null)?.decision

  return (
    <div className="fixed inset-0 z-50 flex justify-end bg-black/30" onClick={onClose}>
      <aside
        role="dialog"
        aria-labelledby="sync-item-title"
        className="flex h-full w-full max-w-2xl flex-col bg-white shadow-xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-start justify-between gap-4 border-b border-gray-200 px-6 py-4">
          <div className="min-w-0">
            <h2 id="sync-item-title" className="truncate text-lg font-semibold text-gray-900">
              {item?.track_title || 'Sync item'}
            </h2>
            {item && (
              <p className="mt-0.5 flex items-center gap-1.5 text-sm text-gray-500">
                {item.track_artist && <span>{item.track_artist} ·</span>}
                <ServiceIcon service={item.source_service} className="h-3.5 w-3.5" />
                {item.source_playlist_name || serviceName(item.source_service)}
                <span>→</span>
                <ServiceIcon service={item.destination_service} className="h-3.5 w-3.5" />
                {item.destination_playlist_name || serviceName(item.destination_service)}
              </p>
            )}
          </div>
          <button type="button" onClick={onClose} className="text-gray-400 hover:text-gray-600" aria-label="Close">
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="flex-1 space-y-6 overflow-y-auto px-6 py-5 text-sm">
          {isLoading && <p className="text-gray-500">Loading…</p>}
          {error && <p className="text-red-600">Failed to load: {error.message}</p>}
          {item && (
            <>
              <section className="grid grid-cols-2 gap-x-6 gap-y-3 sm:grid-cols-4">
                <Field label="Status">
                  <QueueStatusPill status={item.status} />
                </Field>
                <Field label="Attempts">{item.attempt_count}</Field>
                <Field label="Queued">
                  <span title={fullTime(item.created)}>{timeAgo(item.created)}</span>
                </Field>
                <Field label="Updated">
                  <span title={fullTime(item.updated)}>{timeAgo(item.updated)}</span>
                </Field>
                {item.track_id && item.operation === 'add' && (
                  <div className="col-span-full">
                    <Field label={`Source on ${serviceName(item.source_service)}`}>
                      <TrackLink
                        track={{ id: item.track_id, title: item.track_title, artist: item.track_artist }}
                        fallbackPlatform={item.source_service}
                      />
                    </Field>
                  </div>
                )}
                {item.error_message && (
                  <div className="col-span-full">
                    <Field label={item.status === 'skipped' ? 'Reason' : 'Last error'}>
                      <span className={item.status === 'error' ? 'text-red-700' : 'text-gray-700'}>{item.error_message}</span>
                    </Field>
                  </div>
                )}
              </section>

              {decision && (
                <section>
                  <h3 className="mb-2 font-medium text-gray-900">Why it was queued</h3>
                  <p className="text-gray-600">
                    Closest track already in the {serviceName(item.destination_service)} playlist:{' '}
                    {decision.best_match ? (
                      <>
                        <TrackLink track={decision.best_match} /> (score <Score value={decision.score ?? 0} />)
                      </>
                    ) : (
                      'none'
                    )}
                    . A match needs {MATCH_THRESHOLD.toFixed(2)}.
                  </p>
                </section>
              )}

              <section>
                <h3 className="mb-2 font-medium text-gray-900">Attempts</h3>
                {logs?.items?.length ? (
                  <ol className="space-y-4">
                    {logs.items.map((log) => {
                      const run = ((log.details_json as { data?: ExecutorRun } | null)?.data ?? {}) as ExecutorRun
                      return (
                        <li key={log.id} className="rounded-lg border border-gray-200">
                          <div className="flex flex-wrap items-center gap-2 border-b border-gray-100 px-3 py-2">
                            {run.outcome && <Pill tone={outcomeTone[run.outcome] ?? 'gray'}>{run.outcome.replace(/_/g, ' ')}</Pill>}
                            <span className="text-xs text-gray-500" title={fullTime(log.created)}>
                              {timeAgo(log.created)}
                            </span>
                            {run.search_query && (
                              <span className="text-xs text-gray-500">
                                searched <span className="font-mono text-gray-700">“{run.search_query}”</span>
                              </span>
                            )}
                          </div>
                          {run.error && <p className="px-3 pt-2 text-xs text-red-700">{run.error}</p>}
                          {run.candidates?.length ? (
                            <table className="w-full text-xs">
                              <thead className="text-left text-gray-500">
                                <tr>
                                  <th className="px-3 py-1.5 font-medium">#</th>
                                  <th className="px-3 py-1.5 font-medium">Candidate</th>
                                  <th className="px-3 py-1.5 text-right font-medium">Score</th>
                                </tr>
                              </thead>
                              <tbody className="divide-y divide-gray-100">
                                {run.candidates.map((c) => (
                                  <tr key={`${c.rank}-${c.id}`} className={run.selected?.id === c.id ? 'bg-green-50' : ''}>
                                    <td className="px-3 py-1.5 text-gray-400">{c.rank}</td>
                                    <td className="px-3 py-1.5">
                                      <TrackLink track={c} fallbackPlatform={item.destination_service} />
                                      {c.channel && <div className="text-gray-400">{c.channel}</div>}
                                    </td>
                                    <td className="px-3 py-1.5 text-right">
                                      <Score value={c.score} />
                                    </td>
                                  </tr>
                                ))}
                              </tbody>
                            </table>
                          ) : (
                            !run.error && <p className="px-3 py-2 text-xs text-gray-500">{log.message}</p>
                          )}
                        </li>
                      )
                    })}
                  </ol>
                ) : (
                  <p className="text-gray-500">Not attempted yet.</p>
                )}
              </section>

              {item.operation === 'add' && item.track_id && isSyncItemExecutable(item.status) && (
                <section className="rounded-lg border border-gray-200 p-3">
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <p className="text-gray-600">
                      Wrong song, or you don't want it on {serviceName(item.destination_service)}?
                    </p>
                    <button type="button" className={buttonClass.secondary} disabled={block.isPending} onClick={() => block.mutate()}>
                      <Ban className="h-4 w-4" /> Never sync this track
                    </button>
                  </div>
                  {block.error && <p className="mt-2 text-xs text-red-700">{block.error.message}</p>}
                </section>
              )}

              <details className="text-xs text-gray-500">
                <summary className="cursor-pointer select-none">Raw data</summary>
                <pre className="mt-2 max-h-80 overflow-auto rounded-lg bg-gray-50 p-3 text-gray-700">
                  {JSON.stringify({ item, executor_logs: logs?.items }, null, 2)}
                </pre>
              </details>
            </>
          )}
        </div>
      </aside>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <dt className="text-xs font-medium text-gray-500">{label}</dt>
      <dd className="mt-1 break-words text-gray-900">{children}</dd>
    </div>
  )
}
