import { createLazyFileRoute } from '@tanstack/react-router'
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  useReactTable,
  getCoreRowModel,
  flexRender,
  createColumnHelper,
} from '@tanstack/react-table'
import { api, type ActivityLog } from '../../lib/api'
import { 
  AlertCircle, 
  Info, 
  AlertTriangle,
  Clock,
  Calendar,
  Filter,
  X,
  ExternalLink,
  Play,
  Pause,
  CheckCircle,
  XCircle,
  SkipForward
} from 'lucide-react'

const columnHelper = createColumnHelper<ActivityLog>()

function getLevelIcon(level: string) {
  switch (level) {
    case 'error':
      return <AlertCircle className="h-4 w-4 text-red-500" />
    case 'warn':
      return <AlertTriangle className="h-4 w-4 text-yellow-500" />
    case 'info':
    default:
      return <Info className="h-4 w-4 text-blue-500" />
  }
}

function getLevelBadgeClass(level: string) {
  switch (level) {
    case 'error':
      return 'bg-red-100 text-red-800'
    case 'warn':
      return 'bg-yellow-100 text-yellow-800'
    case 'info':
    default:
      return 'bg-blue-100 text-blue-800'
  }
}

function getJobTypeBadgeClass(jobType: string) {
  switch (jobType) {
    case 'analysis':
      return 'bg-purple-100 text-purple-800'
    case 'execution':
      return 'bg-green-100 text-green-800'
    case 'system':
    default:
      return 'bg-gray-100 text-gray-800'
  }
}

