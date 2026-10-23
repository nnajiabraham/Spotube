import type { MappingSchedule } from './api/types'

// Schedule helpers shared by the mapping pages. Days are a bitmask: Sunday = 1 ... Saturday = 64.

export const EVERY_DAY = 127
export const WEEKDAYS = 0b0111110
export const WEEKENDS = 0b1000001

/** Monday first, as people read a week. */
export const DAY_ORDER = [1, 2, 3, 4, 5, 6, 0] as const
export const DAY_SHORT = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

export const INTERVALS = [15, 30, 60, 120, 180, 360, 720, 1440]

/** The browser's time zone, for new schedules. */
export function browserTimeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || ''
  } catch {
    return ''
  }
}

export function intervalLabel(minutes: number): string {
  if (minutes >= 1440) return 'Once a day'
  if (minutes < 60) return `Every ${minutes} minutes`
  if (minutes === 60) return 'Every hour'
  if (minutes % 60 === 0) return `Every ${minutes / 60} hours`
  return `Every ${minutes} minutes`
}

export function daysLabel(days: number): string {
  if ((days & EVERY_DAY) === EVERY_DAY) return 'every day'
  if (days === WEEKDAYS) return 'Mon–Fri'
  if (days === WEEKENDS) return 'weekends'
  return DAY_ORDER.filter((d) => days & (1 << d))
    .map((d) => DAY_SHORT[d])
    .join(', ')
}

/** "Every 2 hours from 07:30, Mon–Fri" / "Once a day at 03:00, every day". */
export function describeSchedule(s: MappingSchedule): string {
  const start = s.schedule_start || '00:00'
  const when = s.interval_minutes >= 1440 ? `at ${start}` : `from ${start}`
  return `${intervalLabel(s.interval_minutes)} ${when}, ${daysLabel(s.schedule_days ?? EVERY_DAY)}`
}

/** "in 12 min" / "in 3 h" / "tomorrow 03:00" for the next run (epoch seconds). */
export function nextRunLabel(nextRunAt: number | null | undefined, now = Date.now()): string {
  if (!nextRunAt) return 'within a minute'
  const minutes = Math.ceil((nextRunAt - now / 1000) / 60)
  if (minutes <= 1) return 'within a minute'
  if (minutes < 60) return `in ${minutes} min`
  if (minutes < 24 * 60) return `in ${Math.round(minutes / 60)} h`
  return new Date(nextRunAt * 1000).toLocaleString(undefined, { weekday: 'short', hour: '2-digit', minute: '2-digit' })
}

/** "today" / "tomorrow" / "in 4 days" for a number of quota resets (-1: never). */
export function daysLabelFromNow(days: number): string {
  if (days < 0) return 'not until the quota or schedules change'
  if (days === 0) return 'today'
  if (days === 1) return 'by tomorrow'
  return `in about ${days} days`
}
