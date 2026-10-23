import type { MappingSchedule } from '../lib/api/types'
import { DAY_ORDER, DAY_SHORT, EVERY_DAY, INTERVALS, WEEKDAYS, WEEKENDS, describeSchedule, intervalLabel } from '../lib/schedule'
import { inputClass } from '../lib/format'

export type MappingOptionsValue = MappingSchedule & {
  sync_tracks: boolean
  sync_name: boolean
}

function Toggle({
  checked,
  onChange,
  label,
  description,
}: {
  checked: boolean
  onChange: (checked: boolean) => void
  label: string
  description: string
}) {
  return (
    <label className="flex cursor-pointer items-start gap-3">
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="mt-0.5 h-4 w-4 accent-ink"
      />
      <span>
        <span className="block text-sm font-medium text-gray-900">{label}</span>
        <span className="block text-sm text-gray-500">{description}</span>
      </span>
    </label>
  )
}

const presets = [
  { label: 'Every day', days: EVERY_DAY },
  { label: 'Weekdays', days: WEEKDAYS },
  { label: 'Weekends', days: WEEKENDS },
]

/** When a mapping syncs: interval, start time and days. */
export function ScheduleFields({
  value,
  onChange,
}: {
  value: MappingSchedule
  onChange: (value: MappingSchedule) => void
}) {
  const options = INTERVALS.includes(value.interval_minutes)
    ? INTERVALS
    : [...INTERVALS, value.interval_minutes].sort((a, b) => a - b)
  const daily = value.interval_minutes >= 1440
  const toggleDay = (d: number) => {
    const days = value.schedule_days ^ (1 << d)
    if (days !== 0) onChange({ ...value, schedule_days: days })
  }

  return (
    <fieldset className="space-y-3">
      <legend className="text-sm font-medium text-gray-900">Schedule</legend>
      <div className="flex flex-wrap items-end gap-3">
        <label className="block">
          <span className="mb-1 block text-[11px] font-medium uppercase tracking-wider text-gray-500">Check</span>
          <select
            aria-label="Check for changes"
            value={value.interval_minutes}
            onChange={(e) => onChange({ ...value, interval_minutes: Number(e.target.value) })}
            className={`${inputClass} !w-48`}
          >
            {options.map((m) => (
              <option key={m} value={m}>
                {intervalLabel(m)}
              </option>
            ))}
          </select>
        </label>
        <label className="block">
          <span className="mb-1 block text-[11px] font-medium uppercase tracking-wider text-gray-500">
            {daily ? 'At' : 'Starting at'}
          </span>
          <input
            type="time"
            aria-label={daily ? 'Run at' : 'Start time'}
            value={value.schedule_start}
            onChange={(e) => e.target.value && onChange({ ...value, schedule_start: e.target.value })}
            className={`${inputClass} !w-32`}
            required
          />
        </label>
        {value.schedule_tz && <span className="pb-2 text-xs text-gray-400">{value.schedule_tz}</span>}
      </div>
      <div className="flex flex-wrap items-center gap-1.5" role="group" aria-label="Days">
        {DAY_ORDER.map((d) => {
          const on = (value.schedule_days & (1 << d)) !== 0
          return (
            <button
              key={d}
              type="button"
              aria-pressed={on}
              onClick={() => toggleDay(d)}
              className={`w-11 rounded-md border px-0 py-1 text-xs font-medium transition-colors ${
                on ? 'border-ink bg-ink text-white' : 'border-gray-300 bg-white text-gray-500 hover:border-gray-400'
              }`}
            >
              {DAY_SHORT[d]}
            </button>
          )
        })}
        <span className="mx-1 text-gray-300">|</span>
        {presets.map((p) => (
          <button
            key={p.label}
            type="button"
            onClick={() => onChange({ ...value, schedule_days: p.days })}
            className={`rounded px-1.5 py-0.5 text-xs ${
              value.schedule_days === p.days ? 'font-semibold text-ink' : 'text-gray-500 hover:text-ink'
            }`}
          >
            {p.label}
          </button>
        ))}
      </div>
      <p className="text-xs text-gray-500">
        {describeSchedule(value)}. Each check costs 1 YouTube unit; the YouTube playlist is only re-read (about 1 unit
        per 50 videos) when it changed.
      </p>
    </fieldset>
  )
}

/** Sync options shared by the create and edit mapping pages. */
export function MappingOptions({
  value,
  onChange,
  scheduleNote,
}: {
  value: MappingOptionsValue
  onChange: (value: MappingOptionsValue) => void
  /** Shown under the schedule (e.g. the recommendation). */
  scheduleNote?: React.ReactNode
}) {
  return (
    <div className="space-y-5">
      <Toggle
        checked={value.sync_tracks}
        onChange={(sync_tracks) => onChange({ ...value, sync_tracks })}
        label="Sync tracks"
        description="Tracks added on either side are found and added to the other. Removals are not copied."
      />
      <Toggle
        checked={value.sync_name}
        onChange={(sync_name) => onChange({ ...value, sync_name })}
        label="Sync playlist name"
        description="Rename the YouTube playlist to match the Spotify one."
      />
      <div className="border-t border-gray-100 pt-5">
        <ScheduleFields value={value} onChange={(s) => onChange({ ...value, ...s })} />
        {scheduleNote && <div className="mt-3">{scheduleNote}</div>}
      </div>
    </div>
  )
}
