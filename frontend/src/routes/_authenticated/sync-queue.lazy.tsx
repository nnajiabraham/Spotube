import { createLazyFileRoute, Link } from '@tanstack/react-router'
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
  type ColumnSizingState,
} from '@tanstack/react-table'
import {
  syncItemsAPI,
  isSyncItemExecutable,
  type SyncItemDetails,
  type SyncItemOperation,
  type SyncItemService,
  type SyncItemStatus,
} from '../../lib/api/sync-items'
import { activityLogsAPI } from '../../lib/api/activity-logs'
import { Play, ExternalLink, Loader2, Eye, X } from 'lucide-react'

const columnHelper = createColumnHelper<SyncItemDetails>()

const defaultColumnSizing: ColumnSizingState = {
  status: 90,
  operation: 80,
  route: 120,
  track: 200,
  actions: 140,
  playlists: 220,
  mapping_id: 100,
  attempt_count: 70,
  updated: 140,
}

function WrapCell({ children, className = '' }: { children: React.ReactNode; className?: string }) {
  return (
    <div className={`max-h-24 overflow-y-auto break-words whitespace-normal text-sm ${className}`}>
      {children}
    </div>
  )
}

function statusBadgeClass(status: SyncItemStatus): string {
  switch (status) {
    case 'pending':
      return 'bg-yellow-100 text-yellow-800'
    case 'running':
      return 'bg-blue-100 text-blue-800'
    case 'done':
      return 'bg-green-100 text-green-800'
    case 'error':
      return 'bg-red-100 text-red-800'
    case 'skipped':
      return 'bg-orange-100 text-orange-800'
    default:
      return 'bg-gray-100 text-gray-800'
  }
}

