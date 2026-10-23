import { RefreshCw } from 'lucide-react'
import type { Mapping } from '../lib/api/index'
import type { useSyncNow } from '../lib/useSyncNow'
import { Pill } from './ui'
import { buttonClass, fullTime, timeAgo } from '../lib/format'

// Statuses that pause part of a mapping until something changes.
const pausedStatus: Record<string, { tone: 'yellow' | 'red'; label: string }> = {
  waiting_quota: { tone: 'yellow', label: 'Waiting for quota' },
  playlist_missing: { tone: 'red', label: 'Playlist missing' },
  playlist_full: { tone: 'yellow', label: 'YouTube playlist full' },
}

export function MappingSyncStatus({ mapping }: { mapping: Mapping }) {
  if (mapping.syncing) {
    return (
      <Pill tone="blue">
        <RefreshCw className="h-3 w-3 animate-spin" aria-hidden="true" /> Syncing
      </Pill>
    )
  }
  if (!mapping.last_analysis_at) {
    return <Pill>Never synced</Pill>
  }
  const when = (
    <span className="text-xs text-gray-500" title={fullTime(mapping.last_analysis_at)}>
      {timeAgo(mapping.last_analysis_at)}
    </span>
  )
  if (mapping.last_sync_status === 'reconnect_required') {
    return (
      <span className="inline-flex flex-wrap items-center gap-2" title={mapping.last_sync_error}>
        <Pill tone="yellow">Reconnect needed</Pill>
        {when}
      </span>
    )
  }
  const paused = pausedStatus[mapping.last_sync_status ?? '']
  if (paused) {
    return (
      <span className="inline-flex flex-wrap items-center gap-2" title={mapping.last_sync_error}>
        <Pill tone={paused.tone}>{paused.label}</Pill>
        {when}
      </span>
    )
  }
  if (mapping.last_sync_status === 'error') {
    return (
      <span className="inline-flex flex-wrap items-center gap-2" title={mapping.last_sync_error}>
        <Pill tone="red">Failed</Pill>
        {when}
      </span>
    )
  }
  return (
    <span className="inline-flex flex-wrap items-center gap-2">
      <Pill tone="green">OK</Pill>
      {when}
    </span>
  )
}

export function SyncNowButton({
  mappingId,
  sync,
  compact = false,
}: {
  mappingId: string
  sync: ReturnType<typeof useSyncNow>
  compact?: boolean
}) {
  const running = sync.isPending && sync.variables === mappingId
  return (
    <button
      type="button"
      onClick={() => sync.mutate(mappingId)}
      disabled={sync.isPending}
      className={compact ? buttonClass.ghost : buttonClass.secondary}
      title="Sync now"
      aria-label="Sync now"
    >
      <RefreshCw className={`h-4 w-4 ${running ? 'animate-spin' : ''}`} />
      {!compact && (running ? 'Syncing…' : 'Sync now')}
    </button>
  )
}
