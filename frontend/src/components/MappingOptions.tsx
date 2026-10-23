export type MappingOptionsValue = {
  sync_tracks: boolean
  sync_name: boolean
  interval_minutes: number
}

const intervals = [5, 15, 30, 60, 180, 360, 720, 1440]

function intervalLabel(minutes: number): string {
  if (minutes < 60) return `Every ${minutes} minutes`
  if (minutes === 60) return 'Every hour'
  if (minutes === 1440) return 'Once a day'
  return `Every ${minutes / 60} hours`
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
        className="mt-0.5 h-4 w-4 accent-indigo-600"
      />
      <span>
        <span className="block text-sm font-medium text-gray-900">{label}</span>
        <span className="block text-sm text-gray-500">{description}</span>
      </span>
    </label>
  )
}

/** Sync options shared by the create and edit mapping pages. */
export function MappingOptions({
  value,
  onChange,
}: {
  value: MappingOptionsValue
  onChange: (value: MappingOptionsValue) => void
}) {
  const options = intervals.includes(value.interval_minutes)
    ? intervals
    : [...intervals, value.interval_minutes].sort((a, b) => a - b)

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
      <label className="block">
        <span className="block text-sm font-medium text-gray-900">Check for changes</span>
        <select
          value={value.interval_minutes}
          onChange={(e) => onChange({ ...value, interval_minutes: Number(e.target.value) })}
          className="mt-1.5 w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm sm:w-64"
        >
          {options.map((m) => (
            <option key={m} value={m}>
              {intervalLabel(m)}
            </option>
          ))}
        </select>
        <span className="mt-1 block text-xs text-gray-500">
          Each sync reads both playlists; adding a track to YouTube uses about 150 of the 10,000 daily quota units.
        </span>
      </label>
    </div>
  )
}
