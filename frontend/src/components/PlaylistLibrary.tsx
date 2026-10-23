import { useState } from 'react'
import { Link } from '@tanstack/react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArchiveRestore, ExternalLink, ListMusic, Search, ShieldCheck } from 'lucide-react'
import { oauthAPI } from '../lib/api/oauth'
import { mappingsAPI } from '../lib/api/mappings'
import { backupsAPI, playlistURL, type Backup } from '../lib/api/backups'
import type { Service } from '../lib/api/tracks'
import { inputClass, serviceName } from '../lib/format'
import { Button, EmptyState, Notice, Pill, ServiceIcon } from './ui'

type Row = { id: string; name: string; image?: string; detail: string }

function usePlaylists(service: Service) {
  return useQuery({
    queryKey: [`${service}-playlists`],
    queryFn: async (): Promise<Row[]> => {
      if (service === 'spotify') {
        const items = await oauthAPI.spotify.getPlaylists()
        return items.map((p) => ({
          id: p.id,
          name: p.name,
          image: p.images?.[p.images.length - 1]?.url,
          detail: [typeof p.track_count === 'number' ? `${p.track_count} tracks` : '', p.public === false ? 'private' : p.public ? 'public' : '']
            .filter(Boolean)
            .join(' · '),
        }))
      }
      const items = await oauthAPI.youtube.getPlaylists()
      return items.map((p) => ({
        id: p.id,
        name: p.name || p.title || p.id,
        detail: typeof p.itemCount === 'number' ? `${p.itemCount} videos` : '',
      }))
    },
  })
}

/** Backups keyed by "service|playlistID". */
function useBackupIndex() {
  return useQuery({
    queryKey: ['backups'],
    queryFn: () => backupsAPI.list().then((r) => r.items),
    select: (items) => new Map(items.map((b) => [`${b.service}|${b.playlist_id}`, b] as const)),
  })
}

/**
 * The account's playlists on one service, showing which are synced by a mapping and which are
 * backed up, with a one-click "Back up". Used on the playlist settings pages and in the
 * Backups page's "Add playlist" dialog.
 */
export function PlaylistLibrary({ service, compact = false }: { service: Service; compact?: boolean }) {
  const queryClient = useQueryClient()
  const [query, setQuery] = useState('')
  const playlists = usePlaylists(service)
  const backups = useBackupIndex()
  const mappings = useQuery({
    queryKey: ['mappings', 'all'],
    queryFn: () => mappingsAPI.getList({ per_page: 100 }).then((r) => r.items),
  })

  const create = useMutation({
    mutationFn: (p: Row) => backupsAPI.create({ service, playlist_id: p.id, playlist_name: p.name }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['backups'] }),
  })

  const pairedWith = (id: string) => {
    const m = mappings.data?.find((m) => (service === 'spotify' ? m.spotify_playlist_id : m.youtube_playlist_id) === id)
    if (!m) return null
    return service === 'spotify' ? m.youtube_playlist_name : m.spotify_playlist_name
  }

  const q = query.trim().toLowerCase()
  const rows = (playlists.data ?? []).filter((p) => p.name.toLowerCase().includes(q))

  return (
    <div>
      <label className="relative mb-3 block">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-gray-400" />
        <input
          className={`${inputClass} pl-9`}
          placeholder={`Filter ${playlists.data?.length ?? ''} ${serviceName(service)} playlists`}
          aria-label={`Filter ${serviceName(service)} playlists`}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      </label>

      {create.error && (
        <Notice tone="error" className="mb-3">
          Couldn't start the backup: {create.error.message}
        </Notice>
      )}

      {playlists.isLoading ? (
        <p className="py-8 text-center text-sm text-gray-500">Loading playlists…</p>
      ) : playlists.error ? (
        <Notice tone="error">
          Couldn't load {serviceName(service)} playlists ({playlists.error.message}). Reconnect the account on the{' '}
          <Link to="/dashboard" className="underline">
            dashboard
          </Link>
          .
        </Notice>
      ) : rows.length === 0 ? (
        <EmptyState icon={<ListMusic />} title={q ? 'No playlists match' : 'No playlists yet'} />
      ) : (
        <ul className={`divide-y divide-gray-200 ${compact ? 'max-h-[55vh] overflow-y-auto' : ''}`}>
          {rows.map((p) => {
            const backup = backups.data?.get(`${service}|${p.id}`)
            const paired = pairedWith(p.id)
            return (
              <li key={p.id} className="flex items-center gap-3 py-2.5">
                {p.image ? (
                  <img src={p.image} alt="" className="h-10 w-10 shrink-0 rounded-md object-cover" loading="lazy" />
                ) : (
                  <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-gray-100">
                    <ServiceIcon service={service} className="h-4 w-4" />
                  </span>
                )}
                <div className="min-w-0 flex-1">
                  <a
                    href={playlistURL(service, p.id)}
                    target="_blank"
                    rel="noreferrer"
                    className="group inline-flex max-w-full items-center gap-1 font-medium text-ink hover:underline"
                  >
                    <span className="truncate">{p.name}</span>
                    <ExternalLink className="h-3 w-3 shrink-0 text-gray-300 group-hover:text-gray-500" />
                  </a>
                  <div className="flex flex-wrap items-center gap-x-2 text-xs text-gray-500">
                    {p.detail && <span>{p.detail}</span>}
                    {paired && <span>⇄ synced with “{paired}”</span>}
                  </div>
                </div>
                <BackupState backup={backup} onBackUp={() => create.mutate(p)} pending={create.isPending && create.variables?.id === p.id} />
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}

function BackupState({ backup, onBackUp, pending }: { backup?: Backup; onBackUp: () => void; pending: boolean }) {
  if (!backup) {
    return (
      <Button size="sm" icon={<ArchiveRestore className="h-3.5 w-3.5" />} loading={pending} onClick={onBackUp}>
        Back up
      </Button>
    )
  }
  return (
    <Link to="/backups/$backupId" params={{ backupId: backup.id }} className="shrink-0" title="Open backup">
      <Pill tone={backup.last_error ? 'red' : backup.enabled ? 'green' : 'gray'}>
        <ShieldCheck className="h-3 w-3" />
        {backup.last_error ? 'Backup failing' : backup.enabled ? `Backed up · ${backup.snapshot_count}` : 'Backup paused'}
      </Pill>
    </Link>
  )
}
