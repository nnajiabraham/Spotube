// Backup schedule options shared by the Backups pages.

export const BACKUP_INTERVALS = [
  { hours: 6, label: 'every 6 hours' },
  { hours: 12, label: 'every 12 hours' },
  { hours: 24, label: 'daily' },
  { hours: 72, label: 'every 3 days' },
  { hours: 168, label: 'weekly' },
]

export const KEEP_OPTIONS = [10, 30, 60, 120]

export function intervalLabel(hours: number): string {
  return BACKUP_INTERVALS.find((i) => i.hours === hours)?.label ?? `every ${hours} h`
}
