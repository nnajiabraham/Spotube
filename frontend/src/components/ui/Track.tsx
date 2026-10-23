import type { ReactNode } from 'react'
import { ExternalLink, Music2 } from 'lucide-react'
import { ServiceIcon } from './icons'
import { formatDuration, trackURL, youtubeThumbnail } from '../../lib/format'

export type TrackLike = {
  id?: string
  title?: string
  artist?: string
  channel?: string
  album?: string
  duration_ms?: number
  image?: string
}

/** Square artwork for Spotify, 16:9 thumbnail for YouTube, or a placeholder. */
export function TrackArt({ service, image, id, size = 'md' }: { service: string; image?: string; id?: string; size?: 'sm' | 'md' }) {
  const box = size === 'sm' ? 'h-9' : 'h-12'
  const shape = service === 'youtube' ? `${box} aspect-video` : `${box} aspect-square`
  if (!image && service === 'youtube') image = youtubeThumbnail(id)
  if (image) return <img src={image} alt="" loading="lazy" className={`${shape} shrink-0 rounded-md bg-gray-100 object-cover`} />
  return (
    <div className={`${shape} flex shrink-0 items-center justify-center rounded-md bg-gray-100 text-gray-300`}>
      <Music2 className="h-4 w-4" />
    </div>
  )
}

/**
 * One track or video: artwork, title, artist and details, with a link that opens it on its
 * service. `trailing` holds actions or a score.
 */
export function TrackRow({
  service,
  track,
  trailing,
  showArt = true,
  highlight = false,
}: {
  service: string
  track: TrackLike
  trailing?: ReactNode
  showArt?: boolean
  highlight?: boolean
}) {
  const url = trackURL(service, track.id)
  const meta = [
    service === 'youtube' && track.channel && track.channel !== track.artist ? track.channel : '',
    track.album,
    formatDuration(track.duration_ms),
  ].filter(Boolean)
  return (
    <div className={`flex items-center gap-3 rounded-lg px-3 py-2.5 ${highlight ? 'bg-spotify-50 ring-1 ring-spotify-100' : ''}`}>
      {showArt && <TrackArt service={service} image={track.image} id={track.id} />}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          <ServiceIcon service={service} className="h-3.5 w-3.5 shrink-0" />
          {url ? (
            <a
              href={url}
              target="_blank"
              rel="noreferrer"
              className="group inline-flex min-w-0 items-center gap-1 font-medium text-ink hover:underline"
              title={`Open on ${service === 'youtube' ? 'YouTube' : 'Spotify'}`}
            >
              <span className="truncate">{track.title || track.id || 'Unknown track'}</span>
              <ExternalLink className="h-3 w-3 shrink-0 text-gray-300 group-hover:text-gray-500" />
            </a>
          ) : (
            <span className="truncate font-medium text-ink">{track.title || 'Unknown track'}</span>
          )}
        </div>
        <div className="truncate text-xs text-gray-500">
          {track.artist || 'Unknown artist'}
          {meta.length > 0 && <span className="text-gray-400"> · {meta.join(' · ')}</span>}
        </div>
      </div>
      {trailing && <div className="flex shrink-0 items-center gap-2">{trailing}</div>}
    </div>
  )
}
