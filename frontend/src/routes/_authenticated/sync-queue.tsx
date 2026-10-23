import { createFileRoute } from '@tanstack/react-router'
import type { SyncItemStatus } from '../../lib/api/sync-items'

const statuses: SyncItemStatus[] = ['pending', 'running', 'done', 'error', 'skipped', 'review']

export type SyncQueueSearch = { status?: SyncItemStatus }

export const Route = createFileRoute('/_authenticated/sync-queue')({
  validateSearch: (search: Record<string, unknown>): SyncQueueSearch => {
    const status = search.status as SyncItemStatus
    return statuses.includes(status) ? { status } : {}
  },
})
