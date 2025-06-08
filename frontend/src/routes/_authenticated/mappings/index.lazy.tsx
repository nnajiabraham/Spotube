import { createLazyFileRoute, Link } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../../lib/api'
import { Trash2, Edit } from 'lucide-react'

function MappingsList() {
  const queryClient = useQueryClient()

  const { data, isLoading, error } = useQuery({
    queryKey: ['mappings'],
    queryFn: () => api.getMappings(),
  })

  const deleteMutation = useMutation({
    mutationFn: api.deleteMapping,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['mappings'] })
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
        <div className="text-red-600">Error loading mappings: {error.message}</div>
      </div>
    )
  }

  const mappings = data?.items || []

  return (
    <div className="min-h-screen bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-7xl mx-auto">
        <div className="sm:flex sm:items-center">
          <div className="sm:flex-auto">
            <h1 className="text-2xl font-semibold text-gray-900">Playlist Mappings</h1>
            <p className="mt-2 text-sm text-gray-700">
              Manage your synchronized playlists between Spotify and YouTube
            </p>
          </div>
          <div className="mt-4 sm:mt-0 sm:ml-16 sm:flex-none">
            <Link
              to="/mappings/new"
              className="inline-flex items-center justify-center rounded-md border border-transparent bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-2 sm:w-auto"
            >
              Add mapping
            </Link>
          </div>
        </div>

        {mappings.length === 0 ? (
          <div className="mt-8 text-center">
            <p className="text-gray-500">No mappings yet. Create your first mapping to start syncing playlists.</p>
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
                          Spotify Playlist
                        </th>
                        <th className="px-3 py-3.5 text-left text-sm font-semibold text-gray-900">
                          YouTube Playlist
                        </th>
                        <th className="px-3 py-3.5 text-left text-sm font-semibold text-gray-900">
                          Sync Options
                        </th>
                        <th className="px-3 py-3.5 text-left text-sm font-semibold text-gray-900">
                          Interval
                        </th>
                        <th className="relative py-3.5 pl-3 pr-4 sm:pr-6">
                          <span className="sr-only">Actions</span>
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-gray-200 bg-white">
                      {mappings.map((mapping) => (
                        <tr key={mapping.id}>
                          <td className="whitespace-nowrap px-3 py-4 text-sm text-gray-900">
                            {mapping.spotify_playlist_name || mapping.spotify_playlist_id}
                          </td>
                          <td className="whitespace-nowrap px-3 py-4 text-sm text-gray-900">
                            {mapping.youtube_playlist_name || mapping.youtube_playlist_id}
                          </td>
                          <td className="whitespace-nowrap px-3 py-4 text-sm text-gray-500">
                            <div className="flex flex-col">
                              {mapping.sync_name && <span>✓ Name</span>}
                              {mapping.sync_tracks && <span>✓ Tracks</span>}
                            </div>
                          </td>
                          <td className="whitespace-nowrap px-3 py-4 text-sm text-gray-500">
                            {mapping.interval_minutes} min
                          </td>
                          <td className="relative whitespace-nowrap py-4 pl-3 pr-4 text-right text-sm font-medium sm:pr-6">
                            <Link
                              to="/mappings/$mappingId/edit"
                              params={{ mappingId: mapping.id }}
                              className="text-indigo-600 hover:text-indigo-900 mr-4"
                            >
                              <Edit className="h-4 w-4 inline" />
                            </Link>
                            <button
                              onClick={() => {
                                if (confirm('Are you sure you want to delete this mapping?')) {
                                  deleteMutation.mutate(mapping.id)
                                }
                              }}
                              className="text-red-600 hover:text-red-900"
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

export const Route = createLazyFileRoute('/_authenticated/mappings/')({
  component: MappingsList,
}) 