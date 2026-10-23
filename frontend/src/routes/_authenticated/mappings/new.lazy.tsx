import { createLazyFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Check, Search } from 'lucide-react'
import { mappingsAPI, oauthAPI } from '../../../lib/api/index'
import { MappingOptions, type MappingOptionsValue } from '../../../components/MappingOptions'
import { Card, Page, Pill, ServiceIcon } from '../../../components/ui'
import { buttonClass } from '../../../lib/format'

type PickerItem = { id: string; name: string; detail: string; paired: boolean; sameName?: boolean }

const normalize = (s: string) => s.trim().toLowerCase()

function PlaylistPicker({
  service,
  items,
  isLoading,
  error,
  selectedId,
  onSelect,
}: {
  service: 'spotify' | 'youtube'
  items: PickerItem[]
  isLoading: boolean
  error: Error | null
  selectedId: string
  onSelect: (id: string) => void
}) {
  const [query, setQuery] = useState('')
  const visible = items.filter((p) => normalize(p.name).includes(normalize(query)))

  return (
    <Card className="flex flex-col overflow-hidden">
      <div className="flex items-center gap-2 border-b border-gray-200 px-4 py-3">
        <ServiceIcon service={service} className="h-5 w-5" />
        <h2 className="font-medium text-gray-900">{service === 'spotify' ? 'Spotify' : 'YouTube'} playlist</h2>
      </div>
      <div className="border-b border-gray-100 px-3 py-2">
        <label className="flex items-center gap-2 rounded-lg bg-gray-50 px-3 py-1.5">
          <Search className="h-4 w-4 text-gray-400" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={`Search ${items.length} playlists`}
            className="w-full bg-transparent text-sm outline-none"
            aria-label={`Search ${service} playlists`}
          />
        </label>
      </div>
      <div className="h-80 overflow-y-auto" role="radiogroup" aria-label={`${service} playlists`}>
        {isLoading ? (
          <p className="p-4 text-sm text-gray-500">Loading playlists…</p>
        ) : error ? (
          <p className="p-4 text-sm text-youtube-700">
            Couldn't load playlists ({error.message}). Check the account on the{' '}
            <Link to="/dashboard" className="underline">
              dashboard
            </Link>
            .
          </p>
        ) : visible.length === 0 ? (
          <p className="p-4 text-sm text-gray-500">No playlists match.</p>
        ) : (
          visible.map((p) => {
            const selected = p.id === selectedId
            return (
              <button
                key={p.id}
                type="button"
                role="radio"
                aria-checked={selected}
                disabled={p.paired}
                onClick={() => onSelect(p.id)}
                className={`flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm disabled:cursor-not-allowed disabled:opacity-50 ${
                  selected ? 'bg-gray-100' : 'hover:bg-gray-50'
                }`}
              >
                <span
                  className={`flex h-4 w-4 shrink-0 items-center justify-center rounded-full border ${
                    selected ? 'border-ink bg-ink text-white' : 'border-gray-300'
                  }`}
                >
                  {selected && <Check className="h-3 w-3" />}
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium text-gray-900">{p.name}</span>
                  <span className="block text-xs text-gray-500">{p.detail}</span>
                </span>
                {p.paired && <Pill>Already paired</Pill>}
                {!p.paired && p.sameName && <Pill tone="green">Same name</Pill>}
              </button>
            )
          })
        )}
      </div>
    </Card>
  )
}

function NewMapping() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [spotifyId, setSpotifyId] = useState('')
  const [youtubeId, setYoutubeId] = useState('')
  const [options, setOptions] = useState<MappingOptionsValue>({
    sync_tracks: true,
    sync_name: false,
    interval_minutes: 60,
  })

  const spotify = useQuery({ queryKey: ['spotify-playlists'], queryFn: oauthAPI.spotify.getPlaylists })
  const youtube = useQuery({ queryKey: ['youtube-playlists'], queryFn: oauthAPI.youtube.getPlaylists })
  const existing = useQuery({ queryKey: ['mappings', 'all'], queryFn: () => mappingsAPI.getList({ per_page: 200 }) })

  const pairedSpotify = new Set(existing.data?.items.map((m) => m.spotify_playlist_id))
  const pairedYouTube = new Set(existing.data?.items.map((m) => m.youtube_playlist_id))
  const selectedSpotify = spotify.data?.find((p) => p.id === spotifyId)
  const selectedYouTube = youtube.data?.find((p) => p.id === youtubeId)

  const spotifyItems: PickerItem[] = (spotify.data ?? []).map((p) => ({
    id: p.id,
    name: p.name,
    detail: [`${p.track_count ?? 0} tracks`, p.owner?.display_name && `by ${p.owner.display_name}`].filter(Boolean).join(' · '),
    paired: pairedSpotify.has(p.id),
  }))

  // Put a YouTube playlist with the same name as the chosen Spotify one first.
  const target = selectedSpotify ? normalize(selectedSpotify.name) : ''
  const youtubeItems: PickerItem[] = (youtube.data ?? [])
    .map((p) => {
      const name = p.name || p.title || p.id
      return {
        id: p.id,
        name,
        detail: `${p.itemCount ?? 0} videos`,
        paired: pairedYouTube.has(p.id),
        sameName: target !== '' && normalize(name) === target,
      }
    })
    .sort((a, b) => Number(b.sameName) - Number(a.sameName))

  const create = useMutation({
    mutationFn: () =>
      mappingsAPI.create({
        spotify_playlist_id: spotifyId,
        youtube_playlist_id: youtubeId,
        spotify_playlist_name: selectedSpotify?.name,
        youtube_playlist_name: selectedYouTube?.name || selectedYouTube?.title,
        ...options,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['mappings'] })
      navigate({ to: '/mappings' })
    },
  })

  return (
    <Page title="Pair playlists" description="Choose one playlist on each side. The first sync runs within a minute.">
      <div className="grid gap-4 md:grid-cols-2">
        <PlaylistPicker
          service="spotify"
          items={spotifyItems}
          isLoading={spotify.isLoading}
          error={spotify.error}
          selectedId={spotifyId}
          onSelect={setSpotifyId}
        />
        <PlaylistPicker
          service="youtube"
          items={youtubeItems}
          isLoading={youtube.isLoading}
          error={youtube.error}
          selectedId={youtubeId}
          onSelect={setYoutubeId}
        />
      </div>

      <Card className="mt-4 p-5">
        <MappingOptions value={options} onChange={setOptions} />
      </Card>

      {create.error && (
        <div className="mt-4 rounded-lg border border-youtube/30 bg-youtube-50 px-4 py-3 text-sm text-youtube-700">
          {create.error.message}
        </div>
      )}

      <div className="mt-6 flex items-center justify-end gap-3">
        <Link to="/mappings" className={buttonClass.secondary}>
          Cancel
        </Link>
        <button
          type="button"
          className={buttonClass.primary}
          disabled={!spotifyId || !youtubeId || create.isPending}
          onClick={() => create.mutate()}
        >
          {create.isPending ? 'Saving…' : 'Pair playlists'}
        </button>
      </div>
    </Page>
  )
}

export const Route = createLazyFileRoute('/_authenticated/mappings/new')({
  component: NewMapping,
})
