// Formatting helpers and shared class names used across pages.

export const buttonClass = {
  primary:
    'inline-flex items-center justify-center gap-1.5 rounded-lg bg-indigo-600 px-3.5 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-700 disabled:cursor-not-allowed disabled:opacity-50',
  secondary:
    'inline-flex items-center justify-center gap-1.5 rounded-lg border border-gray-300 bg-white px-3.5 py-2 text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50',
  ghost:
    'inline-flex items-center justify-center gap-1.5 rounded-lg px-2 py-1.5 text-sm font-medium text-gray-600 hover:bg-gray-100 hover:text-gray-900 disabled:cursor-not-allowed disabled:opacity-50',
}

export function serviceName(service: string): string {
  return service === 'youtube' ? 'YouTube' : service === 'spotify' ? 'Spotify' : service
}

/** "3 min ago" style label for an epoch-seconds timestamp. */
export function timeAgo(epochSeconds: number | null | undefined, now = Date.now()): string {
  if (!epochSeconds) return 'never'
  const seconds = Math.round(now / 1000 - epochSeconds)
  if (seconds < 45) return 'just now'
  const minutes = Math.round(seconds / 60)
  if (minutes < 60) return `${minutes} min ago`
  const hours = Math.round(minutes / 60)
  if (hours < 24) return `${hours} h ago`
  const days = Math.round(hours / 24)
  if (days < 30) return `${days} d ago`
  return new Date(epochSeconds * 1000).toLocaleDateString()
}

export function fullTime(epochSeconds: number | null | undefined): string {
  return epochSeconds ? new Date(epochSeconds * 1000).toLocaleString() : ''
}
