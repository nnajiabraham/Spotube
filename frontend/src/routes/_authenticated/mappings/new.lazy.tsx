import { createLazyFileRoute, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { api } from '../../../lib/api'
import type { SpotifyPlaylist, YouTubePlaylist } from '../../../lib/pocketbase'

function NewMapping() {
  const navigate = useNavigate()
  const [step, setStep] = useState(1)
  const [formData, setFormData] = useState({
    spotify_playlist_id: '',
    youtube_playlist_id: '',
    sync_name: true,
    sync_tracks: true,
    interval_minutes: 60,
  })

  // Fetch playlists
  const { data: spotifyPlaylists, isLoading: spotifyLoading } = useQuery({
    queryKey: ['spotify-playlists'],
    queryFn: () => api.getSpotifyPlaylists({ limit: 50 }),
  })

  const { data: youtubePlaylists, isLoading: youtubeLoading } = useQuery({
    queryKey: ['youtube-playlists'],
    queryFn: () => api.getYouTubePlaylists(),
  })

  const createMutation = useMutation({
    mutationFn: api.createMapping,
    onSuccess: () => {
      navigate({ to: '/mappings' })
    },
  })

  const handleNext = () => {
    if (step < 4) setStep(step + 1)
  }

  const handleBack = () => {
    if (step > 1) setStep(step - 1)
  }

  const handleSubmit = () => {
    createMutation.mutate(formData)
  }

  const selectedSpotifyPlaylist = spotifyPlaylists?.items.find(p => p.id === formData.spotify_playlist_id)
  const selectedYouTubePlaylist = youtubePlaylists?.items.find(p => p.id === formData.youtube_playlist_id)

  return (
    <div className="min-h-screen bg-gray-50 py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-3xl mx-auto">
        <h1 className="text-2xl font-semibold text-gray-900 mb-8">Create New Mapping</h1>

        {/* Progress indicator */}
        <div className="mb-8">
          <div className="flex items-center justify-between">
            {[1, 2, 3, 4].map((i) => (
              <div
                key={i}
                className={`flex items-center ${i < 4 ? 'flex-1' : ''}`}
              >
                <div
                  className={`flex items-center justify-center w-10 h-10 rounded-full ${
                    i <= step ? 'bg-indigo-600 text-white' : 'bg-gray-300 text-gray-600'
                  }`}
                >
                  {i}
                </div>
                {i < 4 && (
                  <div
                    className={`flex-1 h-0.5 mx-2 ${
                      i < step ? 'bg-indigo-600' : 'bg-gray-300'
                    }`}
                  />
                )}
              </div>
            ))}
          </div>
        </div>

        {/* Step content */}
        <div className="bg-white shadow rounded-lg p-6">
          {step === 1 && (
            <div>
              <h2 className="text-lg font-medium mb-4">Step 1: Choose Spotify Playlist</h2>
              {spotifyLoading ? (
                <div className="text-center py-4">Loading playlists...</div>
              ) : (
                <div className="space-y-2">
                  {spotifyPlaylists?.items.map((playlist: SpotifyPlaylist) => (
                    <label
                      key={playlist.id}
                      className={`flex items-center p-4 border rounded-lg cursor-pointer hover:bg-gray-50 ${
                        formData.spotify_playlist_id === playlist.id
                          ? 'border-indigo-600 bg-indigo-50'
                          : 'border-gray-300'
                      }`}
                    >
                      <input
                        type="radio"
                        name="spotify_playlist"
                        value={playlist.id}
                        checked={formData.spotify_playlist_id === playlist.id}
                        onChange={(e) =>
                          setFormData({ ...formData, spotify_playlist_id: e.target.value })
                        }
                        className="mr-3"
                      />
                      <div className="flex-1">
                        <div className="font-medium">{playlist.name}</div>
                        <div className="text-sm text-gray-500">
                          {playlist.track_count} tracks • {playlist.public ? 'Public' : 'Private'}
                        </div>
                      </div>
                    </label>
                  ))}
                </div>
              )}
            </div>
          )}

          {step === 2 && (
            <div>
              <h2 className="text-lg font-medium mb-4">Step 2: Choose YouTube Playlist</h2>
              {youtubeLoading ? (
                <div className="text-center py-4">Loading playlists...</div>
              ) : (
                <div className="space-y-2">
                  {youtubePlaylists?.items.map((playlist: YouTubePlaylist) => (
                    <label
                      key={playlist.id}
                      className={`flex items-center p-4 border rounded-lg cursor-pointer hover:bg-gray-50 ${
                        formData.youtube_playlist_id === playlist.id
                          ? 'border-indigo-600 bg-indigo-50'
                          : 'border-gray-300'
                      }`}
                    >
                      <input
                        type="radio"
                        name="youtube_playlist"
                        value={playlist.id}
                        checked={formData.youtube_playlist_id === playlist.id}
                        onChange={(e) =>
                          setFormData({ ...formData, youtube_playlist_id: e.target.value })
                        }
                        className="mr-3"
                      />
                      <div className="flex-1">
                        <div className="font-medium">{playlist.title}</div>
                        <div className="text-sm text-gray-500">
                          {playlist.itemCount} items
                        </div>
                      </div>
                    </label>
                  ))}
                  <div className="mt-4 p-4 bg-gray-100 rounded-lg">
                    <p className="text-sm text-gray-600">
                      Creating new playlists on YouTube is coming soon!
                    </p>
                  </div>
                </div>
              )}
            </div>
          )}

          {step === 3 && (
            <div>
              <h2 className="text-lg font-medium mb-4">Step 3: Sync Options</h2>
              <div className="space-y-6">
                <div>
                  <label className="flex items-center">
                    <input
                      type="checkbox"
                      checked={formData.sync_name}
                      onChange={(e) =>
                        setFormData({ ...formData, sync_name: e.target.checked })
                      }
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
                      checked={formData.sync_tracks}
                      onChange={(e) =>
                        setFormData({ ...formData, sync_tracks: e.target.checked })
                      }
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
                      min="5"
                      max="720"
                      step="5"
                      value={formData.interval_minutes}
                      onChange={(e) =>
                        setFormData({ ...formData, interval_minutes: Number(e.target.value) })
                      }
                      className="mt-2 w-full"
                    />
                    <div className="flex justify-between text-sm text-gray-500 mt-1">
                      <span>5 min</span>
                      <span className="font-medium">{formData.interval_minutes} minutes</span>
                      <span>720 min</span>
                    </div>
                  </label>
                </div>
              </div>
            </div>
          )}

          {step === 4 && (
            <div>
              <h2 className="text-lg font-medium mb-4">Review & Save</h2>
              <div className="space-y-4">
                <div>
                  <h3 className="font-medium text-gray-900">Spotify Playlist</h3>
                  <p className="text-gray-600">{selectedSpotifyPlaylist?.name || formData.spotify_playlist_id}</p>
                </div>

                <div>
                  <h3 className="font-medium text-gray-900">YouTube Playlist</h3>
                  <p className="text-gray-600">{selectedYouTubePlaylist?.title || formData.youtube_playlist_id}</p>
                </div>

                <div>
                  <h3 className="font-medium text-gray-900">Sync Options</h3>
                  <ul className="text-gray-600 space-y-1">
                    {formData.sync_name && <li>✓ Sync playlist name</li>}
                    {formData.sync_tracks && <li>✓ Sync tracks</li>}
                    <li>Sync every {formData.interval_minutes} minutes</li>
                  </ul>
                </div>
              </div>
            </div>
          )}

          {/* Navigation buttons */}
          <div className="mt-8 flex justify-between">
            {step > 1 && (
              <button
                onClick={handleBack}
                className="px-4 py-2 border border-gray-300 rounded-md text-gray-700 hover:bg-gray-50"
              >
                Back
              </button>
            )}
            <div className="ml-auto">
              {step < 4 ? (
                <button
                  onClick={handleNext}
                  disabled={
                    (step === 1 && !formData.spotify_playlist_id) ||
                    (step === 2 && !formData.youtube_playlist_id)
                  }
                  className="px-4 py-2 bg-indigo-600 text-white rounded-md hover:bg-indigo-700 disabled:bg-gray-400"
                >
                  Next
                </button>
              ) : (
                <button
                  onClick={handleSubmit}
                  disabled={createMutation.isPending}
                  className="px-4 py-2 bg-indigo-600 text-white rounded-md hover:bg-indigo-700 disabled:bg-gray-400"
                >
                  {createMutation.isPending ? 'Creating...' : 'Create Mapping'}
                </button>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export const Route = createLazyFileRoute('/_authenticated/mappings/new')({
  component: NewMapping,
}) 