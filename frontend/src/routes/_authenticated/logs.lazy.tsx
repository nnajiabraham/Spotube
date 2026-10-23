import { createLazyFileRoute, Link } from '@tanstack/react-router'
import { useState } from 'react'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { ChevronDown, ChevronLeft, ChevronRight, RefreshCw } from 'lucide-react'
import { activityLogsAPI, mappingsAPI } from '../../lib/api/index'
import type { ActivityLog } from '../../lib/api/types'
import type { ActivityLogsListOptions } from '../../lib/api/activity-logs'
import { SyncItemDetail } from '../../components/SyncItemDetail'
import { Card, Page } from '../../components/ui'
import { buttonClass, fullTime, linkClass, timeAgo } from '../../lib/format'

const PER_PAGE = 50

type Filter = Pick<ActivityLogsListOptions, 'level' | 'job_type'>

const tabs: { label: string; filter: Filter }[] = [
  { label: 'All', filter: {} },
  { label: 'Errors', filter: { level: 'error' } },
  { label: 'Sync runs', filter: { job_type: 'analysis' } },
  { label: 'Track copies', filter: { job_type: 'executor' } },
  { label: 'System', filter: { job_type: 'system' } },
]

const jobLabel: Record<ActivityLog['job_type'], string> = {
  analysis: 'Sync run',
  executor: 'Track copy',
  system: 'System',
}

const levelDot: Record<ActivityLog['level'], string> = {
  info: 'bg-gray-300',
  warn: 'bg-amber-500',
  error: 'bg-youtube',
}

function needsReconnect(message: string): string | null {
  if (!/reconnect required|invalid_grant|no token found/i.test(message)) return null
  if (/spotify/i.test(message)) return 'Spotify'
  if (/youtube|google/i.test(message)) return 'YouTube'
  return 'the account'
}

