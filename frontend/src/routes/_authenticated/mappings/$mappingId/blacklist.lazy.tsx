import { createLazyFileRoute, useParams } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../../../lib/api'
import { Trash2, ArrowLeft } from 'lucide-react'
import { Link } from '@tanstack/react-router'

function BlacklistPage() {
  const { mappingId } = useParams({ strict: false })
  const queryClient = useQueryClient()

  const { data: blacklistData, isLoading, error } = useQuery({
    queryKey: ['blacklist', mappingId],
    queryFn: () => api.getBlacklist(mappingId),
    enabled: !!mappingId,
  })

  const { data: mappingData } = useQuery({
    queryKey: ['mapping', mappingId],
    queryFn: () => api.getMapping(mappingId!),
    enabled: !!mappingId,
  })

  const deleteBlacklistMutation = useMutation({
    mutationFn: api.deleteBlacklistEntry,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['blacklist', mappingId] })
    },
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-gray-900"></div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-red-600">Error loading blacklist: {error.message}</div>
      </div>
    )
  }

  const blacklistEntries = blacklistData?.items || []

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  const getServiceBadgeColor = (service: string) => {
    return service === 'spotify' 
      ? 'bg-green-100 text-green-800' 
      : 'bg-red-100 text-red-800'
  }

  const getReasonBadgeColor = (reason: string) => {
    const colors = {
      'not_found': 'bg-gray-100 text-gray-800',
      'forbidden': 'bg-yellow-100 text-yellow-800',
      'unauthorized': 'bg-orange-100 text-orange-800',
      'invalid': 'bg-purple-100 text-purple-800',
      'error': 'bg-red-100 text-red-800',
    }
    return colors[reason as keyof typeof colors] || 'bg-gray-100 text-gray-800'
  }

  return (
    <div className="min-h-screen bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-7xl mx-auto">
        <div className="mb-8">
          <Link
            to="/mappings/$mappingId/edit"
            params={{ mappingId: mappingId! }}
            className="inline-flex items-center text-sm text-gray-500 hover:text-gray-700"
          >
            <ArrowLeft className="h-4 w-4 mr-1" />
            Back to mapping
          </Link>
        </div>

        <div className="sm:flex sm:items-center">
          <div className="sm:flex-auto">
            <h1 className="text-2xl font-semibold text-gray-900">Blacklisted Tracks</h1>
            <p className="mt-2 text-sm text-gray-700">
              Tracks that failed to sync for mapping:{' '}
              <span className="font-medium">
                {mappingData?.spotify_playlist_name || mappingData?.spotify_playlist_id} ↔{' '}
                {mappingData?.youtube_playlist_name || mappingData?.youtube_playlist_id}
              </span>
            </p>
            <p className="mt-1 text-xs text-gray-500">
              Remove tracks from the blacklist to allow them to be retried in future sync attempts.
            </p>
          </div>
        </div>

        {blacklistEntries.length === 0 ? (
          <div className="mt-8 text-center">
            <div className="bg-white rounded-lg shadow px-6 py-8">
              <div className="text-gray-400 mb-4">
                <svg className="mx-auto h-12 w-12" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </div>
              <h3 className="text-lg font-medium text-gray-900 mb-2">No blacklisted tracks</h3>
              <p className="text-gray-500">All tracks in this mapping are syncing successfully.</p>
            </div>
          </div>
        ) : (
          <div className="mt-8 flex flex-col">
            <div className="-my-2 -mx-4 overflow-x-auto sm:-mx-6 lg:-mx-8">
              <div className="inline-block min-w-full py-2 align-middle md:px-6 lg:px-8">
                <div className="overflow-hidden shadow ring-1 ring-black ring-opacity-5 md:rounded-lg">
                  <table className="min-w-full divide-y divide-gray-300">
                    <thead className="bg-gray-50">
                      <tr>
                        <th className="px-3 py-3.5 text-left text-sm font-semibold text-gray-900">
                          Service
                        </th>
                        <th className="px-3 py-3.5 text-left text-sm font-semibold text-gray-900">
                          Track ID
                        </th>
                        <th className="px-3 py-3.5 text-left text-sm font-semibold text-gray-900">
                          Reason
                        </th>
                        <th className="px-3 py-3.5 text-left text-sm font-semibold text-gray-900">
                          Skip Count
                        </th>
                        <th className="px-3 py-3.5 text-left text-sm font-semibold text-gray-900">
                          Last Skipped
                        </th>
                        <th className="relative py-3.5 pl-3 pr-4 sm:pr-6">
                          <span className="sr-only">Actions</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-200 bg-white">
                      {blacklistEntries.map((entry) => (
                        <tr key={entry.id}>
                          <td className="whitespace-nowrap px-3 py-4 text-sm">
                            <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getServiceBadgeColor(entry.service)}`}>
                              {entry.service === 'spotify' ? 'Spotify' : 'YouTube'}
                            </span>
                          </td>
                          <td className="px-3 py-4 text-sm text-gray-900 font-mono">
                            <div className="max-w-xs truncate">
                              {entry.track_id}
                            </div>
                          </td>
                          <td className="whitespace-nowrap px-3 py-4 text-sm">
                            <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${getReasonBadgeColor(entry.reason)}`}>
                              {entry.reason.replace('_', ' ')}
                            </span>
                          </td>
                          <td className="whitespace-nowrap px-3 py-4 text-sm text-gray-500">
                            {entry.skip_counter}
                          </td>
                          <td className="whitespace-nowrap px-3 py-4 text-sm text-gray-500">
                            {formatDate(entry.last_skipped_at)}
                          </td>
                          <td className="relative whitespace-nowrap py-4 pl-3 pr-4 text-right text-sm font-medium sm:pr-6">
                            <button
                              onClick={() => {
                                if (confirm('Remove this track from the blacklist? It will be retried in future sync attempts.')) {
                                  deleteBlacklistMutation.mutate(entry.id)
                                }
                              }}
                              className="text-indigo-600 hover:text-indigo-900"
                              title="Remove from blacklist"
                            >
                              <Trash2 className="h-4 w-4 inline" />
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

export const Route = createLazyFileRoute('/_authenticated/mappings/$mappingId/blacklist')({
  component: BlacklistPage,
}) 