import type { QueryClient } from '@tanstack/react-query'
import type { SyncItemDetails } from './api/sync-items'
import type { TrackLike } from '../components/ui'

type AnalysisSource = { title?: string; artist?: string; channel?: string; duration_ms?: number; image?: string }

/** The source track of an add item, with duration and channel when analysis recorded them. */
export function sourceTrackOf(item: SyncItemDetails): TrackLike {
  const decision = (item.analysis_context_json as { decision?: { source?: AnalysisSource } } | null)?.decision
  const source: AnalysisSource = item.last_run?.source ?? decision?.source ?? {}
  return {
    id: item.track_id,
    title: source.title || item.track_title,
    artist: source.artist || item.track_artist,
    channel: source.channel,
    duration_ms: source.duration_ms,
    image: source.image,
  }
}

/** Refresh everything that shows queue or review state after a decision. */
export function invalidateQueue(queryClient: QueryClient) {
  for (const key of ['sync-items', 'sync-item', 'review-count', 'dashboard-stats', 'executor-logs', 'track-links', 'blacklist']) {
    queryClient.invalidateQueries({ queryKey: [key] })
  }
}

/** Default search text: "artist title" of the source track. */
export function defaultQuery(item: SyncItemDetails): string {
  const source = item.last_run?.source
  const title = source?.title || item.track_title || ''
  const artist = source?.artist || item.track_artist || ''
  return [artist, title].filter(Boolean).join(' ').trim()
}