function LogRow({
  log,
  mappingName,
  onOpenItem,
}: {
  log: ActivityLog
  mappingName?: string
  onOpenItem: (id: string) => void
}) {
  const [open, setOpen] = useState(false)
  const reconnect = log.level === 'error' ? needsReconnect(log.message) : null

  return (
    <li className="px-4 py-3">
      <div className="flex gap-3">
        <span className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${levelDot[log.level]}`} aria-label={log.level} />
        <div className="min-w-0 flex-1">
          <p className={`break-words text-sm ${log.level === 'error' ? 'text-youtube-700' : 'text-gray-800'}`}>{log.message}</p>
          <div className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-gray-500">
            <span title={fullTime(log.created)}>{timeAgo(log.created)}</span>
            <span className="text-gray-300">·</span>
            <span>{jobLabel[log.job_type]}</span>
            {mappingName && (
              <>
                <span className="text-gray-300">·</span>
                <span className="truncate">{mappingName}</span>
              </>
            )}
            {reconnect && (
              <>
                <span className="text-gray-300">·</span>
                <Link to="/dashboard" className={linkClass}>
                  Reconnect {reconnect}
                </Link>
              </>
            )}
            {log.sync_item_id && (
              <>
                <span className="text-gray-300">·</span>
                <button
                  type="button"
                  className={linkClass}
                  onClick={() => onOpenItem(log.sync_item_id!)}
                >
                  View track
                </button>
              </>
            )}
            {log.details_json && (
              <>
                <span className="text-gray-300">·</span>
                <button
                  type="button"
                  className="inline-flex items-center gap-0.5 hover:text-gray-800"
                  aria-expanded={open}
                  onClick={() => setOpen(!open)}
                >
                  Details {open ? <ChevronDown className="h-3 w-3" /> : <ChevronRight className="h-3 w-3" />}
                </button>
              </>
            )}
          </div>
          {open && log.details_json && (
            <pre className="mt-2 max-h-72 overflow-auto rounded-lg bg-gray-50 p-3 text-xs text-gray-700 ring-1 ring-gray-200">
              {JSON.stringify(log.details_json, null, 2)}
            </pre>
          )}
        </div>
      </div>
    </li>
  )
}

function ActivityLogsPage() {
  const [tab, setTab] = useState(0)
  const [page, setPage] = useState(1)
  const [openItemId, setOpenItemId] = useState<string | null>(null)
  const filter = tabs[tab].filter

  const { data, isLoading, isFetching, error, refetch } = useQuery({
    queryKey: ['activity-logs', 'page', filter, page],
    queryFn: () => activityLogsAPI.getList({ ...filter, page, per_page: PER_PAGE }),
    placeholderData: keepPreviousData,
    refetchInterval: 30_000,
  })
  const { data: mappings } = useQuery({
    queryKey: ['mappings', 'all'],
    queryFn: () => mappingsAPI.getList({ per_page: 200 }),
  })
  const mappingNames = new Map(
    (mappings?.items ?? []).map((m) => [
      m.id,
      `${m.spotify_playlist_name || m.spotify_playlist_id} ⇄ ${m.youtube_playlist_name || m.youtube_playlist_id}`,
    ]),
  )

  const logs = data?.items ?? []
  const totalPages = Math.max(data?.totalPages ?? 1, 1)

  return (
    <Page
      title="Activity"
      description="What the sync engine did, newest first. Entries older than 30 days are deleted automatically."
      actions={
        <button type="button" className={buttonClass.secondary} onClick={() => refetch()} disabled={isFetching}>
          <RefreshCw className={`h-4 w-4 ${isFetching ? 'animate-spin' : ''}`} /> Refresh
        </button>
      }
    >
      <div className="mb-4 inline-flex flex-wrap rounded-lg border border-gray-200 bg-white p-1" role="tablist">
        {tabs.map((t, i) => (
          <button
            key={t.label}
            type="button"
            role="tab"
            aria-selected={tab === i}
            onClick={() => {
              setTab(i)
              setPage(1)
            }}
            className={`rounded-md px-3 py-1.5 text-sm font-medium ${
              tab === i ? 'bg-gray-900 text-white' : 'text-gray-600 hover:text-gray-900'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>

      {error && (
        <div className="mb-4 rounded-lg border border-youtube-100 bg-youtube-50 px-4 py-3 text-sm text-youtube-700">
          Couldn't load activity: {error.message}
        </div>
      )}

      <Card>
        {isLoading ? (
          <p className="p-6 text-sm text-gray-500" role="status">
            Loading activity…
          </p>
        ) : logs.length === 0 ? (
          <p className="p-10 text-center text-sm text-gray-500">
            {tab === 0 ? 'Nothing has run yet.' : 'Nothing here.'}
          </p>
        ) : (
          <ul className="divide-y divide-gray-100">
            {logs.map((log) => (
              <LogRow
                key={log.id}
                log={log}
                mappingName={log.mapping_id ? mappingNames.get(log.mapping_id) : undefined}
                onOpenItem={setOpenItemId}
              />
            ))}
          </ul>
        )}
        {data && data.totalItems > 0 && (
          <div className="flex items-center justify-between border-t border-gray-100 px-4 py-2.5 text-sm text-gray-500">
            <span>{data.totalItems.toLocaleString()} entries</span>
            {totalPages > 1 && (
              <div className="flex items-center gap-1">
                <button
                  type="button"
                  className={buttonClass.ghost}
                  disabled={page <= 1}
                  onClick={() => setPage(page - 1)}
                  aria-label="Previous page"
                >
                  <ChevronLeft className="h-4 w-4" />
                </button>
                <span>
                  {page} / {totalPages}
                </span>
                <button
                  type="button"
                  className={buttonClass.ghost}
                  disabled={page >= totalPages}
                  onClick={() => setPage(page + 1)}
                  aria-label="Next page"
                >
                  <ChevronRight className="h-4 w-4" />
                </button>
              </div>
            )}
          </div>
        )}
      </Card>

      {openItemId && <SyncItemDetail itemId={openItemId} onClose={() => setOpenItemId(null)} />}
    </Page>
  )
}

export const Route = createLazyFileRoute('/_authenticated/logs')({
  component: ActivityLogsPage,
})
