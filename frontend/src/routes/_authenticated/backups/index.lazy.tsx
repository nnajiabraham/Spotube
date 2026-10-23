import { createLazyFileRoute, Link } from '@tanstack/react-router'
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArchiveRestore, Plus, RefreshCw } from 'lucide-react'
import { backupsAPI, type Backup } from '../../../lib/api/backups'
import type { Service } from '../../../lib/api/tracks'
import { fullTime, serviceName, timeAgo } from '../../../lib/format'
import { PlaylistLibrary } from '../../../components/PlaylistLibrary'
import { BackupSchedule, ChangeCount } from '../../../components/BackupParts'
import { Button, Card, Dialog, EmptyState, Notice, Page, Pill, ServiceIcon, Tabs } from '../../../components/ui'

function AddPlaylistDialog({ onClose }: { onClose: () => void }) {
  const [service, setService] = useState<Service>('spotify')
  return (
    <Dialog
      title="Back up a playlist"
      subtitle="Spotube saves the track list now, then on the schedule you choose and whenever a sync reads it."
      onClose={onClose}
      size="lg"
      footer={
        <Button variant="primary" onClick={onClose}>
          Done
        </Button>
      }
    >
      <div className="mb-4">
        <Tabs<Service>
          label="Service"
          value={service}
          onChange={setService}
          items={[
            { value: 'spotify', label: <><ServiceIcon service="spotify" className="mr-1.5 h-3.5 w-3.5" />Spotify</> },
            { value: 'youtube', label: <><ServiceIcon service="youtube" className="mr-1.5 h-3.5 w-3.5" />YouTube</> },
          ]}
        />
      </div>
      <PlaylistLibrary key={service} service={service} compact />
    </Dialog>
  )
}

function BackupRow({ backup }: { backup: Backup }) {
  const queryClient = useQueryClient()
  const [result, setResult] = useState<string | null>(null)
  const run = useMutation({
    mutationFn: () => backupsAPI.run(backup.id),
    onSuccess: (r) =>
      setResult(r.changed ? `Saved a new snapshot (${r.snapshot.track_count} tracks).` : 'No changes since the last snapshot.'),
    onSettled: () => queryClient.invalidateQueries({ queryKey: ['backups'] }),
  })
  const latest = backup.latest_snapshot

  return (
    <li className="grid grid-cols-1 gap-3 px-4 py-4 md:grid-cols-[minmax(0,2fr)_minmax(0,1.3fr)_minmax(0,1fr)_auto] md:items-center">
      <div className="flex min-w-0 items-center gap-3">
        <ServiceIcon service={backup.service} className="h-5 w-5 shrink-0" />
        <div className="min-w-0">
          <Link
            to="/backups/$backupId"
            params={{ backupId: backup.id }}
            className="block truncate font-medium text-ink hover:underline"
          >
            {backup.playlist_name || backup.playlist_id}
          </Link>
          <div className="text-xs text-gray-500">
            {serviceName(backup.service)} · <BackupSchedule backup={backup} />
          </div>
        </div>
      </div>

      <div className="text-sm">
        {latest ? (
          <>
            <div className="flex items-center gap-2">
              <span className="tabular-nums text-ink">{latest.track_count} tracks</span>
              <ChangeCount added={latest.added} removed={latest.removed} />
            </div>
            <div className="text-xs text-gray-500" title={fullTime(latest.created)}>
              last change {timeAgo(latest.created)}
            </div>
          </>
        ) : (
          <span className="text-xs text-gray-500">No snapshot yet</span>
        )}
      </div>

      <div className="text-xs text-gray-500">
        <div>
          <span className="tabular-nums text-ink">{backup.snapshot_count}</span> / {backup.keep_snapshots} snapshots
        </div>
        <div title={fullTime(backup.last_checked_at)}>checked {timeAgo(backup.last_checked_at)}</div>
      </div>

      <div className="flex items-center gap-2 md:justify-end">
        {!backup.enabled && <Pill>Paused</Pill>}
        <Button size="sm" icon={<RefreshCw className="h-3.5 w-3.5" />} loading={run.isPending} onClick={() => run.mutate()}>
          Back up now
        </Button>
        <Link to="/backups/$backupId" params={{ backupId: backup.id }} className="text-sm font-medium text-gray-500 hover:text-ink">
          Open →
        </Link>
      </div>

      {(backup.last_error || run.error || result) && (
        <div className="md:col-span-4">
          {backup.last_error && !result && <Notice tone="error">Last check failed: {backup.last_error}</Notice>}
          {run.error && <Notice tone="error">{run.error.message}</Notice>}
          {result && (
            <Notice tone="success" onDismiss={() => setResult(null)}>
              {result}
            </Notice>
          )}
        </div>
      )}
    </li>
  )
}

function BackupsPage() {
  const [adding, setAdding] = useState(false)
  const { data, isLoading, error } = useQuery({
    queryKey: ['backups'],
    queryFn: () => backupsAPI.list().then((r) => r.items),
  })
  const items = data ?? []

  return (
    <Page
      title="Backups"
      description="Point-in-time copies of your playlists on either service. A snapshot is saved only when a playlist changes, so you can see what it held on any day and rebuild it."
      actions={
        <Button variant="primary" icon={<Plus className="h-4 w-4" />} onClick={() => setAdding(true)}>
          Add playlist
        </Button>
      }
    >
      {error && <Notice tone="error" className="mb-4">Couldn't load backups: {error.message}</Notice>}
      <Card className="overflow-hidden">
        {isLoading ? (
          <p className="px-4 py-10 text-center text-sm text-gray-500">Loading…</p>
        ) : items.length === 0 ? (
          <EmptyState
            icon={<ArchiveRestore />}
            title="No playlists backed up yet"
            action={
              <Button variant="primary" icon={<Plus className="h-4 w-4" />} onClick={() => setAdding(true)}>
                Add playlist
              </Button>
            }
          >
            Pick playlists from Spotify or YouTube. Spotube keeps their track lists, with each track's ID on both
            services, so a damaged playlist can be restored.
          </EmptyState>
        ) : (
          <ul className="divide-y divide-gray-200">
            {items.map((b) => (
              <BackupRow key={b.id} backup={b} />
            ))}
          </ul>
        )}
      </Card>

      <dl className="mt-6 grid gap-4 text-xs text-gray-500 sm:grid-cols-3">
        <div>
          <dt className="mb-1 font-semibold uppercase tracking-wider text-gray-400">When</dt>
          <dd>On each playlist's schedule, whenever a sync reads it (free), and when you press Back up now.</dd>
        </div>
        <div>
          <dt className="mb-1 font-semibold uppercase tracking-wider text-gray-400">What</dt>
          <dd>Name and tracks in order, each with its Spotify and YouTube IDs when Spotube knows both.</dd>
        </div>
        <div>
          <dt className="mb-1 font-semibold uppercase tracking-wider text-gray-400">Cost</dt>
          <dd>Spotify is free. A YouTube check costs about 3 quota units per 50 videos.</dd>
        </div>
      </dl>

      {adding && <AddPlaylistDialog onClose={() => setAdding(false)} />}
    </Page>
  )
}

export const Route = createLazyFileRoute('/_authenticated/backups/')({
  component: BackupsPage,
})