function SyncItemModal({ 
  syncItemId, 
  onClose 
}: { 
  syncItemId: string; 
  onClose: () => void 
}) {
  const { data: syncItem, isLoading, error } = useQuery({
    queryKey: ['sync-item', syncItemId],
    queryFn: () => api.getSyncItem(syncItemId),
    enabled: !!syncItemId,
  })

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'pending':
        return <Clock className="h-4 w-4 text-yellow-500" />
      case 'running':
        return <Play className="h-4 w-4 text-blue-500" />
      case 'done':
        return <CheckCircle className="h-4 w-4 text-green-500" />
      case 'error':
        return <XCircle className="h-4 w-4 text-red-500" />
      case 'skipped':
        return <SkipForward className="h-4 w-4 text-orange-500" />
      default:
        return <Pause className="h-4 w-4 text-gray-500" />
    }
  }

  return (
    <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl max-w-2xl w-full mx-4 max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between p-6 border-b">
          <h2 className="text-lg font-semibold text-gray-900">Sync Item Details</h2>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-gray-500 focus:outline-none"
            aria-label="Close modal"
          >
            <X className="h-6 w-6" />
          </button>
        </div>
        
        <div className="p-6">
          {isLoading && (
            <div className="animate-pulse space-y-4">
              <div className="h-4 bg-gray-200 rounded w-3/4"></div>
              <div className="h-4 bg-gray-200 rounded w-1/2"></div>
              <div className="h-4 bg-gray-200 rounded w-2/3"></div>
            </div>
          )}
          
          {error && (
            <div className="bg-red-50 border border-red-200 rounded-md p-4">
              <div className="flex">
                <AlertCircle className="h-5 w-5 text-red-400" />
                <div className="ml-3">
                  <h3 className="text-sm font-medium text-red-800">
                    Error loading sync item
                  </h3>
                  <div className="mt-2 text-sm text-red-700">
                    <p>Unable to fetch sync item details.</p>
                  </div>
                </div>
              </div>
            </div>
          )}
          
          {syncItem && (
            <div className="space-y-6">
              {/* Status */}
              <div>
                <h3 className="text-sm font-medium text-gray-500 mb-2">Status</h3>
                <div className="flex items-center space-x-2">
                  {getStatusIcon(syncItem.status)}
                  <span className="text-sm font-medium capitalize">{syncItem.status}</span>
                </div>
              </div>

              {/* Action & Services */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <h3 className="text-sm font-medium text-gray-500 mb-2">Action</h3>
                  <p className="text-sm text-gray-900 capitalize">{syncItem.action.replace('_', ' ')}</p>
                </div>
                <div>
                  <h3 className="text-sm font-medium text-gray-500 mb-2">Service</h3>
                  <p className="text-sm text-gray-900 capitalize">{syncItem.service}</p>
                </div>
              </div>

              {/* Track Details */}
              {syncItem.source_track_title && (
                <div>
                  <h3 className="text-sm font-medium text-gray-500 mb-2">Track Details</h3>
                  <div className="bg-gray-50 rounded-md p-3 space-y-2">
                    <div>
                      <span className="text-xs font-medium text-gray-600">Title:</span>
                      <p className="text-sm text-gray-900">{syncItem.source_track_title}</p>
                    </div>
                    {syncItem.source_track_id && (
                      <div>
                        <span className="text-xs font-medium text-gray-600">Track ID:</span>
                        <p className="text-sm text-gray-900 font-mono">{syncItem.source_track_id}</p>
                      </div>
                    )}
                    {syncItem.source_service && syncItem.destination_service && (
                      <div>
                        <span className="text-xs font-medium text-gray-600">Direction:</span>
                        <p className="text-sm text-gray-900 capitalize">
                          {syncItem.source_service} → {syncItem.destination_service}
                        </p>
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* Error Details */}
              {syncItem.last_error && (
                <div>
                  <h3 className="text-sm font-medium text-gray-500 mb-2">Last Error</h3>
                  <div className="bg-red-50 border border-red-200 rounded-md p-3">
                    <p className="text-sm text-red-700">{syncItem.last_error}</p>
                  </div>
                </div>
              )}

              {/* Attempts */}
              <div>
                <h3 className="text-sm font-medium text-gray-500 mb-2">Attempts</h3>
                <p className="text-sm text-gray-900">{syncItem.attempts}</p>
              </div>

              {/* Timestamps */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <h3 className="text-sm font-medium text-gray-500 mb-2">Created</h3>
                  <p className="text-sm text-gray-900">{new Date(syncItem.created).toLocaleString()}</p>
                </div>
                <div>
                  <h3 className="text-sm font-medium text-gray-500 mb-2">Updated</h3>
                  <p className="text-sm text-gray-900">{new Date(syncItem.updated).toLocaleString()}</p>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function ActivityLogsPage() {
  const [levelFilter, setLevelFilter] = useState<string>('')
  const [jobTypeFilter, setJobTypeFilter] = useState<string>('')
  const [selectedSyncItemId, setSelectedSyncItemId] = useState<string | null>(null)

  const { data: logsData, isLoading, error, refetch } = useQuery({
    queryKey: ['activity-logs', levelFilter, jobTypeFilter],
    queryFn: () => api.getActivityLogs({
      level: levelFilter || undefined,
      job_type: jobTypeFilter || undefined,
      perPage: 100,
    }),
    refetchInterval: 30000, // Refresh every 30 seconds
  })

  const columns = [
    columnHelper.accessor('created', {
      header: 'Time',
      cell: (info) => {
        const date = new Date(info.getValue())
        return (
          <div className="flex items-center space-x-2">
            <Calendar className="h-4 w-4 text-gray-400" />
            <span className="text-sm text-gray-900">
              {date.toLocaleDateString()} {date.toLocaleTimeString()}
            </span>
          </div>
        )
      },
    }),
    columnHelper.accessor('level', {
      header: 'Level',
      cell: (info) => {
        const level = info.getValue()
        return (
          <div className="flex items-center space-x-2">
            {getLevelIcon(level)}
            <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getLevelBadgeClass(level)}`}>
              {level.toUpperCase()}
            </span>
          </div>
        )
      },
    }),
    columnHelper.accessor('job_type', {
      header: 'Job Type',
      cell: (info) => {
        const jobType = info.getValue()
        return (
          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getJobTypeBadgeClass(jobType)}`}>
            {jobType.toUpperCase()}
          </span>
        )
      },
    }),
    columnHelper.accessor('message', {
      header: 'Message',
      cell: (info) => {
        const message = info.getValue()
        const syncItemId = info.row.original.sync_item_id
        
        if (syncItemId) {
          return (
            <button
              onClick={() => setSelectedSyncItemId(syncItemId)}
              className="text-left hover:text-blue-600 focus:outline-none focus:text-blue-600 flex items-center space-x-1"
            >
              <span className="text-sm text-gray-900">{message}</span>
              <ExternalLink className="h-3 w-3 text-blue-500" />
            </button>
          )
        }
        
        return <span className="text-sm text-gray-900">{message}</span>
      },
    }),
  ]

  const table = useReactTable({
    data: logsData?.items || [],
    columns,
    getCoreRowModel: getCoreRowModel(),
  })

  return (
    <div className="min-h-screen bg-gray-50 py-8 px-4 sm:px-6 lg:px-8">
      <div className="max-w-7xl mx-auto">
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-gray-900">Activity Logs</h1>
          <p className="mt-2 text-gray-600">
            Monitor system activity and sync job execution
          </p>
        </div>

        {/* Filters */}
        <div className="mb-6 flex flex-wrap gap-4">
          <div className="flex items-center space-x-2">
            <Filter className="h-4 w-4 text-gray-500" />
            <select
              value={levelFilter}
              onChange={(e) => setLevelFilter(e.target.value)}
              className="block w-32 pl-3 pr-10 py-2 text-base border-gray-300 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm rounded-md"
            >
              <option value="">All Levels</option>
              <option value="info">Info</option>
              <option value="warn">Warning</option>
              <option value="error">Error</option>
            </select>
          </div>
          
          <div className="flex items-center space-x-2">
            <select
              value={jobTypeFilter}
              onChange={(e) => setJobTypeFilter(e.target.value)}
              className="block w-36 pl-3 pr-10 py-2 text-base border-gray-300 focus:outline-none focus:ring-indigo-500 focus:border-indigo-500 sm:text-sm rounded-md"
            >
              <option value="">All Job Types</option>
              <option value="analysis">Analysis</option>
              <option value="execution">Execution</option>
              <option value="system">System</option>
            </select>
          </div>

          <button
            onClick={() => refetch()}
            className="inline-flex items-center px-3 py-2 border border-transparent text-sm leading-4 font-medium rounded-md text-white bg-indigo-600 hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500"
          >
            Refresh
          </button>
        </div>

        {/* Content */}
        {error && (
          <div className="bg-red-50 border border-red-200 rounded-md p-4 mb-6">
            <div className="flex">
              <AlertCircle className="h-5 w-5 text-red-400" />
              <div className="ml-3">
                <h3 className="text-sm font-medium text-red-800">
                  Error loading activity logs
                </h3>
                <div className="mt-2 text-sm text-red-700">
                  <p>Unable to fetch activity logs. Please try refreshing.</p>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Table */}
        <div className="bg-white shadow overflow-hidden sm:rounded-md">
          {isLoading ? (
            <div className="animate-pulse" role="status" aria-label="Loading activity logs">
              <div className="px-4 py-5 sm:p-6 space-y-4">
                {[...Array(5)].map((_, i) => (
                  <div key={i} className="flex space-x-4">
                    <div className="h-4 bg-gray-200 rounded w-24"></div>
                    <div className="h-4 bg-gray-200 rounded w-16"></div>
                    <div className="h-4 bg-gray-200 rounded w-20"></div>
                    <div className="h-4 bg-gray-200 rounded flex-1"></div>
                  </div>
                ))}
              </div>
            </div>
          ) : (
            <table className="min-w-full divide-y divide-gray-200">
              <thead className="bg-gray-50">
                {table.getHeaderGroups().map((headerGroup) => (
                  <tr key={headerGroup.id}>
                    {headerGroup.headers.map((header) => (
                      <th
                        key={header.id}
                        className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                      >
                        {header.isPlaceholder
                          ? null
                          : flexRender(
                              header.column.columnDef.header,
                              header.getContext()
                            )}
                      </th>
                    ))}
                  </tr>
                ))}
              </thead>
              <tbody className="bg-white divide-y divide-gray-200">
                {table.getRowModel().rows.map((row) => (
                  <tr key={row.id} className="hover:bg-gray-50">
                    {row.getVisibleCells().map((cell) => (
                      <td
                        key={cell.id}
                        className="px-6 py-4 whitespace-nowrap"
                      >
                        {flexRender(
                          cell.column.columnDef.cell,
                          cell.getContext()
                        )}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          )}

          {!isLoading && (!logsData?.items || logsData.items.length === 0) && (
            <div className="text-center py-12">
              <Info className="mx-auto h-12 w-12 text-gray-400" />
              <h3 className="mt-2 text-sm font-medium text-gray-900">No activity logs found</h3>
              <p className="mt-1 text-sm text-gray-500">
                {levelFilter || jobTypeFilter
                  ? 'Try adjusting your filters to see more logs.'
                  : 'Activity logs will appear here when system operations occur.'}
              </p>
            </div>
          )}
        </div>

        {/* Pagination info */}
        {logsData && logsData.totalItems > 0 && (
          <div className="mt-4 text-sm text-gray-700">
            Showing {logsData.items.length} of {logsData.totalItems} activity logs
          </div>
        )}
      </div>

      {/* Modal */}
      {selectedSyncItemId && (
        <SyncItemModal
          syncItemId={selectedSyncItemId}
          onClose={() => setSelectedSyncItemId(null)}
        />
      )}
    </div>
  )
}

export const Route = createLazyFileRoute('/_authenticated/logs')({
  component: ActivityLogsPage,
}) 