import { createLazyFileRoute, Link } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ArrowLeftRight, ExternalLink, Pencil, Plus, Trash2 } from 'lucide-react'
import { api, type Mapping } from '../../../lib/api'
import { MappingSyncStatus, SyncNowButton } from '../../../components/MappingSync'
import { Card, Page, Pill, ServiceIcon } from '../../../components/ui'
import { buttonClass, timeAgo } from '../../../lib/format'
import { useSyncNow } from '../../../lib/useSyncNow'

const playlistURL = {
  spotify: (id: string) => `https://open.spotify.com/playlist/${id}`,
  youtube: (id: string) => `https://www.youtube.com/playlist?list=${id}`,
}

function PlaylistName({ service, id, name }: { service: 'spotify' | 'youtube'; id: string; name: string }) {
  return (
    <a
      href={playlistURL[service](id)}
      target="_blank"
      rel="noreferrer"
      className="group inline-flex min-w-0 items-center gap-2 font-medium text-gray-900 hover:text-indigo-700"
    >
      <ServiceIcon service={service} className="h-5 w-5 shrink-0" />
      <span className="truncate">{name || id}</span>
      <ExternalLink className="h-3.5 w-3.5 shrink-0 text-gray-300 group-hover:text-indigo-500" />
    </a>
  )
}

function nextSyncLabel(mapping: Mapping): string {
  if (!mapping.last_analysis_at) return 'within a minute'
  const due = mapping.last_analysis_at + mapping.interval_minutes * 60
  const minutes = Math.ceil((due - Date.now() / 1000) / 60)
  return minutes <= 1 ? 'within a minute' : `in ${minutes} min`
}

function MappingCard({ mapping, sync }: { mapping: Mapping; sync: ReturnType<typeof useSyncNow> }) {
  const queryClient = useQueryClient()
  const remove = useMutation({
    mutationFn: api.deleteMapping,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['mappings'] }),
  })

  return (
    <Card className="p-4">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <PlaylistName service="spotify" id={mapping.spotify_playlist_id} name={mapping.spotify_playlist_name} />
        <ArrowLeftRight className="h-4 w-4 shrink-0 text-gray-400" />
        <PlaylistName service="youtube" id={mapping.youtube_playlist_id} name={mapping.youtube_playlist_name} />
        <div className="ml-auto flex items-center gap-1">
          <SyncNowButton mappingId={mapping.id} sync={sync} />
          <Link
            to="/mappings/$mappingId/edit"
            params={{ mappingId: mapping.id }}
            className={buttonClass.ghost}
            aria-label="Edit mapping"
            title="Edit"
          >
            <Pencil className="h-4 w-4" />
          </Link>
          <button
            type="button"
            className={`${buttonClass.ghost} hover:text-red-600`}
            aria-label="Delete mapping"
            title="Delete"
            disabled={remove.isPending}
            onClick={() => {
              if (confirm('Stop syncing these playlists? The playlists themselves are not changed.')) {
                remove.mutate(mapping.id)
              }
            }}
          >
            <Trash2 className="h-4 w-4" />
          </button>
        </div>
      </div>

      <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-gray-500">
        <MappingSyncStatus mapping={mapping} />
        <span className="text-gray-300">·</span>
        <span>
          every {mapping.interval_minutes} min · next {nextSyncLabel(mapping)}
        </span>
        <span className="text-gray-300">·</span>
        {mapping.sync_tracks && <Pill>Tracks</Pill>}
        {mapping.sync_name && <Pill>Name</Pill>}
        {mapping.tracks_count > 0 && (
          <span title={`Track count at last sync (${timeAgo(mapping.last_analysis_at)})`}>
            {mapping.tracks_count} tracks
          </span>
        )}
      </div>

      {mapping.last_sync_error && (
        <p className="mt-2 rounded-lg bg-red-50 px-3 py-2 text-xs text-red-700">{mapping.last_sync_error}</p>
      )}
    </Card>
  )
}

function MappingsList() {
  const sync = useSyncNow()
  const { data, isLoading, error } = useQuery({
    queryKey: ['mappings'],
    queryFn: () => api.getMappings(),
  })
  const mappings = data?.items ?? []

  return (
    <Page
      title="Mappings"
      description="Each mapping keeps one Spotify playlist and one YouTube playlist in sync."
      actions={
        <Link to="/mappings/new" className={buttonClass.primary}>
          <Plus className="h-4 w-4" /> Pair playlists
        </Link>
      }
    >
      {isLoading ? (
        <p className="text-sm text-gray-500" role="status">
          Loading mappings…
        </p>
      ) : error ? (
        <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800">
          Error loading mappings: {error.message}
        </div>
      ) : mappings.length === 0 ? (
        <Card className="p-10 text-center">
          <p className="text-gray-500">No mappings yet. Pair a Spotify and a YouTube playlist to start syncing.</p>
          <Link to="/mappings/new" className={`${buttonClass.primary} mt-4`}>
            <Plus className="h-4 w-4" /> Pair playlists
          </Link>
        </Card>
      ) : (
        <div className="space-y-3">
          {sync.error && (
            <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800">
              Sync failed: {sync.error.message}
            </div>
          )}
          {mappings.map((mapping) => (
            <MappingCard key={mapping.id} mapping={mapping} sync={sync} />
          ))}
        </div>
      )}
    </Page>
  )
}

export const Route = createLazyFileRoute('/_authenticated/mappings/')({
  component: MappingsList,
})
