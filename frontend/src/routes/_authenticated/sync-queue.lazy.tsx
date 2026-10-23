import { createLazyFileRoute, useNavigate, useSearch } from '@tanstack/react-router'
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowRight, ChevronLeft, ChevronRight, Loader2, RotateCw } from 'lucide-react'
import {
  syncItemsAPI,
  isSyncItemExecutable,
  type SyncItemDetails,
  type SyncItemService,
  type SyncItemStatus,
} from '../../lib/api/sync-items'
import { SyncItemDetail } from '../../components/SyncItemDetail'
import { Card, Page, QueueStatusPill, ServiceIcon } from '../../components/ui'
import { buttonClass, fullTime, timeAgo } from '../../lib/format'

const PER_PAGE = 25

const statusTabs: Array<{ value: SyncItemStatus | undefined; label: string }> = [
  { value: undefined, label: 'All' },
  { value: 'pending', label: 'Pending' },
  { value: 'done', label: 'Synced' },
  { value: 'error', label: 'Failed' },
  { value: 'skipped', label: 'Skipped' },
]

function TrackCell({ item }: { item: SyncItemDetails }) {
  if (item.operation === 'rename') {
    return (
      <>
        <div className="font-medium text-gray-900">Rename playlist</div>
        <div className="text-xs text-gray-500">to “{item.track_title}”</div>
      </>
    )
  }
  return (
    <>
      <div className="font-medium text-gray-900">{item.track_title || item.track_id || '—'}</div>
      {item.track_artist && <div className="text-xs text-gray-500">{item.track_artist}</div>}
    </>
  )
}