function SyncItemDetailModal({
  itemId,
  onClose,
}: {
  itemId: string
  onClose: () => void
}) {
  const { data: item, isLoading, error } = useQuery({
    queryKey: ['sync-item', itemId],
    queryFn: () => syncItemsAPI.getOne(itemId),
    enabled: !!itemId,
  })

  const { data: executorLogs } = useQuery({
    queryKey: ['executor-logs', itemId],
    queryFn: () =>
      activityLogsAPI.getList({
        sync_item_id: itemId,
        job_type: 'executor',
        per_page: 10,
      }),
    enabled: !!itemId,
  })

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div
        className="flex max-h-[90vh] w-full max-w-2xl flex-col overflow-hidden rounded-lg bg-white shadow-xl"
        role="dialog"
        aria-labelledby="sync-item-detail-title"
      >
        <div className="flex items-center justify-between border-b px-6 py-4">
          <h2 id="sync-item-detail-title" className="text-lg font-semibold text-gray-900">
            Sync item details
          </h2>
          <button
            type="button"
            onClick={onClose}
            className="text-gray-400 hover:text-gray-600"
            aria-label="Close"
          >
            <X className="h-6 w-6" />
          </button>
        </div>

        <div className="overflow-y-auto px-6 py-4">
          {isLoading && <p className="text-sm text-gray-500">Loading…</p>}
          {error && (
            <p className="text-sm text-red-600">Failed to load sync item: {error.message}</p>
          )}
          {item && (
            <div className="space-y-4 text-sm">
              <DetailRow label="ID" value={item.id} mono />
              <DetailRow label="Status" value={item.status} />
              <DetailRow label="Operation" value={item.operation} />
              <DetailRow
                label="Route"
                value={`${item.source_service} → ${item.destination_service}`}
              />
              <DetailRow
                label="Playlists"
                value={`${item.source_playlist_name || '—'} → ${item.destination_playlist_name || '—'}`}
              />
              <DetailRow
                label="Track / target"
                value={
                  item.track_artist
                    ? `${item.track_title || '—'} — ${item.track_artist}`
                    : item.track_title || '—'
                }
              />
              <DetailRow label="Mapping" value={item.mapping_id} mono />
              <DetailRow label="Attempts" value={String(item.attempt_count)} />
              {item.error_message ? (
                <DetailRow label="Error" value={item.error_message} error />
              ) : null}
              <DetailRow
                label="Updated"
                value={new Date(item.updated * 1000).toLocaleString()}
              />

              {item.analysis_context_json ? (
                <div className="border-t pt-4">
                  <h3 className="mb-2 font-medium text-gray-900">Analysis context</h3>
                  <pre className="max-h-40 overflow-auto rounded-md bg-gray-50 p-3 text-xs text-gray-800">
                    {JSON.stringify(item.analysis_context_json, null, 2)}
                  </pre>
                </div>
              ) : null}

              <div className="border-t pt-4">
                <h3 className="mb-2 font-medium text-gray-900">Executor logs for this item</h3>
                {executorLogs?.items?.length ? (
                  <ul className="max-h-40 space-y-2 overflow-y-auto rounded-md bg-gray-50 p-3 text-xs">
                    {executorLogs.items.map((log) => (
                      <li key={log.id} className="text-gray-700">
                        <span className="text-gray-500">
                          {new Date(log.created * 1000).toLocaleString()} —
                        </span>{' '}
                        {log.message}
                        {log.details_json ? (
                          <pre className="mt-1 max-h-32 overflow-auto rounded bg-white p-2 text-[10px] text-gray-600">
                            {JSON.stringify(log.details_json, null, 2)}
                          </pre>
                        ) : null}
                      </li>
                    ))}
                  </ul>
                ) : (
                  <p className="text-xs text-gray-500">
                    No executor logs for this mapping yet. Run Execute to generate entries.
                  </p>
                )}
                <Link
                  to="/logs"
                  className="mt-2 inline-block text-xs text-indigo-600 hover:text-indigo-800"
                >
                  Open full activity logs →
                </Link>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function DetailRow({
  label,
  value,
  mono,
  error: isError,
}: {
  label: string
  value: string
  mono?: boolean
  error?: boolean
}) {
  return (
    <div>
      <dt className="text-xs font-medium text-gray-500">{label}</dt>
      <dd
        className={`mt-0.5 max-h-32 overflow-y-auto break-words whitespace-normal ${mono ? 'font-mono text-xs' : ''} ${isError ? 'text-red-700' : 'text-gray-900'}`}
      >
        {value}
      </dd>
    </div>
  )
}

function SyncQueuePage() {
  const queryClient = useQueryClient()
  const [statusFilter, setStatusFilter] = useState<SyncItemStatus | ''>('')
  const [serviceFilter, setServiceFilter] = useState<SyncItemService | ''>('')
  const [operationFilter, setOperationFilter] = useState<SyncItemOperation | ''>('')
  const [executingId, setExecutingId] = useState<string | null>(null)
  const [detailItemId, setDetailItemId] = useState<string | null>(null)
  const [columnSizing, setColumnSizing] = useState<ColumnSizingState>(defaultColumnSizing)
  const [lastMessage, setLastMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(
    null,
  )

  const { data, isLoading, error } = useQuery({
    queryKey: ['sync-items', statusFilter, serviceFilter, operationFilter],
    queryFn: () =>
      syncItemsAPI.getList({
        per_page: 50,
        status: statusFilter || undefined,
        service: serviceFilter || undefined,
        operation: operationFilter || undefined,
        order: 'desc',
      }),
    // Analysis may refresh rename rows in place; always load current IDs when opening the page.
    refetchOnMount: 'always',
  })

  const executeMutation = useMutation({
    mutationFn: (id: string) => syncItemsAPI.execute(id),
    onMutate: (id) => {
      setExecutingId(id)
      setLastMessage(null)
    },
    onSuccess: (result) => {
      if (result.status === 'done') {
        setLastMessage({ type: 'success', text: `Item ${result.id.slice(0, 8)}… completed successfully.` })
      } else if (result.status === 'skipped') {
        setLastMessage({
          type: 'error',
          text: result.error_message || 'Track was skipped (blacklisted).',
        })
      } else {
        setLastMessage({
          type: 'error',
          text: result.execution_log || result.error_message || 'Execution failed.',
        })
      }
      queryClient.invalidateQueries({ queryKey: ['sync-items'] })
      queryClient.invalidateQueries({ queryKey: ['executor-logs', result.id] })
    },
    onError: (err: Error) => {
      setLastMessage({ type: 'error', text: err.message })
    },
    onSettled: () => {
      setExecutingId(null)
    },
  })

  const columns = [
    columnHelper.accessor('status', {
      header: 'Status',
      size: 90,
      enableResizing: false,
      cell: (info) => (
        <span
          className={`inline-flex rounded-full px-2 py-1 text-xs font-semibold ${statusBadgeClass(info.getValue())}`}
        >
          {info.getValue()}
        </span>
      ),
    }),
    columnHelper.accessor('operation', {
      header: 'Operation',
      cell: (info) => <span className="text-sm text-gray-900">{info.getValue()}</span>,
    }),
    columnHelper.display({
      id: 'route',
      header: 'Source → Dest',
      cell: ({ row }) => (
        <span className="text-sm capitalize text-gray-900">
          {row.original.source_service} → {row.original.destination_service}
        </span>
      ),
    }),
    columnHelper.display({
      id: 'track',
      header: 'Track / target',
      size: 200,
      enableResizing: true,
      cell: ({ row }) => {
        const title = row.original.track_title || '—'
        const artist = row.original.track_artist
        return (
          <WrapCell>
            <div className="text-gray-900">{title}</div>
            {artist ? <div className="text-xs text-gray-500">{artist}</div> : null}
          </WrapCell>
        )
      },
    }),
    columnHelper.display({
      id: 'actions',
      header: 'Actions',
      size: 140,
      enableResizing: false,
      cell: ({ row }) => {
        const item = row.original
        const canExecute = isSyncItemExecutable(item.status)
        const isRunning = executingId === item.id

        return (
          <div className="flex flex-shrink-0 items-center gap-2">
            <button
              type="button"
              disabled={!canExecute || isRunning || executeMutation.isPending}
              onClick={() => executeMutation.mutate(item.id)}
              className="inline-flex items-center gap-1 rounded-md bg-indigo-600 px-2.5 py-1.5 text-xs font-medium text-white shadow-sm hover:bg-indigo-700 disabled:cursor-not-allowed disabled:bg-gray-300"
            >
              {isRunning ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden />
              ) : (
                <Play className="h-3.5 w-3.5" aria-hidden />
              )}
              Execute
            </button>
            <button
              type="button"
              onClick={() => setDetailItemId(item.id)}
              className="inline-flex items-center gap-1 rounded-md border border-gray-300 bg-white px-2.5 py-1.5 text-xs font-medium text-gray-700 shadow-sm hover:bg-gray-50"
            >
              <Eye className="h-3.5 w-3.5" aria-hidden />
              View
            </button>
          </div>
        )
      },
    }),
    columnHelper.display({
      id: 'playlists',
      header: 'Playlists',
      size: 220,
      enableResizing: true,
      cell: ({ row }) => (
        <WrapCell className="text-xs text-gray-600">
          <div>{row.original.source_playlist_name || '—'}</div>
          <div className="text-gray-400">→ {row.original.destination_playlist_name || '—'}</div>
        </WrapCell>
      ),
    }),
    columnHelper.accessor('mapping_id', {
      header: 'Mapping',
      cell: (info) => (
        <Link
          to="/mappings/$mappingId/edit"
          params={{ mappingId: info.getValue() }}
          className="inline-flex items-center gap-1 text-sm text-indigo-600 hover:text-indigo-900"
        >
          {info.getValue().slice(0, 8)}…
          <ExternalLink className="h-3 w-3" />
        </Link>
      ),
    }),
    columnHelper.accessor('attempt_count', {
      header: 'Attempts',
      cell: (info) => <span className="text-sm text-gray-700">{info.getValue()}</span>,
    }),
    columnHelper.accessor('updated', {
      header: 'Updated',
      cell: (info) => (
        <span className="whitespace-nowrap text-xs text-gray-500">
          {new Date(info.getValue() * 1000).toLocaleString()}
        </span>
      ),
    }),
  ]

  const table = useReactTable({
    data: data?.items ?? [],
    columns,
    getCoreRowModel: getCoreRowModel(),
    columnResizeMode: 'onChange',
    enableColumnResizing: true,
    state: { columnSizing },
    onColumnSizingChange: setColumnSizing,
  })

  return (
    <div className="min-h-screen bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
      <div className="mx-auto max-w-7xl">
        <div className="sm:flex sm:items-center sm:justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-gray-900">Sync Queue</h1>
            <p className="mt-2 text-sm text-gray-700">
              Review pending sync work and run items one at a time against Spotify or YouTube.
            </p>
          </div>
          <Link to="/logs" className="text-sm text-indigo-600 hover:text-indigo-800">
            View activity logs →
          </Link>
        </div>

        {lastMessage && (
          <div
            className={`mt-4 rounded-md border p-4 text-sm ${
              lastMessage.type === 'success'
                ? 'border-green-200 bg-green-50 text-green-800'
                : 'border-red-200 bg-red-50 text-red-800'
            }`}
            role="alert"
          >
            {lastMessage.text}
          </div>
        )}

        <div className="mt-6 flex flex-wrap gap-4">
          <label className="text-sm text-gray-700">
            Status
            <select
              className="ml-2 rounded-md border-gray-300 text-sm"
              value={statusFilter}
              onChange={(e) => setStatusFilter(e.target.value as SyncItemStatus | '')}
            >
              <option value="">All</option>
              <option value="pending">Pending</option>
              <option value="running">Running</option>
              <option value="done">Done</option>
              <option value="error">Error</option>
              <option value="skipped">Skipped</option>
            </select>
          </label>
          <label className="text-sm text-gray-700">
            Destination
            <select
              className="ml-2 rounded-md border-gray-300 text-sm"
              value={serviceFilter}
              onChange={(e) => setServiceFilter(e.target.value as SyncItemService | '')}
            >
              <option value="">All</option>
              <option value="spotify">Spotify</option>
              <option value="youtube">YouTube</option>
            </select>
          </label>
          <label className="text-sm text-gray-700">
            Operation
            <select
              className="ml-2 rounded-md border-gray-300 text-sm"
              value={operationFilter}
              onChange={(e) => setOperationFilter(e.target.value as SyncItemOperation | '')}
            >
              <option value="">All</option>
              <option value="add">Add</option>
              <option value="rename">Rename</option>
              <option value="remove">Remove</option>
            </select>
          </label>
        </div>

        {error && (
          <div className="mt-6 rounded-md bg-red-50 p-4 text-sm text-red-800">
            Error loading sync queue: {error.message}
          </div>
        )}

        <p className="mt-4 text-xs text-gray-500">
          Scroll horizontally if needed — Execute and View are in the Actions column. After analysis
          runs, refresh this page if Execute returns &quot;not found&quot; for a visible row.
        </p>

        <div className="mt-2 overflow-x-auto rounded-lg shadow ring-1 ring-black/5">
          {isLoading ? (
            <div className="bg-white p-8 text-center text-gray-500">Loading sync items…</div>
          ) : (
            <table className="w-full table-fixed divide-y divide-gray-300 bg-white" style={{ width: table.getCenterTotalSize() }}>
              <thead className="bg-gray-50">
                {table.getHeaderGroups().map((headerGroup) => (
                  <tr key={headerGroup.id}>
                    {headerGroup.headers.map((header) => (
                      <th
                        key={header.id}
                        style={{ width: header.getSize() }}
                        className={`relative px-3 py-3.5 text-left text-xs font-semibold text-gray-900 ${
                          header.column.id === 'actions' ? 'sticky right-0 bg-gray-50 shadow-[-4px_0_8px_-4px_rgba(0,0,0,0.1)]' : ''
                        } ${header.column.id === 'mapping_id' || header.column.id === 'updated' ? 'hidden lg:table-cell' : ''}`}
                      >
                        {header.isPlaceholder
                          ? null
                          : flexRender(header.column.columnDef.header, header.getContext())}
                        {header.column.getCanResize() ? (
                          <button
                            type="button"
                            aria-label={`Resize ${header.column.id} column`}
                            onMouseDown={header.getResizeHandler()}
                            onTouchStart={header.getResizeHandler()}
                            className="absolute right-0 top-0 h-full w-1.5 cursor-col-resize select-none touch-none bg-transparent hover:bg-indigo-200"
                          />
                        ) : null}
                      </th>
                    ))}
                  </tr>
                ))}
              </thead>
              <tbody className="divide-y divide-gray-200">
                {table.getRowModel().rows.length === 0 ? (
                  <tr>
                    <td colSpan={columns.length} className="px-3 py-8 text-center text-sm text-gray-500">
                      No sync items found. Enable analysis workers and wait for a mapping diff, or
                      adjust filters.
                    </td>
                  </tr>
                ) : (
                  table.getRowModel().rows.map((row) => (
                    <tr key={row.id} className="group hover:bg-gray-50">
                      {row.getVisibleCells().map((cell) => (
                        <td
                          key={cell.id}
                          style={{ width: cell.column.getSize() }}
                          className={`px-3 py-4 align-top ${
                            cell.column.id === 'actions'
                              ? 'sticky right-0 bg-white group-hover:bg-gray-50 shadow-[-4px_0_8px_-4px_rgba(0,0,0,0.08)]'
                              : ''
                          } ${cell.column.id === 'mapping_id' || cell.column.id === 'updated' ? 'hidden lg:table-cell' : ''}`}
                        >
                          {flexRender(cell.column.columnDef.cell, cell.getContext())}
                        </td>
                      ))}
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          )}
        </div>

        {data && data.totalItems > 0 && (
          <p className="mt-4 text-sm text-gray-500">
            Showing {data.items.length} of {data.totalItems} items
          </p>
        )}
      </div>

      {detailItemId && (
        <SyncItemDetailModal itemId={detailItemId} onClose={() => setDetailItemId(null)} />
      )}
    </div>
  )
}

export const Route = createLazyFileRoute('/_authenticated/sync-queue')({
  component: SyncQueuePage,
})
