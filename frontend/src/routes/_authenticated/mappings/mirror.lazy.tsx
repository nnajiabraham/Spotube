import { createLazyFileRoute, Link } from '@tanstack/react-router'
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Copy, Search } from 'lucide-react'
import { mappingsAPI, oauthAPI, settingsAPI, type EstimatePair, type MirrorResult } from '../../../lib/api/index'
import type { Service } from '../../../lib/api/tracks'
import { QuotaEstimate } from '../../../components/QuotaEstimate'
import { Button, Card, Notice, Page, Pill, ServiceIcon, type PillTone } from '../../../components/ui'
import { buttonClass, linkClass, serviceName } from '../../../lib/format'
import { browserTimeZone } from '../../../lib/schedule'

type Row = {
  id: string
  name: string
  tracks: number
  paired: boolean
  sameName: boolean
}

const normalize = (s: string) => s.trim().toLowerCase()

const resultTone: Record<MirrorResult['status'], PillTone> = {
  created: 'green',
  already_paired: 'gray',
  waiting_quota: 'yellow',
  failed: 'red',
}
const resultLabel: Record<MirrorResult['status'], string> = {
  created: 'created',
  already_paired: 'already paired',
  waiting_quota: 'waiting for quota',
  failed: 'failed',
}

function Checkbox({ checked, onChange, label }: { checked: boolean; onChange: (v: boolean) => void; label: string }) {
  return (
    <label className="flex cursor-pointer items-center gap-2 text-sm text-gray-700">
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="h-4 w-4 accent-ink"
      />
      {label}
    </label>
  )
}

