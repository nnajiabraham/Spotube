import { RefreshCw } from 'lucide-react'
import type { Mapping } from '../lib/api/index'
import type { useSyncNow } from '../lib/useSyncNow'
import { Pill } from './ui'
import { buttonClass, fullTime, timeAgo } from '../lib/format'

export function MappingSyncStatus({ mapping }: { mapping: Mapping }) {
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
