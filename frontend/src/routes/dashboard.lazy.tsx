import { Link, createLazyFileRoute, useSearch } from '@tanstack/react-router'
import { useQuery } from '@tanstack/react-query'
import { ArrowLeftRight, Plus, RefreshCw } from 'lucide-react'
import { ConnectionCard } from '../components/ConnectionCard'
import { DashboardStatsCards } from '../components/DashboardStatsCards'
import { MappingSyncStatus, SyncNowButton } from '../components/MappingSync'
import { useSyncNow } from '../lib/useSyncNow'
import { Card, Page, SectionTitle, ServiceIcon } from '../components/ui'
import { buttonClass, fullTime, sectionLinkClass, timeAgo } from '../lib/format'
import { dashboardAPI, mappingsAPI } from '../lib/api/index'

const REFRESH_MS = 30_000

function AuthNotice() {
  const params = useSearch({ from: '/dashboard' }) as { spotify?: string; youtube?: string; message?: string }
  const service = params.spotify ? 'Spotify' : params.youtube ? 'YouTube' : null
  const result = params.spotify ?? params.youtube
  if (!service || !result) return null

  const ok = result === 'connected'
  return (
    <div
      role="status"
      className={`mb-6 rounded-lg border px-4 py-3 text-sm ${
        ok ? 'border-spotify/40 bg-spotify-50 text-spotify-700' : 'border-youtube/30 bg-youtube-50 text-youtube-700'
      }`}
    >
      <span className="font-medium">{service}:</span>{' '}
      {ok ? 'connected successfully.' : params.message || 'connection failed. Try again.'}
      {!ok && params.message?.includes('state mismatch') && (
        <span className="mt-1 block">Open the app at http://127.0.0.1:5173 (not localhost) and try again.</span>
      )}
    </div>
  )
}

function MappingsPanel() {
  const sync = useSyncNow()
  const { data, isLoading } = useQuery({
    queryKey: ['mappings', 'dashboard'],
    queryFn: () => mappingsAPI.getList({ per_page: 50 }),
    refetchInterval: REFRESH_MS,
  })
  const mappings = data?.items ?? []

  return (
    <Card>
      {isLoading ? (
        <div className="p-6 text-sm text-gray-500">Loading mappings…</div>
      ) : mappings.length === 0 ? (
        <div className="p-8 text-center">
          <p className="text-sm text-gray-500">No playlists are paired yet.</p>
          <Link to="/mappings/new" className={`${buttonClass.primary} mt-4`}>
            <Plus className="h-4 w-4" /> Pair two playlists
          </Link>
        </div>
      ) : (
        <ul className="divide-y divide-gray-100">
          {mappings.map((m) => (
            <li key={m.id} className="flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3">
              <Link
                to="/mappings/$mappingId/edit"
                params={{ mappingId: m.id }}
                className="flex min-w-0 flex-1 items-center gap-2 text-sm font-medium text-gray-900 hover:underline"
              >
                <ServiceIcon service="spotify" />
                <span className="truncate">{m.spotify_playlist_name || m.spotify_playlist_id}</span>
                <ArrowLeftRight className="h-3.5 w-3.5 shrink-0 text-gray-400" />
                <ServiceIcon service="youtube" />
                <span className="truncate">{m.youtube_playlist_name || m.youtube_playlist_id}</span>
              </Link>
              <span className="text-xs text-gray-500">every {m.interval_minutes} min</span>
              <MappingSyncStatus mapping={m} />
              <SyncNowButton mappingId={m.id} sync={sync} compact />
            </li>
          ))}
        </ul>
      )}
    </Card>
  )
}

const levelDot: Record<string, string> = {
  info: 'bg-gray-300',
  warn: 'bg-amber-400',
  error: 'bg-youtube',
}

function Dashboard() {
  const { data: stats, isLoading, dataUpdatedAt, refetch, isFetching } = useQuery({
    queryKey: ['dashboard-stats'],
    queryFn: dashboardAPI.getStats,
    refetchInterval: REFRESH_MS,
  })

  return (
    <Page
      title="Dashboard"
      description="Keeps paired Spotify and YouTube playlists in sync."
      actions={
        <>
          {dataUpdatedAt > 0 && (
            <span className="text-xs text-gray-400">Updated {timeAgo(dataUpdatedAt / 1000)}</span>
          )}
          <button type="button" onClick={() => refetch()} className={buttonClass.secondary} disabled={isFetching}>
            <RefreshCw className={`h-4 w-4 ${isFetching ? 'animate-spin' : ''}`} /> Refresh
          </button>
        </>
      }
    >
      <AuthNotice />

      <section className="mb-8">
        <SectionTitle>Accounts</SectionTitle>
        <div className="grid gap-3 md:grid-cols-2">
          <ConnectionCard service="spotify" />
          <ConnectionCard service="youtube" />
        </div>
      </section>

      <section className="mb-8">
        <SectionTitle
          action={
            <Link to="/sync-queue" className={sectionLinkClass}>
              Open queue →
            </Link>
          }
        >
          Sync queue
        </SectionTitle>
        <DashboardStatsCards stats={stats} isLoading={isLoading} />
      </section>

      <div className="grid gap-8 lg:grid-cols-5">
        <section className="lg:col-span-3">
          <SectionTitle
            action={
              <Link to="/mappings" className={sectionLinkClass}>
                Manage →
              </Link>
            }
          >
            Paired playlists
          </SectionTitle>
          <MappingsPanel />
        </section>

        <section className="lg:col-span-2">
          <SectionTitle
            action={
              <Link to="/logs" className={sectionLinkClass}>
                All activity →
              </Link>
            }
          >
            Recent activity
          </SectionTitle>
          <Card>
            {stats?.recent_runs?.length ? (
              <ul className="divide-y divide-gray-100">
                {stats.recent_runs.slice(0, 8).map((run, i) => (
                  <li key={`${run.timestamp}-${i}`} className="flex gap-3 px-4 py-2.5">
                    <span className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${levelDot[run.level] ?? 'bg-gray-300'}`} />
                    <div className="min-w-0 flex-1">
                      <p className="line-clamp-2 text-sm text-gray-700" title={run.message}>
                        {run.message}
                      </p>
                      <p className="text-xs text-gray-400" title={fullTime(run.timestamp)}>
                        {run.job_type} · {timeAgo(run.timestamp)}
                      </p>
                    </div>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="p-6 text-sm text-gray-500">{isLoading ? 'Loading…' : 'Nothing has run yet.'}</p>
            )}
          </Card>
        </section>
      </div>
    </Page>
  )
}

export const Route = createLazyFileRoute('/dashboard')({
  component: Dashboard,
})
