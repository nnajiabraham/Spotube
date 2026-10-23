// Formatting helpers and shared class names used across pages.

const buttonBase =
  'inline-flex items-center justify-center gap-1.5 whitespace-nowrap rounded-md border px-3 py-1.5 text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ink disabled:cursor-not-allowed disabled:opacity-50'

export const buttonClass = {
  primary: `${buttonBase} border-ink bg-ink text-white hover:border-ink-700 hover:bg-ink-700`,
  secondary: `${buttonBase} border-gray-300 bg-white text-ink hover:border-gray-400 hover:bg-gray-50`,
  ghost: `${buttonBase} border-transparent !px-2 !py-1 text-gray-600 hover:bg-gray-100 hover:text-ink`,
  danger: `${buttonBase} border-youtube-700 bg-youtube-700 text-white hover:bg-youtube-600`,
  spotify: `${buttonBase} border-spotify-600 bg-spotify text-ink hover:bg-spotify-600`,
}

export type ButtonVariant = keyof typeof buttonClass

/** Inline text link. */
export const linkClass = 'font-medium text-ink underline decoration-gray-300 underline-offset-4 hover:decoration-ink'

/** Header link next to a section title ("Open queue →"). */
export const sectionLinkClass = 'text-xs font-medium text-gray-500 hover:text-ink'

/** Text inputs and selects. */
export const inputClass =
  'block w-full rounded-md border border-gray-300 bg-white px-3 py-1.5 text-sm text-ink placeholder:text-gray-400 focus:border-ink focus:outline-none focus:ring-1 focus:ring-ink'

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

export function trackURL(service: string | undefined, id: string | undefined): string | null {
  if (!id) return null
  if (service === 'youtube') return `https://www.youtube.com/watch?v=${id}`
  if (service === 'spotify') return `https://open.spotify.com/track/${id}`
  return null
}

/** "3:45" from milliseconds; empty when unknown. */
/**
 * A YouTube video's thumbnail, built from its ID. It is the URL the Data API returns, so it costs
 * no quota.
 */
export function youtubeThumbnail(id?: string): string | undefined {
  return id ? `https://i.ytimg.com/vi/${encodeURIComponent(id)}/mqdefault.jpg` : undefined
}

export function formatDuration(ms?: number): string {
  if (!ms || ms <= 0) return ''
  const total = Math.round(ms / 1000)
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = String(total % 60).padStart(2, '0')
  return h > 0 ? `${h}:${String(m).padStart(2, '0')}:${s}` : `${m}:${s}`
}
