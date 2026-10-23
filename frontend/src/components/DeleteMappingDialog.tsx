import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Trash2 } from 'lucide-react'
import { mappingsAPI, type DeleteMappingResult, type Mapping } from '../lib/api/index'
import { Button, Dialog, Notice, ServiceIcon } from './ui'

/**
 * Confirms deleting a mapping. The playlists stay unless the user ticks one or both; deleting
 * the YouTube one costs 50 quota units, and Spotify "deletes" by unfollowing.
 */
export function DeleteMappingDialog({ mapping, onClose }: { mapping: Mapping; onClose: () => void }) {
  const queryClient = useQueryClient()
  const [spotify, setSpotify] = useState(false)
  const [youtube, setYoutube] = useState(false)
  const [result, setResult] = useState<DeleteMappingResult | null>(null)

  const remove = useMutation({
    mutationFn: () => mappingsAPI.delete(mapping.id, { spotify, youtube }),
    onSuccess: (res) => {
      for (const key of ['mappings', 'dashboard-stats', 'quota', 'spotify-playlists', 'youtube-playlists']) {
        queryClient.invalidateQueries({ queryKey: [key] })
      }
      // Close straight away unless a playlist couldn't be deleted.
      if (res.playlists.some((p) => !p.deleted)) setResult(res)
      else onClose()
    },
  })

  const sides = [
    {
      service: 'spotify' as const,
      name: mapping.spotify_playlist_name || mapping.spotify_playlist_id,
      checked: spotify,
      set: setSpotify,
      note: 'Removes it from your library (Spotify unfollows; it can be followed again).',
    },
    {
      service: 'youtube' as const,
      name: mapping.youtube_playlist_name || mapping.youtube_playlist_id,
      checked: youtube,
      set: setYoutube,
      note: 'Deletes it permanently. Uses 50 YouTube quota units.',
    },
  ]

  return (
    <Dialog
      title="Delete mapping"
      subtitle="Spotube stops syncing these playlists."
      onClose={onClose}
      footer={
        result ? (
          <Button onClick={onClose}>Close</Button>
        ) : (
          <>
            <Button onClick={onClose}>Cancel</Button>
            <Button
              variant="danger"
              icon={<Trash2 className="h-4 w-4" />}
              loading={remove.isPending}
              onClick={() => remove.mutate()}
            >
              {spotify || youtube ? 'Delete mapping and playlists' : 'Delete mapping'}
            </Button>
          </>
        )
      }
    >
      {result ? (
        <div className="space-y-2">
          <Notice tone="success">The mapping was deleted.</Notice>
          {result.playlists.map((p) => (
            <Notice key={p.service} tone={p.deleted ? 'success' : 'error'}>
              {p.service === 'spotify' ? 'Spotify' : 'YouTube'} playlist{' '}
              {p.deleted ? 'deleted.' : `not deleted: ${p.error}`}
            </Notice>
          ))}
        </div>
      ) : (
        <div className="space-y-4">
          <p className="text-gray-600">The playlists are kept by default. Also delete:</p>
          {sides.map((side) => (
            <label key={side.service} className="flex cursor-pointer items-start gap-3">
              <input
                type="checkbox"
                checked={side.checked}
                onChange={(e) => side.set(e.target.checked)}
                className="mt-0.5 h-4 w-4 accent-youtube-700"
                aria-label={`Also delete the ${side.service === 'spotify' ? 'Spotify' : 'YouTube'} playlist`}
              />
              <span className="min-w-0">
                <span className="flex items-center gap-2 font-medium text-ink">
                  <ServiceIcon service={side.service} className="h-4 w-4" />
                  <span className="truncate">{side.name}</span>
                </span>
                <span className="block text-xs text-gray-500">{side.note}</span>
              </span>
            </label>
          ))}
          {remove.error && <Notice tone="error">{remove.error.message}</Notice>}
        </div>
      )}
    </Dialog>
  )
}
