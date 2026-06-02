import { createLazyFileRoute, Link } from '@tanstack/react-router'
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  createColumnHelper,
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import {
  syncItemsAPI,
  isSyncItemExecutable,
  type SyncItemDetails,
  type SyncItemOperation,
  type SyncItemService,
  type SyncItemStatus,
} from '../../lib/api/sync-items'
import { Play, ExternalLink, Loader2 } from 'lucide-react'

const columnHelper = createColumnHelper<SyncItemDetails>()

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

function SyncQueuePage() {
  const queryClient = useQueryClient()
  const [statusFilter, setStatusFilter] = useState<SyncItemStatus | ''>('')
  const [serviceFilter, setServiceFilter] = useState<SyncItemService | ''>('')
  const [operationFilter, setOperationFilter] = useState<SyncItemOperation | ''>('')
  const [executingId, setExecutingId] = useState<string | null>(null)
  const [lastMessage, setLastMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

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
  })

  const executeMutation = useMutation({
    mutationFn: (id: string) => syncItemsAPI.execute(id),
    onMutate: (id) => {
      setExecutingId(id)
      setLastMessage(null)
    },
    onSuccess: (result) => {
      if (result.status === 'done') {
        setLastMessage({ type: 'success', text: `Item ${result.id} completed successfully.` })
      } else if (result.status === 'skipped') {
        setLastMessage({ type: 'error', text: result.error_message || 'Track was skipped (blacklisted).' })
      } else {
        setLastMessage({
          type: 'error',
          text: result.execution_log || result.error_message || 'Execution failed.',
        })
      }
      queryClient.invalidateQueries({ queryKey: ['sync-items'] })
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
      cell: (info) => (
        <span className={`inline-flex rounded-full px-2 py-1 text-xs font-semibold ${statusBadgeClass(info.getValue())}`}>
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
        <span className="text-sm text-gray-900 capitalize">
          {row.original.source_service} → {row.original.destination_service}
        </span>
      ),
    }),
    columnHelper.display({
      id: 'playlists',
      header: 'Playlists',
      cell: ({ row }) => (
        <div className="text-xs text-gray-600 max-w-xs">
          <div>{row.original.source_playlist_name || '—'}</div>
          <div className="text-gray-400">→ {row.original.destination_playlist_name || '—'}</div>
        </div>
      ),
    }),
    columnHelper.display({
      id: 'track',
      header: 'Track / target',
      cell: ({ row }) => {
        const title = row.original.track_title || '—'
        const artist = row.original.track_artist
        return (
          <div className="text-sm text-gray-900">
            <div>{title}</div>
            {artist ? <div className="text-xs text-gray-500">{artist}</div> : null}
          </div>
        )
      },
    }),
    columnHelper.accessor('mapping_id', {
      header: 'Mapping',
      cell: (info) => (
        <Link
          to="/mappings/$mappingId/edit"
          params={{ mappingId: info.getValue() }}
          className="text-indigo-600 hover:text-indigo-900 text-sm inline-flex items-center gap-1"
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
        <span className="text-xs text-gray-500">
          {new Date(info.getValue() * 1000).toLocaleString()}
        </span>
      ),
    }),
    columnHelper.display({
      id: 'actions',
      header: 'Actions',
      cell: ({ row }) => {
        const item = row.original
        const canExecute = isSyncItemExecutable(item.status)
        const isRunning = executingId === item.id

        return (
          <button
            type="button"
            disabled={!canExecute || isRunning || executeMutation.isPending}
            onClick={() => executeMutation.mutate(item.id)}
            className="inline-flex items-center gap-1 rounded-md border border-transparent bg-indigo-600 px-2 py-1 text-xs font-medium text-white shadow-sm hover:bg-indigo-700 disabled:cursor-not-allowed disabled:bg-gray-300"
          >
            {isRunning ? <Loader2 className="h-3 w-3 animate-spin" /> : <Play className="h-3 w-3" />}
            Execute
          </button>
        )
      },
    }),
  ]

  const table = useReactTable({
    data: data?.items ?? [],
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  return (
    <div className="min-h-screen bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-7xl mx-auto">
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
            className={`mt-4 rounded-md p-4 text-sm ${
              lastMessage.type === 'success'
                ? 'bg-green-50 text-green-800 border border-green-200'
                : 'bg-red-50 text-red-800 border border-red-200'
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
          <div className="mt-6 rounded-md bg-red-50 p-4 text-red-800 text-sm">
            Error loading sync queue: {error.message}
          </div>
        )}

        <div className="mt-8 overflow-hidden shadow ring-1 ring-black ring-opacity-5 md:rounded-lg">
          {isLoading ? (
            <div className="p-8 animate-pulse text-center text-gray-500">Loading sync items…</div>
          ) : (
            <table className="min-w-full divide-y divide-gray-300">
              <thead className="bg-gray-50">
                {table.getHeaderGroups().map((headerGroup) => (
                  <tr key={headerGroup.id}>
                    {headerGroup.headers.map((header) => (
                      <th
                        key={header.id}
                        className="px-3 py-3.5 text-left text-xs font-semibold text-gray-900"
                      >
                        {header.isPlaceholder
                          ? null
                          : flexRender(header.column.columnDef.header, header.getContext())}
                      </th>
                    ))}
                  </tr>
                ))}
              </thead>
              <tbody className="divide-y divide-gray-200 bg-white">
                {table.getRowModel().rows.length === 0 ? (
                  <tr>
                    <td colSpan={columns.length} className="px-3 py-8 text-center text-sm text-gray-500">
                      No sync items found. Enable analysis workers and wait for a mapping diff, or adjust filters.
                    </td>
                  </tr>
                ) : (
                  table.getRowModel().rows.map((row) => (
                    <tr key={row.id}>
                      {row.getVisibleCells().map((cell) => (
                        <td key={cell.id} className="whitespace-nowrap px-3 py-4">
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
    </div>
  )
}

export const Route = createLazyFileRoute('/_authenticated/sync-queue')({
  component: SyncQueuePage,
})
