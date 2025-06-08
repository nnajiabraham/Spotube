import { createLazyFileRoute, useNavigate, useParams } from '@tanstack/react-router'
import { useQuery, useMutation } from '@tanstack/react-query'
import { api } from '../../../../lib/api'
import type { Mapping } from '../../../../lib/pocketbase'

function EditMapping() {
  const { mappingId } = useParams({ from: '/_authenticated/mappings/$mappingId/edit' })
  const navigate = useNavigate()

  const { data: mapping, isLoading } = useQuery({
    queryKey: ['mapping', mappingId],
    queryFn: () => api.getMapping(mappingId),
  })

  const updateMutation = useMutation({
    mutationFn: (data: Partial<Mapping>) => api.updateMapping(mappingId, data),
    onSuccess: () => {
      navigate({ to: '/mappings' })
    },
  })

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-gray-900"></div>
      </div>
    )
  }

  if (!mapping) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-red-600">Mapping not found</div>
      </div>
    )
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    const formData = new FormData(e.target as HTMLFormElement)
    updateMutation.mutate({
      sync_name: formData.get('sync_name') === 'on',
      sync_tracks: formData.get('sync_tracks') === 'on',
      interval_minutes: Number(formData.get('interval_minutes')),
    })
  }

  return (
    <div className="min-h-screen bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-3xl mx-auto">
        <h1 className="text-2xl font-semibold text-gray-900 mb-8">Edit Mapping</h1>

        <div className="bg-white shadow rounded-lg p-6">
          <form onSubmit={handleSubmit}>
            <div className="space-y-6">
              {/* Display read-only playlist info */}
              <div>
                <h3 className="text-lg font-medium mb-2">Spotify Playlist</h3>
                <p className="text-gray-600">
                  {mapping.spotify_playlist_name || mapping.spotify_playlist_id}
                </p>
              </div>

              <div>
                <h3 className="text-lg font-medium mb-2">YouTube Playlist</h3>
                <p className="text-gray-600">
                  {mapping.youtube_playlist_name || mapping.youtube_playlist_id}
                </p>
              </div>

              {/* Editable sync options */}
              <div>
                <label className="flex items-center">
                  <input
                    type="checkbox"
                    name="sync_name"
                    defaultChecked={mapping.sync_name}
                    className="mr-3"
                  />
                  <div>
                    <div className="font-medium">Sync playlist name</div>
                    <div className="text-sm text-gray-500">
                      Keep playlist names synchronized between platforms
                    </div>
                  </div>
                </label>
              </div>

              <div>
                <label className="flex items-center">
                  <input
                    type="checkbox"
                    name="sync_tracks"
                    defaultChecked={mapping.sync_tracks}
                    className="mr-3"
                  />
                  <div>
                    <div className="font-medium">Sync tracks</div>
                    <div className="text-sm text-gray-500">
                      Keep track lists synchronized between platforms
                    </div>
                  </div>
                </label>
              </div>

              <div>
                <label className="block">
                  <span className="font-medium">Sync interval (minutes)</span>
                  <input
                    type="range"
                    name="interval_minutes"
                    min="5"
                    max="720"
                    step="5"
                    defaultValue={mapping.interval_minutes}
                    className="mt-2 w-full"
                  />
                  <div className="flex justify-between text-sm text-gray-500 mt-1">
                    <span>5 min</span>
                    <span className="font-medium">{mapping.interval_minutes} minutes</span>
                    <span>720 min</span>
                  </div>
                </label>
              </div>
            </div>

            <div className="mt-8 flex justify-between">
              <button
                type="button"
                onClick={() => navigate({ to: '/mappings' })}
                className="px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={updateMutation.isPending}
                className="px-4 py-2 bg-indigo-600 text-white rounded-md hover:bg-indigo-700 disabled:bg-gray-400"
              >
                {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  )
}

export const Route = createLazyFileRoute('/_authenticated/mappings/$mappingId/edit')({
  component: EditMapping,
}) 