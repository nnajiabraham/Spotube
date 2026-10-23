import type { Backup } from '../lib/api/backups'
import { intervalLabel } from '../lib/backups'

export function BackupSchedule({ backup }: { backup: Backup }) {
  return <span>{backup.enabled ? intervalLabel(backup.interval_hours) : 'paused'}</span>
}

/** "+3 −1" against the previous snapshot; "first" for the first snapshot. */
export function ChangeCount({ added, removed }: { added?: number; removed?: number }) {
  if (added == null && removed == null) {
    return <span className="rounded bg-gray-100 px-1.5 py-0.5 text-[11px] text-gray-500">first</span>
  }
  if (!added && !removed) {
    return <span className="rounded bg-gray-100 px-1.5 py-0.5 text-[11px] text-gray-500">order or name</span>
  }
  return (
    <span className="inline-flex gap-1 text-[11px] tabular-nums">
      {!!added && <span className="rounded bg-spotify-50 px-1.5 py-0.5 text-spotify-700">+{added}</span>}
      {!!removed && <span className="rounded bg-youtube-50 px-1.5 py-0.5 text-youtube-700">−{removed}</span>}
    </span>
  )
}
