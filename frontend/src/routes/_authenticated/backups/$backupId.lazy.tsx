import { createLazyFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Download, ExternalLink, History, RefreshCw, RotateCcw, Trash2 } from 'lucide-react'
import {
  backupsAPI,
  estimateRestoreCost,
  playlistURL,
  type Backup,
  type RestoreMode,
  type RestoreResult,
  type Snapshot,
  type SnapshotSummary,
  type SnapshotTrack,
} from '../../../lib/api/backups'
import { dashboardAPI } from '../../../lib/api/dashboard'
import type { Service } from '../../../lib/api/tracks'
import { buttonClass, formatDuration, fullTime, inputClass, serviceName, timeAgo, trackURL } from '../../../lib/format'
import { ChangeCount } from '../../../components/BackupParts'
import { BACKUP_INTERVALS, KEEP_OPTIONS } from '../../../lib/backups'
import { Button, Card, Dialog, EmptyState, Notice, Page, Pill, SectionTitle, ServiceIcon } from '../../../components/ui'

const sourceLabel: Record<SnapshotSummary['source'], string> = {
  initial: 'first backup',
  scheduled: 'scheduled',
  manual: 'manual',
  sync: 'during sync',
}

function when(epoch: number) {
  return new Date(epoch * 1000).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

function Timeline({ items, selected, onSelect }: { items: SnapshotSummary[]; selected?: string; onSelect: (id: string) => void }) {
  return (
    <ol className="relative space-y-1 before:absolute before:bottom-3 before:left-[11px] before:top-3 before:w-px before:bg-gray-200">
      {items.map((s, i) => {
        const active = s.id === selected
        return (
          <li key={s.id}>
            <button
              type="button"
              onClick={() => onSelect(s.id)}
              aria-current={active || undefined}
              className={`relative flex w-full items-start gap-3 rounded-lg px-1.5 py-2 text-left transition-colors ${
                active ? 'bg-gray-100' : 'hover:bg-gray-50'
              }`}
            >
              <span
                className={`relative z-10 mt-1 h-2.5 w-2.5 shrink-0 translate-x-[3px] rounded-full ring-4 ring-white ${
                  i === 0 ? 'bg-spotify' : active ? 'bg-ink' : 'bg-gray-300'
                }`}
              />
              <span className="min-w-0 flex-1">
                <span className="flex items-center justify-between gap-2">
                  <span className="text-sm font-medium text-ink">{when(s.created)}</span>
                  <ChangeCount added={s.added} removed={s.removed} />
                </span>
                <span className="block text-xs text-gray-500">
                  {s.track_count} tracks · {sourceLabel[s.source]}
                  {i === 0 && ' · current'}
                </span>
                {s.confirmed_at > s.created + 60 && (
                  <span className="block text-xs text-gray-400" title={fullTime(s.confirmed_at)}>
                    unchanged until {when(s.confirmed_at)}
                  </span>
                )}
              </span>
            </button>
          </li>
        )
      })}
    </ol>
  )
}

function IDCell({ service, id }: { service: Service; id?: string }) {
  if (!id) return <span className="text-gray-300">—</span>
  return (
    <a
      href={trackURL(service, id) ?? undefined}
      target="_blank"
      rel="noreferrer"
      className="inline-flex items-center gap-1 whitespace-nowrap text-[11px] tabular-nums text-gray-500 hover:text-ink hover:underline"
      title={`Open on ${serviceName(service)}`}
    >
      <ServiceIcon service={service} className="h-3 w-3" />
      {id}
    </a>
  )
}

function TrackTable({ tracks, added }: { tracks: SnapshotTrack[]; added: Set<string> }) {
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full text-sm">
        <thead className="border-b border-gray-200 text-left text-[11px] uppercase tracking-wider text-gray-400">
          <tr>
            <th className="w-10 px-3 py-2 text-right font-medium">#</th>
            <th className="px-3 py-2 font-medium">Title</th>
            <th className="hidden px-3 py-2 font-medium sm:table-cell">Length</th>
            <th className="px-3 py-2 font-medium">Spotify</th>
            <th className="px-3 py-2 font-medium">YouTube</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-gray-100">
          {tracks.map((t) => (
            <tr key={`${t.position}-${t.id}`} className={added.has(t.id) ? 'bg-spotify-50/60' : ''}>
              <td className="px-3 py-2 text-right text-xs tabular-nums text-gray-400">{t.position + 1}</td>
              <td className="max-w-xs px-3 py-2">
                <div className="truncate text-ink">{t.title || t.raw_title || t.id}</div>
                <div className="truncate text-xs text-gray-500">{t.artist || t.channel || 'Unknown artist'}</div>
              </td>
              <td className="hidden px-3 py-2 text-xs tabular-nums text-gray-500 sm:table-cell">{formatDuration(t.duration_ms)}</td>
              <td className="px-3 py-2">
                <IDCell service="spotify" id={t.spotify_id} />
              </td>
              <td className="px-3 py-2">
                <IDCell service="youtube" id={t.youtube_id} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

type Choice = { mode: RestoreMode; service: Service }

function RestoreDialog({ backup, snapshot, onClose }: { backup: Backup; snapshot: Snapshot; onClose: () => void }) {
  const queryClient = useQueryClient()
  const other: Service = backup.service === 'spotify' ? 'youtube' : 'spotify'
  const [choice, setChoice] = useState<Choice>({ mode: 'missing', service: backup.service })
  const defaultName = `${snapshot.playlist_name || backup.playlist_name} (restored ${new Date(snapshot.created * 1000).toISOString().slice(0, 10)})`
  const [name, setName] = useState(defaultName)
  const stats = useQuery({ queryKey: ['dashboard-stats'], queryFn: dashboardAPI.getStats })
  const quotaLeft = stats.data ? stats.data.youtube_quota.limit - stats.data.youtube_quota.used : undefined

  const withCounterpart = snapshot.tracks.filter((t) => (other === 'spotify' ? t.spotify_id : t.youtube_id)).length
  const restore = useMutation({
    mutationFn: () =>
      backupsAPI.restore(snapshot.id, {
        mode: choice.mode,
        service: choice.service,
        name: choice.mode === 'new' ? name.trim() || undefined : undefined,
      }),
    onSettled: () => queryClient.invalidateQueries({ queryKey: ['dashboard-stats'] }),
  })

  const trackCount = choice.service === backup.service ? snapshot.tracks.length : withCounterpart
  const cost = estimateRestoreCost(choice.service, choice.mode, trackCount)
  const options: Array<Choice & { title: string; body: string; disabled?: boolean }> = [
    {
      mode: 'missing',
      service: backup.service,
      title: `Put back missing tracks in “${backup.playlist_name}”`,
      body: 'Adds tracks from this snapshot that are no longer in the playlist, at the end. Nothing is removed.',
    },
    {
      mode: 'new',
      service: backup.service,
      title: `New ${serviceName(backup.service)} playlist`,
      body: `A copy with all ${snapshot.tracks.length} tracks in their original order${backup.service === 'youtube' ? ', set to private' : '. Spotify ignores the private flag for apps, so it shows as public'}.`,
    },
    {
      mode: 'new',
      service: other,
      title: `New ${serviceName(other)} playlist`,
      body: `${withCounterpart} of ${snapshot.tracks.length} tracks have a known ${serviceName(other)} ID; the rest are listed afterwards.`,
      disabled: withCounterpart === 0,
    },
  ]
  const result: RestoreResult | undefined = restore.data

  return (
    <Dialog
      title="Restore snapshot"
      subtitle={`${snapshot.track_count} tracks · taken ${fullTime(snapshot.created)}`}
      onClose={onClose}
      footer={
        result ? (
          <Button variant="primary" onClick={onClose}>
            Done
          </Button>
        ) : (
          <>
            <Button onClick={onClose}>Cancel</Button>
            <Button
              variant="primary"
              icon={<RotateCcw className="h-4 w-4" />}
              loading={restore.isPending}
              disabled={quotaLeft != null && cost > quotaLeft}
              onClick={() => restore.mutate()}
            >
              Restore
            </Button>
          </>
        )
      }
    >
      {result ? (
        <div className="space-y-3">
          <Notice tone={result.error ? 'warning' : 'success'}>
            Added {result.added} track{result.added === 1 ? '' : 's'} to{' '}
            <a href={result.playlist_url} target="_blank" rel="noreferrer" className="font-medium underline">
              {result.playlist_name}
            </a>
            {result.already_present > 0 && <> · {result.already_present} were already there</>}.
            {result.error && <> Stopped early: {result.error}</>}
          </Notice>
          {result.no_counterpart.length > 0 && (
            <div>
              <p className="mb-1 text-xs text-gray-500">
                No known {serviceName(result.service)} ID for these, so they weren't added:
              </p>
              <ul className="max-h-48 space-y-0.5 overflow-y-auto text-xs text-gray-600">
                {result.no_counterpart.map((t) => (
                  <li key={t.position}>
                    {t.title} {t.artist && <span className="text-gray-400">— {t.artist}</span>}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      ) : (
        <div className="space-y-4">
          <div role="radiogroup" aria-label="How to restore" className="space-y-2">
            {options.map((o) => {
              const active = o.mode === choice.mode && o.service === choice.service
              return (
                <button
                  key={`${o.mode}-${o.service}`}
                  type="button"
                  role="radio"
                  aria-checked={active}
                  disabled={o.disabled}
                  onClick={() => setChoice({ mode: o.mode, service: o.service })}
                  className={`flex w-full items-start gap-3 rounded-lg border px-3 py-2.5 text-left transition-colors disabled:cursor-not-allowed disabled:opacity-50 ${
                    active ? 'border-ink bg-gray-50' : 'border-gray-200 hover:border-gray-300'
                  }`}
                >
                  <span
                    className={`mt-1 h-3 w-3 shrink-0 rounded-full border ${active ? 'border-ink bg-ink ring-2 ring-white ring-inset' : 'border-gray-300'}`}
                  />
                  <span className="min-w-0">
                    <span className="flex items-center gap-1.5 font-medium text-ink">
                      <ServiceIcon service={o.service} className="h-3.5 w-3.5" />
                      {o.title}
                    </span>
                    <span className="block text-xs text-gray-500">{o.body}</span>
                  </span>
                </button>
              )
            })}
          </div>

          {choice.mode === 'new' && (
            <label className="block">
              <span className="mb-1 block text-xs font-medium text-gray-500">Playlist name</span>
              <input className={inputClass} value={name} onChange={(e) => setName(e.target.value)} />
            </label>
          )}

          <p className="text-xs text-gray-500">
            {choice.service === 'youtube' ? (
              <>
                Uses about <span className="font-medium text-ink">{cost.toLocaleString()}</span> YouTube quota units
                {quotaLeft != null && <> of {quotaLeft.toLocaleString()} left today</>} (50 per track
                {choice.mode === 'missing' ? ' put back' : ', plus 50 for the playlist'}).
                {quotaLeft != null && cost > quotaLeft && (
                  <span className="block text-youtube-700">Not enough quota left today. Try after midnight Pacific.</span>
                )}
              </>
            ) : (
              <>Spotify restores are free.</>
            )}
          </p>
          {restore.error && <Notice tone="error">{restore.error.message}</Notice>}
        </div>
      )}
    </Dialog>
  )
}

function Settings({ backup }: { backup: Backup }) {
  const queryClient = useQueryClient()
  const update = useMutation({
    mutationFn: (data: Parameters<typeof backupsAPI.update>[1]) => backupsAPI.update(backup.id, data),
    onSettled: () => queryClient.invalidateQueries({ queryKey: ['backups'] }),
  })
  const select = `${inputClass} !w-auto !py-1 !text-xs`
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-gray-500">
      <label className="inline-flex cursor-pointer items-center gap-2">
        <input
          type="checkbox"
          className="h-3.5 w-3.5 accent-ink"
          checked={backup.enabled}
          onChange={(e) => update.mutate({ enabled: e.target.checked })}
        />
        Scheduled backups
      </label>
      <label className="inline-flex items-center gap-2">
        Check
        <select
          className={select}
          value={backup.interval_hours}
          disabled={!backup.enabled}
          onChange={(e) => update.mutate({ interval_hours: Number(e.target.value) })}
        >
          {BACKUP_INTERVALS.map((i) => (
            <option key={i.hours} value={i.hours}>
              {i.label}
            </option>
          ))}
        </select>
      </label>
      <label className="inline-flex items-center gap-2">
        Keep
        <select className={select} value={backup.keep_snapshots} onChange={(e) => update.mutate({ keep_snapshots: Number(e.target.value) })}>
          {KEEP_OPTIONS.map((n) => (
            <option key={n} value={n}>
              {n} snapshots
            </option>
          ))}
        </select>
      </label>
      {update.error && <span className="text-youtube-700">{update.error.message}</span>}
    </div>
  )
}

function BackupDetail() {
  const { backupId } = Route.useParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [selected, setSelected] = useState<string>()
  const [restoring, setRestoring] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)

  const backup = useQuery({
    queryKey: ['backups', backupId],
    queryFn: () => backupsAPI.get(backupId),
  })
  const snapshots = useQuery({
    queryKey: ['backups', backupId, 'snapshots'],
    queryFn: () => backupsAPI.snapshots(backupId).then((r) => r.items),
  })
  const selectedId = selected ?? snapshots.data?.[0]?.id
  const snapshot = useQuery({
    queryKey: ['backup-snapshot', selectedId],
    queryFn: () => backupsAPI.snapshot(selectedId!),
    enabled: !!selectedId,
  })

  const run = useMutation({
    mutationFn: () => backupsAPI.run(backupId),
    onSuccess: (r) => {
      setNotice(r.changed ? `Saved a new snapshot with ${r.snapshot.track_count} tracks.` : 'No changes since the last snapshot; marked it as still current.')
      setSelected(undefined)
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: ['backups'] }),
  })
  const remove = useMutation({
    mutationFn: () => backupsAPI.remove(backupId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['backups'] })
      navigate({ to: '/backups' })
    },
  })

  if (backup.error) {
    return (
      <Page title="Backup">
        <Notice tone="error">{backup.error.message}</Notice>
      </Page>
    )
  }
  const b = backup.data
  const snap = snapshot.data
  const added = new Set(snap?.added_tracks.map((t) => t.id))

  return (
    <Page
      title={b ? b.playlist_name || b.playlist_id : 'Backup'}
      description={
        b && (
          <span className="inline-flex flex-wrap items-center gap-x-2">
            <Link to="/backups" className="inline-flex items-center gap-1 hover:text-ink">
              <ArrowLeft className="h-3.5 w-3.5" />
              Backups
            </Link>
            <span className="text-gray-300">/</span>
            <ServiceIcon service={b.service} className="h-3.5 w-3.5" />
            <a href={playlistURL(b.service, b.playlist_id)} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1 hover:text-ink hover:underline">
              {b.playlist_id}
              <ExternalLink className="h-3 w-3" />
            </a>
            <span className="text-gray-300">·</span>
            <span title={fullTime(b.last_checked_at)}>checked {timeAgo(b.last_checked_at)}</span>
          </span>
        )
      }
      actions={
        b && (
          <>
            <Button icon={<RefreshCw className="h-4 w-4" />} loading={run.isPending} onClick={() => run.mutate()}>
              Back up now
            </Button>
            <Button
              variant="ghost"
              icon={<Trash2 className="h-4 w-4" />}
              loading={remove.isPending}
              onClick={() => {
                if (confirm('Stop backing up this playlist and delete all of its snapshots? The playlist itself is not changed.')) {
                  remove.mutate()
                }
              }}
            >
              Delete
            </Button>
          </>
        )
      }
    >
      {b && (
        <Card className="mb-4 px-4 py-3">
          <Settings backup={b} />
        </Card>
      )}
      {b?.last_error && <Notice tone="error" className="mb-4">Last check failed: {b.last_error}</Notice>}
      {notice && (
        <Notice tone="success" className="mb-4" onDismiss={() => setNotice(null)}>
          {notice}
        </Notice>
      )}
      {(run.error || remove.error) && <Notice tone="error" className="mb-4">{(run.error ?? remove.error)?.message}</Notice>}

      <div className="grid gap-4 lg:grid-cols-[18rem_minmax(0,1fr)]">
        <Card className="h-fit p-3">
          <SectionTitle>
            <span className="inline-flex items-center gap-1.5">
              <History className="h-3.5 w-3.5" />
              Snapshots {snapshots.data && <span className="text-gray-400">{snapshots.data.length}</span>}
            </span>
          </SectionTitle>
          {snapshots.isLoading ? (
            <p className="py-4 text-sm text-gray-500">Loading…</p>
          ) : snapshots.data?.length ? (
            <Timeline items={snapshots.data} selected={selectedId} onSelect={setSelected} />
          ) : (
            <p className="py-4 text-sm text-gray-500">No snapshots yet. Press Back up now.</p>
          )}
        </Card>

        <Card className="min-w-0 overflow-hidden">
          {!snap ? (
            snapshot.isLoading ? (
              <p className="px-4 py-10 text-center text-sm text-gray-500">Loading…</p>
            ) : (
              <EmptyState icon={<History />} title="Pick a snapshot" />
            )
          ) : (
            <>
              <div className="flex flex-wrap items-start justify-between gap-3 border-b border-gray-200 px-4 py-3">
                <div>
                  <div className="font-medium text-ink">{snap.playlist_name}</div>
                  <div className="text-xs text-gray-500">
                    {snap.track_count} tracks ·{' '}
                    {formatDuration(snap.tracks.reduce((sum, t) => sum + (t.duration_ms ?? 0), 0)) || 'length unknown'} · taken{' '}
                    {fullTime(snap.created)}
                    {snap.confirmed_at > snap.created + 60 && <> · unchanged until {fullTime(snap.confirmed_at)}</>}
                  </div>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <a className={`${buttonClass.secondary} !px-2.5 !py-1.5 !text-xs`} href={backupsAPI.exportURL(snap.id, 'json')} download>
                    <Download className="h-3.5 w-3.5" /> JSON
                  </a>
                  <a className={`${buttonClass.secondary} !px-2.5 !py-1.5 !text-xs`} href={backupsAPI.exportURL(snap.id, 'csv')} download>
                    <Download className="h-3.5 w-3.5" /> CSV
                  </a>
                  <Button size="sm" variant="primary" icon={<RotateCcw className="h-3.5 w-3.5" />} onClick={() => setRestoring(true)}>
                    Restore…
                  </Button>
                </div>
              </div>

              {snap.has_previous && (snap.added_tracks.length > 0 || snap.removed_tracks.length > 0) && (
                <div className="grid gap-3 border-b border-gray-200 bg-gray-50 px-4 py-3 text-xs sm:grid-cols-2">
                  <div>
                    <div className="mb-1 font-semibold uppercase tracking-wider text-spotify-700">
                      Added since previous · {snap.added_tracks.length}
                    </div>
                    {snap.added_tracks.length ? (
                      <ul className="space-y-0.5 text-gray-600">
                        {snap.added_tracks.map((t) => (
                          <li key={`a-${t.position}`} className="truncate">
                            + {t.title || t.id} {t.artist && <span className="text-gray-400">— {t.artist}</span>}
                          </li>
                        ))}
                      </ul>
                    ) : (
                      <span className="text-gray-400">none</span>
                    )}
                  </div>
                  <div>
                    <div className="mb-1 font-semibold uppercase tracking-wider text-youtube-700">
                      Removed since previous · {snap.removed_tracks.length}
                    </div>
                    {snap.removed_tracks.length ? (
                      <ul className="space-y-0.5 text-gray-600">
                        {snap.removed_tracks.map((t) => (
                          <li key={`r-${t.position}`} className="truncate">
                            − {t.title || t.id} {t.artist && <span className="text-gray-400">— {t.artist}</span>}
                          </li>
                        ))}
                      </ul>
                    ) : (
                      <span className="text-gray-400">none</span>
                    )}
                  </div>
                </div>
              )}

              {snap.tracks.length === 0 ? (
                <EmptyState icon={<History />} title="The playlist was empty" />
              ) : (
                <TrackTable tracks={snap.tracks} added={added} />
              )}
              <div className="flex flex-wrap items-center gap-2 border-t border-gray-200 px-4 py-2.5 text-xs text-gray-500">
                <Pill tone="green">{snap.tracks.filter((t) => t.spotify_id).length} with Spotify ID</Pill>
                <Pill tone="red">{snap.tracks.filter((t) => t.youtube_id).length} with YouTube ID</Pill>
                <span>IDs on the other service come from Matches at the time of the snapshot.</span>
              </div>
            </>
          )}
        </Card>
      </div>

      {restoring && b && snap && <RestoreDialog backup={b} snapshot={snap} onClose={() => setRestoring(false)} />}
    </Page>
  )
}

export const Route = createLazyFileRoute('/_authenticated/backups/$backupId')({
  component: BackupDetail,
})
