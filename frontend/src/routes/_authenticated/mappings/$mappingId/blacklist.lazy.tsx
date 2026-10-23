import { createLazyFileRoute, Link, useParams } from '@tanstack/react-router'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, ArrowRight } from 'lucide-react'
import { blacklistAPI, mappingsAPI } from '../../../../lib/api/index'
import { Card, Page, ServiceIcon } from '../../../../components/ui'
import { buttonClass, fullTime, timeAgo } from '../../../../lib/format'

function BlacklistPage() {
  const { mappingId } = useParams({ from: '/_authenticated/mappings/$mappingId/blacklist' })
  const queryClient = useQueryClient()

  const { data, isLoading, error } = useQuery({
    queryKey: ['blacklist', mappingId],
    queryFn: () => blacklistAPI.getList({ mapping_id: mappingId, per_page: 100 }),
  })
  const { data: mapping } = useQuery({
    queryKey: ['mappings', mappingId],
    queryFn: () => mappingsAPI.getOne(mappingId),
  })

  const unblock = useMutation({
    mutationFn: blacklistAPI.delete,
    onSuccess: () => {
      for (const key of ['blacklist', 'sync-items', 'dashboard-stats']) {
        queryClient.invalidateQueries({ queryKey: [key] })
      }
    },
  })

  const entries = data?.items ?? []
  const pair = mapping
    ? `${mapping.spotify_playlist_name || mapping.spotify_playlist_id} ⇄ ${mapping.youtube_playlist_name || mapping.youtube_playlist_id}`
    : ''

  return (
    <Page
      title="Blocked tracks"
      width="narrow"
      description={
        <>
          Tracks that will never be copied for <span className="font-medium text-gray-700">{pair}</span>. Unblock one to
          queue it again.
        </>
      }
      actions={
        <Link to="/mappings/$mappingId/edit" params={{ mappingId }} className={buttonClass.ghost}>
          <ArrowLeft className="h-4 w-4" /> Back to mapping
        </Link>
      }
    >
      {isLoading ? (
        <p className="text-sm text-gray-500" role="status">
          Loading…
        </p>
      ) : error ? (
        <div className="rounded-lg border border-youtube-100 bg-youtube-50 px-4 py-3 text-sm text-youtube-700">
          Error loading blocked tracks: {error.message}
        </div>
      ) : entries.length === 0 ? (
        <Card className="p-8 text-center text-sm text-gray-500">
          No blocked tracks. Use “Never sync this track” on a Sync Queue item to block one.
        </Card>
      ) : (
        <Card>
          <ul className="divide-y divide-gray-100">
            {entries.map((entry) => {
              const source = entry.service === 'youtube' ? 'spotify' : 'youtube'
              return (
                <li key={entry.id} className="flex flex-wrap items-center gap-3 px-4 py-3">
                  <div className="flex items-center gap-1.5">
                    <ServiceIcon service={source} />
                    <ArrowRight className="h-3.5 w-3.5 text-gray-400" />
                    <ServiceIcon service={entry.service} />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-sm font-medium text-gray-900">{entry.track_title || entry.track_id}</div>
                    <div className="text-xs text-gray-500">
                      {[entry.track_artist, entry.reason, `blocked ${timeAgo(entry.created)}`].filter(Boolean).join(' · ')}
                    </div>
                  </div>
                  <button
                    type="button"
                    className={buttonClass.secondary}
                    title={fullTime(entry.created)}
                    disabled={unblock.isPending}
                    onClick={() => unblock.mutate(entry.id)}
                  >
                    Unblock
                  </button>
                </li>
              )
            })}
          </ul>
        </Card>
      )}
    </Page>
  )
}

export const Route = createLazyFileRoute('/_authenticated/mappings/$mappingId/blacklist')({
  component: BlacklistPage,
})