function MirrorPlaylists() {
  const queryClient = useQueryClient()
  const [source, setSource] = useState<Service>('spotify')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [query, setQuery] = useState('')
  const [syncTracks, setSyncTracks] = useState(true)
  const [syncName, setSyncName] = useState(false)
  const [syncNow, setSyncNow] = useState<boolean | null>(null)
  const target: Service = source === 'spotify' ? 'youtube' : 'spotify'

  const spotify = useQuery({
    queryKey: ['spotify-playlists'],
    queryFn: oauthAPI.spotify.getPlaylists,
  })
  const youtube = useQuery({
    queryKey: ['youtube-playlists'],
    queryFn: oauthAPI.youtube.getPlaylists,
  })
  const existing = useQuery({
    queryKey: ['mappings', 'all'],
    queryFn: () => mappingsAPI.getList({ per_page: 200 }),
  })
  const settings = useQuery({
    queryKey: ['settings'],
    queryFn: settingsAPI.list,
  })
  const startNow = syncNow ?? settings.data?.find((s) => s.key === 'sync_new_mappings_now')?.value !== 'false'

  const paired = new Set(
    existing.data?.items.map((m) => (source === 'spotify' ? m.spotify_playlist_id : m.youtube_playlist_id))
  )
  const targetNames = new Set(
    (target === 'spotify' ? spotify.data?.map((p) => p.name) : youtube.data?.map((p) => p.name || p.title || ''))?.map(
      normalize
    )
  )
  // Playlists that can still be mirrored first; paired ones stay listed, greyed out, at the end.
  const rows: Row[] = (
    source === 'spotify'
      ? (spotify.data ?? []).map((p) => ({ id: p.id, name: p.name, tracks: p.track_count ?? 0 }))
      : (youtube.data ?? []).map((p) => ({ id: p.id, name: p.name || p.title || p.id, tracks: p.itemCount ?? 0 }))
  )
    .map((p) => ({ ...p, paired: paired.has(p.id), sameName: targetNames.has(normalize(p.name)) }))
    .sort((a, b) => Number(a.paired) - Number(b.paired))
  const sourceQuery = source === 'spotify' ? spotify : youtube
  const visible = rows.filter((r) => normalize(r.name).includes(normalize(query)))
  const selectable = visible.filter((r) => !r.paired)
  const chosen = rows.filter((r) => selected.has(r.id) && !r.paired)
  const allVisibleSelected = selectable.length > 0 && selectable.every((r) => selected.has(r.id))

  const toggle = (id: string) =>
    setSelected((s) => {
      const next = new Set(s)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  const toggleAll = () =>
    setSelected((s) => {
      const next = new Set(s)
      for (const r of selectable) {
        if (allVisibleSelected) next.delete(r.id)
        else next.add(r.id)
      }
      return next
    })
  const switchSource = (s: Service) => {
    setSource(s)
    setSelected(new Set())
    mirror.reset()
  }

  const estimatePairs: EstimatePair[] = chosen.map((r) =>
    source === 'spotify'
      ? {
          spotify_playlist_id: r.id,
          spotify_tracks: r.tracks,
          youtube_videos: 0,
          create: 'youtube',
        }
      : { spotify_tracks: 0, youtube_videos: r.tracks, create: 'spotify' }
  )

  const mirror = useMutation({
    mutationFn: () =>
      mappingsAPI.mirror({
        source,
        playlists: chosen.map((r) => ({
          id: r.id,
          name: r.name,
          tracks: r.tracks,
        })),
        sync_tracks: syncTracks,
        sync_name: syncName,
        sync_now: startNow,
        schedule_tz: browserTimeZone(),
      }),
    onSuccess: (res) => {
      setSelected(new Set())
      for (const key of ['mappings', 'dashboard-stats', 'quota', `${target}-playlists`]) {
        queryClient.invalidateQueries({ queryKey: [key] })
      }
      if (res.items.some((r) => r.status === 'waiting_quota')) {
        // Keep the ones that didn't fit today selected so they can be retried after the reset.
        setSelected(new Set(res.items.filter((r) => r.status === 'waiting_quota').map((r) => r.playlist_id)))
      }
    },
  })

  return (
    <Page
      title="Mirror playlists"
      description={
        <>
          Pick playlists on one side and Spotube creates a copy of each on the other, each as its own mapping with a
          recommended schedule. A playlist can only be in one mapping, so ones already paired can't be picked.
        </>
      }
      actions={
        <Link to="/mappings/new" className={buttonClass.secondary}>
          Pair two playlists instead
        </Link>
      }
    >
      <div className="mb-4 flex flex-wrap items-center gap-2" role="radiogroup" aria-label="Copy from">
        <span className="text-sm text-gray-500">Copy from</span>
        {(['spotify', 'youtube'] as const).map((s) => (
          <button
            key={s}
            type="button"
            role="radio"
            aria-checked={source === s}
            onClick={() => switchSource(s)}
            className={`inline-flex items-center gap-2 rounded-md border px-3 py-1.5 text-sm ${
              source === s
                ? 'border-ink bg-ink text-white'
                : 'border-gray-300 bg-white text-gray-600 hover:border-gray-400'
            }`}
          >
            <ServiceIcon service={s} className="h-4 w-4" />
            {serviceName(s)} → {serviceName(s === 'spotify' ? 'youtube' : 'spotify')}
          </button>
        ))}
      </div>

      <Card className="overflow-hidden">
        <div className="flex flex-wrap items-center gap-3 border-b border-gray-200 px-4 py-2.5">
          <input
            type="checkbox"
            checked={allVisibleSelected}
            disabled={selectable.length === 0}
            onChange={toggleAll}
            aria-label="Select all"
            className="h-4 w-4 accent-ink"
          />
          <label className="flex flex-1 items-center gap-2 rounded-md bg-gray-50 px-3 py-1.5">
            <Search className="h-4 w-4 text-gray-400" />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={`Search ${rows.length} ${serviceName(source)} playlists`}
              className="w-full bg-transparent text-sm outline-none"
              aria-label="Search playlists"
            />
          </label>
          <span className="text-xs text-gray-500">{chosen.length} selected</span>
        </div>
        <div className="max-h-[28rem] overflow-y-auto">
          {sourceQuery.isLoading ? (
            <p className="p-4 text-sm text-gray-500">Loading playlists…</p>
          ) : sourceQuery.error ? (
            <p className="p-4 text-sm text-youtube-700">Couldn't load playlists ({sourceQuery.error.message}).</p>
          ) : visible.length === 0 ? (
            <p className="p-4 text-sm text-gray-500">No playlists match.</p>
          ) : (
            visible.map((r) => (
              <label
                key={r.id}
                className={`flex items-center gap-3 border-b border-gray-100 px-4 py-2.5 text-sm last:border-0 ${
                  r.paired ? 'cursor-not-allowed opacity-50' : 'cursor-pointer hover:bg-gray-50'
                }`}
              >
                <input
                  type="checkbox"
                  checked={!r.paired && selected.has(r.id)}
                  disabled={r.paired}
                  onChange={() => toggle(r.id)}
                  aria-label={r.name}
                  className="h-4 w-4 accent-ink"
                />
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium text-ink">{r.name}</span>
                  <span className="block text-xs text-gray-500">
                    {r.tracks.toLocaleString()} {source === 'spotify' ? 'tracks' : 'videos'}
                  </span>
                </span>
                {r.paired ? (
                  <Pill>Already paired</Pill>
                ) : (
                  r.sameName && (
                    <Pill
                      tone="yellow"
                      title={`A ${serviceName(target)} playlist has this name; pairing it may be better`}
                    >
                      Same name on {serviceName(target)}
                    </Pill>
                  )
                )}
              </label>
            ))
          )}
        </div>
      </Card>

      <Card className="mt-4 space-y-3 p-5">
        <Checkbox checked={syncTracks} onChange={setSyncTracks} label="Sync tracks" />
        <Checkbox checked={syncName} onChange={setSyncName} label="Sync playlist name" />
        <Checkbox checked={startNow} onChange={setSyncNow} label="Start syncing now" />
        <p className="text-xs text-gray-500">
          Each new {serviceName(target)} playlist is private
          {target === 'youtube'
            ? ' and uses 50 quota units to create'
            : ' (Spotify shows app-created playlists as public)'}
          . Schedules are recommended per playlist and spaced apart; change them on each mapping later.
        </p>
      </Card>

      {chosen.length > 0 && (
        <div className="mt-4">
          <QuotaEstimate pairs={estimatePairs} />
        </div>
      )}

      {mirror.error && (
        <Notice tone="error" className="mt-4">
          {mirror.error.message}
        </Notice>
      )}

      {mirror.data && (
        <section aria-label="Results" className="mt-4">
          <Card className="divide-y divide-gray-100">
            {mirror.data.items.map((r) => (
              <div key={r.playlist_id} className="flex items-center gap-3 px-4 py-2.5 text-sm">
                <span className="min-w-0 flex-1">
                  <span className="block truncate text-ink">{r.name}</span>
                  {r.error && <span className="block text-xs text-gray-500">{r.error}</span>}
                </span>
                <Pill tone={resultTone[r.status]}>{resultLabel[r.status]}</Pill>
              </div>
            ))}
            <div className="px-4 py-2.5 text-sm">
              <Link to="/mappings" className={linkClass}>
                View mappings
              </Link>
            </div>
          </Card>
        </section>
      )}

      <div className="mt-6 flex items-center justify-end gap-3">
        <Link to="/mappings" className={buttonClass.secondary}>
          Cancel
        </Link>
        <Button
          variant="primary"
          icon={<Copy className="h-4 w-4" />}
          disabled={chosen.length === 0 || chosen.length > 100}
          loading={mirror.isPending}
          onClick={() => mirror.mutate()}
        >
          {chosen.length > 100
            ? 'Pick at most 100'
            : `Create ${serviceName(target)} ${chosen.length === 1 ? 'copy' : `copies (${chosen.length})`}`}
        </Button>
      </div>
    </Page>
  )
}

export const Route = createLazyFileRoute('/_authenticated/mappings/mirror')({
  component: MirrorPlaylists,
})
