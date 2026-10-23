import { createLazyFileRoute } from '@tanstack/react-router'
import { useState, type FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link2, Search, Trash2 } from 'lucide-react'
import { tracksAPI, type TrackLink, type TrackLinkFilter } from '../../lib/api/tracks'
import { fullTime, inputClass, timeAgo } from '../../lib/format'
import { Button, Card, EmptyState, Notice, Page, Pagination, Pill, ScoreBadge, ServiceIcon, Tabs, trackURL } from '../../components/ui'

const PER_PAGE = 50

function Side({ service, id }: { service: 'spotify' | 'youtube'; id?: string }) {
  if (!id) {
    return (
      <span className="inline-flex items-center gap-1.5 text-xs font-medium text-youtube-700">
        <ServiceIcon service={service} className="h-3.5 w-3.5 opacity-40" />
        Not on {service === 'youtube' ? 'YouTube' : 'Spotify'}
      </span>
    )
  }
  return (
    <a
      href={trackURL(service, id) ?? undefined}
      target="_blank"
      rel="noreferrer"
      className="inline-flex items-center gap-1.5 font-mono text-xs text-gray-600 hover:text-ink hover:underline"
    >
      <ServiceIcon service={service} className="h-3.5 w-3.5" />
      {id}
    </a>
  )
}

function MatchesPage() {
  const queryClient = useQueryClient()
  const [filter, setFilter] = useState<TrackLinkFilter>('')
  const [query, setQuery] = useState('')
  const [submitted, setSubmitted] = useState('')
  const [page, setPage] = useState(1)

  const { data, isLoading, error } = useQuery({
    queryKey: ['track-links', filter, submitted, page],
    queryFn: () => tracksAPI.listLinks({ filter, q: submitted, page, per_page: PER_PAGE }),
  })

  const remove = useMutation({
    mutationFn: (link: TrackLink) => tracksAPI.deleteLink(link.id),
    onSettled: () => queryClient.invalidateQueries({ queryKey: ['track-links'] }),
  })

  const onSearch = (e: FormEvent) => {
    e.preventDefault()
    setPage(1)
    setSubmitted(query.trim())
  }

  const links = data?.items ?? []

  return (
    <Page
      title="Matches"
      description="Every song Spotube has paired across services, and the songs you said aren't available. Spotube checks this list before searching, so a track is only ever copied once."
    >
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <Tabs<TrackLinkFilter>
          label="Filter matches"
          value={filter}
          onChange={(v) => {
            setPage(1)
            setFilter(v)
          }}
          items={[
            { value: '', label: 'All' },
            { value: 'linked', label: 'Paired' },
            { value: 'missing_youtube', label: 'Not on YouTube' },
            { value: 'missing_spotify', label: 'Not on Spotify' },
          ]}
        />
        <form onSubmit={onSearch} className="flex gap-2" role="search">
          <input
            className={`${inputClass} w-64`}
            placeholder="Title, artist or track ID"
            aria-label="Search matches"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <Button type="submit" icon={<Search className="h-4 w-4" />}>
            Search
          </Button>
        </form>
      </div>

      {error && <Notice tone="error" className="mb-4">Couldn't load matches: {error.message}</Notice>}
      {remove.error && <Notice tone="error" className="mb-4">{remove.error.message}</Notice>}

      <Card className="overflow-hidden">
        {isLoading ? (
          <p className="px-4 py-10 text-center text-sm text-gray-500">Loading…</p>
        ) : links.length === 0 ? (
          <EmptyState icon={<Link2 />} title={submitted || filter ? 'No matches found' : 'No matches yet'}>
            Matches are remembered as tracks sync, and when you decide a track in Review.
          </EmptyState>
        ) : (
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead className="border-b border-gray-200 bg-gray-50 text-left text-xs font-medium uppercase tracking-wide text-gray-500">
                <tr>
                  <th className="px-4 py-3">Song</th>
                  <th className="px-4 py-3">Spotify</th>
                  <th className="px-4 py-3">YouTube</th>
                  <th className="px-4 py-3">How</th>
                  <th className="hidden px-4 py-3 md:table-cell">Updated</th>
                  <th className="px-4 py-3 text-right">
                    <span className="sr-only">Actions</span>
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100">
                {links.map((link) => (
                  <tr key={link.id} className="align-middle">
                    <td className="max-w-xs px-4 py-3">
                      <div className="truncate font-medium text-ink">{link.title || '—'}</div>
                      {link.artist && <div className="truncate text-xs text-gray-500">{link.artist}</div>}
                    </td>
                    <td className="px-4 py-3">
                      <Side service="spotify" id={link.spotify_id} />
                    </td>
                    <td className="px-4 py-3">
                      <Side service="youtube" id={link.youtube_id} />
                    </td>
                    <td className="px-4 py-3">
                      {link.origin === 'manual' ? (
                        <Pill tone="ink">You decided</Pill>
                      ) : (
                        <span className="inline-flex items-center gap-1.5">
                          <Pill>Auto</Pill>
                          {link.score != null && <ScoreBadge value={link.score} />}
                        </span>
                      )}
                    </td>
                    <td className="hidden whitespace-nowrap px-4 py-3 text-xs text-gray-500 md:table-cell" title={fullTime(link.updated)}>
                      {timeAgo(link.updated)}
                    </td>
                    <td className="px-4 py-3 text-right">
                      <Button
                        variant="ghost"
                        size="sm"
                        icon={<Trash2 className="h-3.5 w-3.5" />}
                        loading={remove.isPending && remove.variables?.id === link.id}
                        onClick={() => remove.mutate(link)}
                        title="Forget this match. Spotube decides again on the next sync; playlists are not changed."
                      >
                        Forget
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <Pagination page={page} totalPages={data?.totalPages ?? 1} totalItems={data?.totalItems ?? 0} noun="match" plural="matches" onPage={setPage} />
      </Card>
    </Page>
  )
}

export const Route = createLazyFileRoute('/_authenticated/matches')({
  component: MatchesPage,
})