function SyncQueuePage() {
  const queryClient = useQueryClient()
  const navigate = useNavigate({ from: '/sync-queue' })
  const { status } = useSearch({ from: '/_authenticated/sync-queue' })
  const [destination, setDestination] = useState<SyncItemService | ''>('')
  const [page, setPage] = useState(1)
  const [detailItemId, setDetailItemId] = useState<string | null>(null)
  const [notice, setNotice] = useState<{ ok: boolean; text: string } | null>(null)

  const setStatus = (value: SyncItemStatus | undefined) => {
    setPage(1)
    navigate({ search: value ? { status: value } : {} })
  }

  const { data, isLoading, error } = useQuery({
    queryKey: ['sync-items', status, destination, page],
    queryFn: () =>
      syncItemsAPI.getList({
        page,
        per_page: PER_PAGE,
        status,
        service: destination || undefined,
        order: 'desc',
      }),
    refetchOnMount: 'always',
  })

  const retry = useMutation({
    mutationFn: (id: string) => syncItemsAPI.execute(id),
    onMutate: () => setNotice(null),
    onSuccess: (result) => {
      const title = result.track_title || 'Item'
      if (result.status === 'done') setNotice({ ok: true, text: `${title}: synced.` })
      else if (result.status === 'skipped') setNotice({ ok: true, text: `${title}: ${result.error_message || 'skipped'}.` })
      else setNotice({ ok: false, text: `${title}: ${result.error_message || result.execution_log || 'failed'}` })
    },
    onError: (err: Error) => setNotice({ ok: false, text: err.message }),
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: ['sync-items'] })
      queryClient.invalidateQueries({ queryKey: ['dashboard-stats'] })
    },
  })

  const items = data?.items ?? []
  const totalPages = data?.totalPages ?? 1

  return (
    <Page
      title="Sync Queue"
      description="Tracks found on one side but not the other, and what happened when Spotube tried to copy them."
    >
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div className="inline-flex rounded-lg border border-gray-200 bg-white p-1" role="tablist">
          {statusTabs.map((tab) => (
            <button
              key={tab.label}
              type="button"
              role="tab"
              aria-selected={status === tab.value}
              onClick={() => setStatus(tab.value)}
              className={`rounded-md px-3 py-1.5 text-sm font-medium ${
                status === tab.value ? 'bg-gray-900 text-white' : 'text-gray-600 hover:text-gray-900'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>
        <label className="flex items-center gap-2 text-sm text-gray-600">
          Copying to
          <select
            className="rounded-lg border border-gray-300 bg-white px-2 py-1.5 text-sm"
            value={destination}
            onChange={(e) => {
              setPage(1)
              setDestination(e.target.value as SyncItemService | '')
            }}
          >
            <option value="">Either service</option>
            <option value="spotify">Spotify</option>
            <option value="youtube">YouTube</option>
          </select>
        </label>
      </div>

      {notice && (
        <div
          role="alert"
          className={`mb-4 rounded-lg border px-4 py-3 text-sm ${
            notice.ok ? 'border-green-200 bg-green-50 text-green-800' : 'border-red-200 bg-red-50 text-red-800'
          }`}
        >
          {notice.text}
        </div>
      )}

      {error && (
        <div className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800">
          Couldn't load the queue: {error.message}
        </div>
      )}

      <Card className="overflow-hidden">
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead className="border-b border-gray-200 bg-gray-50 text-left text-xs font-medium uppercase tracking-wide text-gray-500">
              <tr>
                <th className="px-4 py-3">Track</th>
                <th className="px-4 py-3">Direction</th>
                <th className="px-4 py-3">Status</th>
                <th className="hidden px-4 py-3 md:table-cell">Updated</th>
                <th className="px-4 py-3 text-right">
                  <span className="sr-only">Actions</span>
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100">
              {isLoading ? (
                <tr>
                  <td colSpan={5} className="px-4 py-10 text-center text-gray-500">
                    Loading…
                  </td>
                </tr>
              ) : items.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-4 py-10 text-center text-gray-500">
                    {status ? 'Nothing here.' : 'The queue is empty — both sides are in sync.'}
                  </td>
                </tr>
              ) : (
                items.map((item) => {
                  const running = retry.isPending && retry.variables === item.id
                  return (
                    <tr
                      key={item.id}
                      className="cursor-pointer align-top hover:bg-gray-50"
                      onClick={() => setDetailItemId(item.id)}
                    >
                      <td className="max-w-xs px-4 py-3">
                        <TrackCell item={item} />
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-1.5">
                          <ServiceIcon service={item.source_service} />
                          <ArrowRight className="h-3.5 w-3.5 text-gray-400" />
                          <ServiceIcon service={item.destination_service} />
                        </div>
                        <div className="mt-1 max-w-[12rem] truncate text-xs text-gray-500" title={item.destination_playlist_name}>
                          {item.destination_playlist_name}
                        </div>
                      </td>
                      <td className="max-w-sm px-4 py-3">
                        <QueueStatusPill status={item.status} />
                        {item.error_message && (
                          <div
                            className={`mt-1 line-clamp-2 text-xs ${item.status === 'error' ? 'text-red-700' : 'text-gray-500'}`}
                            title={item.error_message}
                          >
                            {item.error_message}
                          </div>
                        )}
                        {item.attempt_count > 1 && (
                          <div className="mt-0.5 text-xs text-gray-400">{item.attempt_count} attempts</div>
                        )}
                      </td>
                      <td className="hidden whitespace-nowrap px-4 py-3 text-xs text-gray-500 md:table-cell" title={fullTime(item.updated)}>
                        {timeAgo(item.updated)}
                      </td>
                      <td className="whitespace-nowrap px-4 py-3 text-right" onClick={(e) => e.stopPropagation()}>
                        {isSyncItemExecutable(item.status) && (
                          <button
                            type="button"
                            className={buttonClass.secondary}
                            disabled={retry.isPending}
                            onClick={() => retry.mutate(item.id)}
                          >
                            {running ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RotateCw className="h-3.5 w-3.5" />}
                            {item.status === 'pending' ? 'Run now' : 'Retry'}
                          </button>
                        )}
                      </td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>
        {data && data.totalItems > 0 && (
          <div className="flex items-center justify-between border-t border-gray-200 px-4 py-2.5 text-sm text-gray-500">
            <span>
              {data.totalItems.toLocaleString()} item{data.totalItems === 1 ? '' : 's'}
            </span>
            {totalPages > 1 && (
              <div className="flex items-center gap-2">
                <button type="button" className={buttonClass.ghost} disabled={page <= 1} onClick={() => setPage(page - 1)} aria-label="Previous page">
                  <ChevronLeft className="h-4 w-4" />
                </button>
                <span>
                  {page} / {totalPages}
                </span>
                <button type="button" className={buttonClass.ghost} disabled={page >= totalPages} onClick={() => setPage(page + 1)} aria-label="Next page">
                  <ChevronRight className="h-4 w-4" />
                </button>
              </div>
            )}
          </div>
        )}
      </Card>

      {detailItemId && <SyncItemDetail itemId={detailItemId} onClose={() => setDetailItemId(null)} />}
    </Page>
  )
}

export const Route = createLazyFileRoute('/_authenticated/sync-queue')({
  component: SyncQueuePage,
})
