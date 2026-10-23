import { createFileRoute } from '@tanstack/react-router'
import type { SyncItemService } from '../../lib/api/sync-items'

export type ReviewSearch = { mapping?: string; to?: SyncItemService }

export const Route = createFileRoute('/_authenticated/review')({
  validateSearch: (search: Record<string, unknown>): ReviewSearch => {
    const out: ReviewSearch = {}
    if (typeof search.mapping === 'string' && search.mapping) out.mapping = search.mapping
    if (search.to === 'spotify' || search.to === 'youtube') out.to = search.to
    return out
  },
})
