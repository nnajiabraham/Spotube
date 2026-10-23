import { createLazyFileRoute, Link, useNavigate, useSearch } from '@tanstack/react-router'
import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { CheckCircle2 } from 'lucide-react'
import { syncItemsAPI, type SyncItemService } from '../../lib/api/sync-items'
import { mappingsAPI } from '../../lib/api/mappings'
import { inputClass, linkClass } from '../../lib/format'
import { EmptyState, Notice, Page, Pagination, Tabs } from '../../components/ui'
import { ReviewCard, type ReviewOutcome } from '../../components/ReviewCard'

const PER_PAGE = 10

function ReviewPage() {
  const navigate = useNavigate({ from: '/review' })
  const { mapping, to } = useSearch({ from: '/_authenticated/review' })
  const [page, setPage] = useState(1)
  const [outcome, setOutcome] = useState<ReviewOutcome | null>(null)

  const setFilter = (next: { mapping?: string; to?: SyncItemService }) => {
    setPage(1)
    navigate({ search: { mapping, to, ...next } })
  }

  const { data: mappings } = useQuery({
    queryKey: ['mappings', 'review-filter'],
    queryFn: () => mappingsAPI.getList({ per_page: 100 }),
  })
  const { data, isLoading, error } = useQuery({
    queryKey: ['sync-items', 'review', mapping, to, page],
    queryFn: () =>
      syncItemsAPI.getList({
        status: 'review',
        mapping_id: mapping,
        service: to,
        page,
        per_page: PER_PAGE,
        order: 'asc',
        include_last_run: true,
      }),
    refetchOnMount: 'always',
  })
  const items = data?.items ?? []

  return (
    <Page
      title="Review"
      description="Tracks Spotube couldn't match with confidence. Choose the right one, or say it isn't available. Spotube remembers your decision for every playlist."
    >
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <Tabs<'' | SyncItemService>
          label="Direction"
          value={to ?? ''}
          onChange={(v) => setFilter({ to: v || undefined })}
          items={[
            { value: '', label: 'Both directions' },
            { value: 'youtube', label: 'To YouTube' },
            { value: 'spotify', label: 'To Spotify' },
          ]}
        />
        <label className="flex items-center gap-2 text-sm text-gray-600">
          Playlist pair
          <select
            className={`${inputClass} !w-auto`}
            value={mapping ?? ''}
            onChange={(e) => setFilter({ mapping: e.target.value || undefined })}
          >
            <option value="">All</option>
            {mappings?.items.map((m) => (
              <option key={m.id} value={m.id}>
                {m.spotify_playlist_name || m.spotify_playlist_id} ⇄ {m.youtube_playlist_name || m.youtube_playlist_id}
              </option>
            ))}
          </select>
        </label>
      </div>

      {outcome && (
        <Notice tone={outcome.ok ? 'success' : 'warning'} onDismiss={() => setOutcome(null)} className="mb-4">
          {outcome.text}
        </Notice>
      )}
      {error && <Notice tone="error" className="mb-4">Couldn't load the review queue: {error.message}</Notice>}

      {isLoading ? (
        <p className="py-10 text-center text-sm text-gray-500">Loading…</p>
      ) : items.length === 0 ? (
        <div className="rounded-lg border border-dashed border-gray-300 bg-white">
          <EmptyState icon={<CheckCircle2 className="text-spotify" />} title="Nothing to review">
            Every track either synced or has a decision.{' '}
            <Link to="/matches" className={linkClass}>
              See remembered matches
            </Link>
            .
          </EmptyState>
        </div>
      ) : (
        <div className="space-y-4">
          {items.map((item) => (
            <ReviewCard key={item.id} item={item} onOutcome={setOutcome} />
          ))}
          <div className="overflow-hidden rounded-lg border border-gray-200 bg-white">
            <Pagination
              page={page}
              totalPages={data?.totalPages ?? 1}
              totalItems={data?.totalItems ?? 0}
              noun="track"
              onPage={setPage}
            />
          </div>
        </div>
      )}
    </Page>
  )
}

export const Route = createLazyFileRoute('/_authenticated/review')({
  component: ReviewPage,
})
