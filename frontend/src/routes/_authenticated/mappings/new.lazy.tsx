import { createLazyFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Check, Copy, Link2, Plus, Search, Sparkles } from 'lucide-react'
import { mappingsAPI, oauthAPI, settingsAPI, type EstimatePair } from '../../../lib/api/index'
import type { Service } from '../../../lib/api/tracks'
import { MappingOptions, type MappingOptionsValue } from '../../../components/MappingOptions'
import { QuotaEstimate } from '../../../components/QuotaEstimate'
import { Button, Card, Notice, Page, Pill, ServiceIcon } from '../../../components/ui'
import { buttonClass, inputClass, linkClass, serviceName } from '../../../lib/format'
import { EVERY_DAY, browserTimeZone, describeSchedule } from '../../../lib/schedule'

type PickerItem = { id: string; name: string; detail: string; paired: boolean; sameName?: boolean }

const normalize = (s: string) => s.trim().toLowerCase()

function RadioDot({ selected }: { selected: boolean }) {
  return (
    <span
      className={`flex h-4 w-4 shrink-0 items-center justify-center rounded-full border ${
        selected ? 'border-ink bg-ink text-white' : 'border-gray-300'
      }`}
    >
      {selected && <Check className="h-3 w-3" />}
    </span>
  )
}

