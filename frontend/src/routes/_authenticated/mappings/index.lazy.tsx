import { createLazyFileRoute, Link } from '@tanstack/react-router'
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeftRight, Copy, ExternalLink, Pencil, Plus, Trash2 } from 'lucide-react'
import { mappingsAPI, type Mapping } from '../../../lib/api'
import { MappingSyncStatus, SyncNowButton } from '../../../components/MappingSync'
import { DeleteMappingDialog } from '../../../components/DeleteMappingDialog'
import { Card, Page, Pill, ServiceIcon } from '../../../components/ui'
import { buttonClass, timeAgo } from '../../../lib/format'
import { describeSchedule, nextRunLabel } from '../../../lib/schedule'
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
      className="group inline-flex min-w-0 items-center gap-2 font-medium text-gray-900 hover:underline"
    >
      <ServiceIcon service={service} className="h-5 w-5 shrink-0" />
      <span className="truncate">{name || id}</span>
      <ExternalLink className="h-3.5 w-3.5 shrink-0 text-gray-300 group-hover:text-gray-500" />
    </a>
  )
}

/** Queued work for one mapping, linking to the filtered queue and Review. */
function QueueSummary({ mapping }: { mapping: Mapping }) {
  const q = mapping.queue
  if (!q || q.pending + q.review + q.error === 0) return null
  return (
    <>
      <span className="text-gray-300">·</span>
      {q.pending > 0 && (
        <Link to="/sync-queue" search={{ status: 'pending', mapping: mapping.id }} className="hover:text-ink">
          {q.pending.toLocaleString()} queued
        </Link>
      )}
      {q.review > 0 && (
        <Link to="/review" search={{ mapping: mapping.id }} className="font-medium text-amber-700 hover:text-ink">
          {q.review} to review
        </Link>
      )}
      {q.error > 0 && (
        <Link to="/sync-queue" search={{ status: 'error', mapping: mapping.id }} className="text-youtube-700 hover:text-ink">
          {q.error} failed
        </Link>
      )}
    </>
  )
}

function MappingCard({ mapping, sync }: { mapping: Mapping; sync: ReturnType<typeof useSyncNow> }) {
  const [deleting, setDeleting] = useState(false)

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
            className={`${buttonClass.ghost} hover:text-youtube-700`}
            aria-label="Delete mapping"
            title="Delete"
            onClick={() => setDeleting(true)}
          >
            <Trash2 className="h-4 w-4" />
          </button>
        </div>
      </div>
      {deleting && <DeleteMappingDialog mapping={mapping} onClose={() => setDeleting(false)} />}

      <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-gray-500">
        <MappingSyncStatus mapping={mapping} />
        <span className="text-gray-300">·</span>
        <span title={mapping.schedule_tz ? `Times in ${mapping.schedule_tz}` : 'Server time'}>
          {describeSchedule(mapping)} · next {nextRunLabel(mapping.next_run_at)}
        </span>
        <span className="text-gray-300">·</span>
        {mapping.sync_tracks && <Pill>Tracks</Pill>}
        {mapping.sync_name && <Pill>Name</Pill>}
        {mapping.last_analysis_at && (
          <span title={`Playlist sizes at the last sync (${timeAgo(mapping.last_analysis_at)})`}>
            {(mapping.spotify_tracks_count ?? 0).toLocaleString()} on Spotify ·{' '}
            {(mapping.youtube_tracks_count ?? 0).toLocaleString()} on YouTube
          </span>
        )}
        <QueueSummary mapping={mapping} />
      </div>

      {mapping.last_sync_error && (
        <p className="mt-2 rounded-lg bg-youtube-50 px-3 py-2 text-xs text-youtube-700">{mapping.last_sync_error}</p>
      )}
    </Card>
  )
}

function MappingsList() {
  const sync = useSyncNow()
  const { data, isLoading, error } = useQuery({
    queryKey: ['mappings', 'list'],
    queryFn: () => mappingsAPI.getList({ per_page: 200 }),
    // Poll while a first sync runs in the background so its progress shows up.
    refetchInterval: (query) => (query.state.data?.items.some((m) => m.syncing) ? 5_000 : 60_000),
  })
  const mappings = data?.items ?? []

  return (
    <Page
      title="Mappings"
      description="Each mapping keeps one Spotify playlist and one YouTube playlist in sync."
      actions={
        <>
          <Link to="/mappings/mirror" className={buttonClass.secondary}>
            <Copy className="h-4 w-4" /> Mirror playlists
          </Link>
          <Link to="/mappings/new" className={buttonClass.primary}>
            <Plus className="h-4 w-4" /> Pair playlists
          </Link>
        </>
      }
    >
      {isLoading ? (
        <p className="text-sm text-gray-500" role="status">
          Loading mappings…
        </p>
      ) : error ? (
        <div className="rounded-lg border border-youtube-100 bg-youtube-50 px-4 py-3 text-sm text-youtube-700">
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
            <div className="rounded-lg border border-youtube-100 bg-youtube-50 px-4 py-3 text-sm text-youtube-700">
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