function PlaylistPicker({
  service,
  items,
  isLoading,
  error,
  selectedId,
  onSelect,
  creating,
  canCreate,
  onCreate,
  newName,
  onNewName,
  namePlaceholder,
}: {
  service: Service
  items: PickerItem[]
  isLoading: boolean
  error: Error | null
  selectedId: string
  onSelect: (id: string) => void
  /** "New playlist" is selected on this side. */
  creating: boolean
  /** False while the other side is the one being created. */
  canCreate: boolean
  onCreate: () => void
  newName: string
  onNewName: (name: string) => void
  namePlaceholder: string
}) {
  const [query, setQuery] = useState('')
  const visible = items.filter((p) => normalize(p.name).includes(normalize(query)))
  const name = serviceName(service)

  return (
    <Card className="flex flex-col overflow-hidden">
      <div className="flex items-center gap-2 border-b border-gray-200 px-4 py-3">
        <ServiceIcon service={service} className="h-5 w-5" />
        <h2 className="font-medium text-ink">{name} playlist</h2>
      </div>
      <div className="border-b border-gray-100 px-3 py-2">
        <label className="flex items-center gap-2 rounded-md bg-gray-50 px-3 py-1.5">
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
        <button
          type="button"
          role="radio"
          aria-checked={creating}
          disabled={!canCreate}
          onClick={onCreate}
          className={`flex w-full items-center gap-3 border-b border-dashed border-gray-200 px-4 py-2.5 text-left text-sm disabled:cursor-not-allowed disabled:opacity-50 ${
            creating ? 'bg-gray-100' : 'hover:bg-gray-50'
          }`}
        >
          <RadioDot selected={creating} />
          <span className="min-w-0 flex-1">
            <span className="flex items-center gap-1.5 font-medium text-ink">
              <Plus className="h-3.5 w-3.5" />
              New {name} playlist
            </span>
            <span className="block text-xs text-gray-500">
              {canCreate ? `Spotube creates it and copies the other side's tracks in` : 'Only one side can be new'}
            </span>
          </span>
        </button>
        {creating && (
          <div className="border-b border-gray-200 bg-gray-50 px-4 py-3">
            <label className="block">
              <span className="mb-1 block text-[11px] font-medium uppercase tracking-wider text-gray-500">Name</span>
              <input
                className={inputClass}
                value={newName}
                placeholder={namePlaceholder || 'Name for the new playlist'}
                onChange={(e) => onNewName(e.target.value)}
                aria-label={`New ${name} playlist name`}
              />
            </label>
          </div>
        )}
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
            const selected = !creating && p.id === selectedId
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
                <RadioDot selected={selected} />
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium text-ink">{p.name}</span>
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
  // The side Spotube should create instead of pairing an existing playlist.
  const [creating, setCreating] = useState<Service | null>(null)
  const [newName, setNewName] = useState('')
  const [options, setOptions] = useState<MappingOptionsValue>({
    sync_tracks: true,
    sync_name: false,
    interval_minutes: 60,
    schedule_start: '00:00',
    schedule_days: EVERY_DAY,
    schedule_tz: browserTimeZone(),
  })
  // The schedule follows the recommendation until the user changes it.
  const [scheduleEdited, setScheduleEdited] = useState(false)
  const [syncNow, setSyncNow] = useState<boolean | null>(null)

  const spotify = useQuery({ queryKey: ['spotify-playlists'], queryFn: oauthAPI.spotify.getPlaylists })
  const youtube = useQuery({ queryKey: ['youtube-playlists'], queryFn: oauthAPI.youtube.getPlaylists })
  const existing = useQuery({ queryKey: ['mappings', 'all'], queryFn: () => mappingsAPI.getList({ per_page: 200 }) })
  const settings = useQuery({ queryKey: ['settings'], queryFn: settingsAPI.list })
  const syncNowDefault = settings.data?.find((s) => s.key === 'sync_new_mappings_now')?.value !== 'false'

  const pairedSpotify = new Set(existing.data?.items.map((m) => m.spotify_playlist_id))
  const pairedYouTube = new Set(existing.data?.items.map((m) => m.youtube_playlist_id))
  const selectedSpotify = creating === 'spotify' ? undefined : spotify.data?.find((p) => p.id === spotifyId)
  const selectedYouTube = creating === 'youtube' ? undefined : youtube.data?.find((p) => p.id === youtubeId)
  const spotifyName = selectedSpotify?.name ?? ''
  const youtubeName = selectedYouTube ? selectedYouTube.name || selectedYouTube.title || '' : ''

  // Put a playlist with the same name as the one chosen on the other side first.
  const byName = <T extends PickerItem>(items: T[], target: string) =>
    items
      .map((p) => ({ ...p, sameName: target !== '' && normalize(p.name) === normalize(target) }))
      .sort((a, b) => Number(b.sameName) - Number(a.sameName))

  const spotifyItems = byName(
    (spotify.data ?? []).map((p) => ({
      id: p.id,
      name: p.name,
      detail: [`${p.track_count ?? 0} tracks`, p.owner?.display_name && `by ${p.owner.display_name}`].filter(Boolean).join(' · '),
      paired: pairedSpotify.has(p.id),
    })),
    youtubeName,
  )
  const youtubeItems = byName(
    (youtube.data ?? []).map((p) => ({
      id: p.id,
      name: p.name || p.title || p.id,
      detail: `${p.itemCount ?? 0} videos`,
      paired: pairedYouTube.has(p.id),
    })),
    spotifyName,
  )

  const spotifyTracks = selectedSpotify?.track_count ?? 0
  const youtubeVideos = selectedYouTube?.itemCount ?? 0
  const tz = options.schedule_tz ?? ''
  const recommendation = useQuery({
    queryKey: ['schedule-recommendation', spotifyTracks, youtubeVideos, tz],
    queryFn: () => mappingsAPI.recommend({ spotify_tracks: spotifyTracks, youtube_videos: youtubeVideos, tz }),
  })
  const recommended = recommendation.data?.schedule
  useEffect(() => {
    if (recommended && !scheduleEdited) setOptions((o) => ({ ...o, ...recommended, schedule_tz: recommended.schedule_tz || o.schedule_tz }))
  }, [recommended, scheduleEdited])
  const changeOptions = (next: MappingOptionsValue) => {
    if (
      next.interval_minutes !== options.interval_minutes ||
      next.schedule_start !== options.schedule_start ||
      next.schedule_days !== options.schedule_days
    ) {
      setScheduleEdited(true)
    }
    setOptions(next)
  }

  const otherName = creating === 'youtube' ? spotifyName : youtubeName
  const finalNewName = newName.trim() || otherName
  const ready = creating
    ? (creating === 'youtube' ? !!spotifyId : !!youtubeId) && finalNewName !== ''
    : !!spotifyId && !!youtubeId

  const toggleCreate = (service: Service) => {
    setCreating((c) => (c === service ? null : service))
    setNewName('')
  }
  const select = (service: Service, id: string) => {
    if (creating === service) setCreating(null)
    if (service === 'spotify') setSpotifyId(id)
    else setYoutubeId(id)
  }

  const estimatePairs: EstimatePair[] = ready
    ? [
        {
          spotify_playlist_id: creating === 'spotify' ? undefined : spotifyId,
          spotify_tracks: creating === 'spotify' ? 0 : spotifyTracks,
          youtube_videos: creating === 'youtube' ? 0 : youtubeVideos,
          create: creating ?? undefined,
        },
      ]
    : []

  const startNow = syncNow ?? syncNowDefault
  const create = useMutation({
    mutationFn: () =>
      mappingsAPI.create({
        spotify_playlist_id: creating === 'spotify' ? '' : spotifyId,
        youtube_playlist_id: creating === 'youtube' ? '' : youtubeId,
        spotify_playlist_name: creating === 'spotify' ? finalNewName : spotifyName,
        youtube_playlist_name: creating === 'youtube' ? finalNewName : youtubeName,
        create_playlist: creating ?? undefined,
        sync_now: startNow,
        ...options,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['mappings'] })
      if (creating) queryClient.invalidateQueries({ queryKey: [`${creating}-playlists`] })
      navigate({ to: '/mappings' })
    },
  })

  const pickerProps = (service: Service) => ({
    service,
    creating: creating === service,
    canCreate: creating !== (service === 'spotify' ? 'youtube' : 'spotify'),
    onCreate: () => toggleCreate(service),
    newName,
    onNewName: setNewName,
    namePlaceholder: otherName,
    selectedId: service === 'spotify' ? spotifyId : youtubeId,
    onSelect: (id: string) => select(service, id),
  })

  return (
    <Page
      title="Pair playlists"
      description="Choose a playlist on each side, or let Spotube create one side from the other. Each playlist can be in one mapping."
      actions={
        <Link to="/mappings/mirror" className={buttonClass.secondary}>
          <Copy className="h-4 w-4" /> Mirror several
        </Link>
      }
    >
      <div className="grid gap-4 md:grid-cols-2">
        <PlaylistPicker {...pickerProps('spotify')} items={spotifyItems} isLoading={spotify.isLoading} error={spotify.error} />
        <PlaylistPicker {...pickerProps('youtube')} items={youtubeItems} isLoading={youtube.isLoading} error={youtube.error} />
      </div>

      {creating && (
        <Notice tone="info" className="mt-4">
          {creating === 'youtube' ? (
            <>
              Creating the YouTube playlist uses 50 quota units; it is private. The first sync then copies each track over: about
              150 units per track Spotube hasn't matched before (a search and an insert), 50 for ones it has.
            </>
          ) : (
            <>
              Creating the Spotify playlist is free. Spotify ignores the private flag for apps, so it will show as public on your
              profile. The first sync then searches Spotify for each video and adds the matches; weak matches go to Review.
            </>
          )}
        </Notice>
      )}

      <Card className="mt-4 p-5">
        <MappingOptions
          value={options}
          onChange={changeOptions}
          scheduleNote={
            recommendation.data && (
              <div className="space-y-2">
                <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-gray-600">
                  <Sparkles className="h-3.5 w-3.5 text-amber-500" aria-hidden="true" />
                  <span>
                    Recommended: {describeSchedule(recommendation.data.schedule)}. {recommendation.data.reason}; spaced
                    away from your other {existing.data?.totalItems ?? 0} mappings.
                  </span>
                  {scheduleEdited && (
                    <button type="button" className={linkClass} onClick={() => setScheduleEdited(false)}>
                      Use recommended
                    </button>
                  )}
                </p>
                {recommendation.data.warning && <Notice tone="warning">{recommendation.data.warning}</Notice>}
              </div>
            )
          }
        />
        <label className="mt-5 flex cursor-pointer items-start gap-3 border-t border-gray-100 pt-5">
          <input
            type="checkbox"
            checked={startNow}
            onChange={(e) => setSyncNow(e.target.checked)}
            className="mt-0.5 h-4 w-4 accent-ink"
          />
          <span>
            <span className="block text-sm font-medium text-gray-900">Start syncing now</span>
            <span className="block text-sm text-gray-500">
              Run the first sync in the background straight away. Off: it waits for the first scheduled run.
            </span>
          </span>
        </label>
      </Card>

      {ready && (
        <div className="mt-4">
          <QuotaEstimate pairs={estimatePairs} />
        </div>
      )}

      {create.error && (
        <Notice tone="error" className="mt-4">
          {create.error.message}
        </Notice>
      )}

      <div className="mt-6 flex items-center justify-end gap-3">
        <Link to="/mappings" className={buttonClass.secondary}>
          Cancel
        </Link>
        <Button
          variant="primary"
          icon={creating ? <Plus className="h-4 w-4" /> : <Link2 className="h-4 w-4" />}
          disabled={!ready}
          loading={create.isPending}
          onClick={() => create.mutate()}
        >
          {creating ? `Create ${serviceName(creating)} playlist and pair` : 'Pair playlists'}
        </Button>
      </div>
    </Page>
  )
}

export const Route = createLazyFileRoute('/_authenticated/mappings/new')({
  component: NewMapping,
})
