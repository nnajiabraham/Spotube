import { useState, type FormEvent } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link2, Search, Sparkles } from 'lucide-react'
import { tracksAPI, YOUTUBE_SEARCH_COST, type SearchHit } from '../lib/api/tracks'
import type { SyncItemDetails } from '../lib/api/sync-items'
import { serviceName, inputClass } from '../lib/format'
import { defaultQuery } from '../lib/review'
import { Button, EmptyState, Notice, ScoreBadge, Tabs, TrackRow } from './ui'

type Mode = 'suggestions' | 'search' | 'paste'

/**
 * Finds the right destination track for a sync item: the candidates from the last automatic
 * search (free), an in-app search of the destination service, or a pasted link. Searches run
 * only when submitted, and results are cached on the server for 30 minutes.
 */
export function TrackPicker({
  item,
  onPick,
  picking,
}: {
  item: SyncItemDetails
  onPick: (hit: SearchHit) => void
  /** ID of the hit currently being saved. */
  picking?: string | null
}) {
  const dest = item.destination_service
  const suggestions = item.last_run?.candidates ?? []
  const [mode, setMode] = useState<Mode>(suggestions.length ? 'suggestions' : 'search')
  const [query, setQuery] = useState(() => defaultQuery(item))
  const [submitted, setSubmitted] = useState<string | null>(null)
  const [ref, setRef] = useState('')
  const [submittedRef, setSubmittedRef] = useState<string | null>(null)

  const search = useQuery({
    queryKey: ['track-search', dest, submitted, item.id],
    queryFn: () => tracksAPI.search(dest, submitted!, item.id),
    enabled: !!submitted,
    staleTime: Infinity,
    retry: false,
  })
  const lookup = useQuery({
    queryKey: ['track-lookup', dest, submittedRef, item.id],
    queryFn: () => tracksAPI.lookup(dest, submittedRef!, item.id),
    enabled: !!submittedRef,
    staleTime: Infinity,
    retry: false,
  })

  const pickButton = (hit: SearchHit) => (
    <Button
      size="sm"
      variant="primary"
      loading={picking === hit.id}
      disabled={!!picking}
      onClick={() => onPick(hit)}
    >
      Use this
    </Button>
  )

  const hitRow = (hit: SearchHit) => (
    <li key={hit.id}>
      <TrackRow
        service={dest}
        track={hit}
        trailing={
          <>
            {hit.score != null && <ScoreBadge value={hit.score} breakdown={hit.breakdown} />}
            {pickButton(hit)}
          </>
        }
      />
    </li>
  )

  const submitSearch = (e: FormEvent) => {
    e.preventDefault()
    const q = query.trim()
    if (q) setSubmitted(q)
  }
  const submitRef = (e: FormEvent) => {
    e.preventDefault()
    const r = ref.trim()
    if (r) setSubmittedRef(r)
  }

  return (
    <div className="space-y-4">
      <Tabs<Mode>
        label="How to find the track"
        value={mode}
        onChange={setMode}
        items={[
          { value: 'suggestions', label: <><Sparkles className="mr-1.5 h-3.5 w-3.5" />Suggestions{suggestions.length ? ` (${suggestions.length})` : ''}</> },
          { value: 'search', label: <><Search className="mr-1.5 h-3.5 w-3.5" />Search {serviceName(dest)}</> },
          { value: 'paste', label: <><Link2 className="mr-1.5 h-3.5 w-3.5" />Paste a link</> },
        ]}
      />

      {mode === 'suggestions' &&
        (suggestions.length ? (
          <>
            <p className="text-xs text-gray-500">
              What Spotube found when it searched for
              {item.last_run?.search_query ? <> “<span className="font-medium text-gray-700">{item.last_run.search_query}</span>”</> : ' this track'}.
              None scored high enough to add automatically.
            </p>
            <ul className="-mx-3 divide-y divide-gray-100">
              {suggestions.map((c) =>
                hitRow({ ...c, score: c.score, breakdown: c.breakdown }),
              )}
            </ul>
          </>
        ) : (
          <EmptyState icon={<Sparkles />} title="No suggestions yet">
            The last search found nothing. Search {serviceName(dest)} or paste a link.
          </EmptyState>
        ))}

      {mode === 'search' && (
        <>
          <form onSubmit={submitSearch} className="flex gap-2" role="search">
            <input
              className={inputClass}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={`Search ${serviceName(dest)}`}
              aria-label={`Search ${serviceName(dest)}`}
              autoFocus
            />
            <Button type="submit" variant="primary" icon={<Search className="h-4 w-4" />} loading={search.isFetching}>
              Search
            </Button>
          </form>
          {dest === 'youtube' && (
            <p className="text-xs text-gray-500">
              Each YouTube search uses {YOUTUBE_SEARCH_COST} of the 10,000 daily quota units. Repeating a search within
              30 minutes is free; pasting a link costs 1.
            </p>
          )}
          {search.error && <Notice tone="error">{search.error.message}</Notice>}
          {search.data &&
            (search.data.items.length ? (
              <>
                <p className="text-xs text-gray-500">
                  Best match first{search.data.cached ? ' · from cache, no quota used' : ''}.
                </p>
                <ul className="-mx-3 divide-y divide-gray-100">{search.data.items.map(hitRow)}</ul>
              </>
            ) : (
              <EmptyState icon={<Search />} title="No results">
                Try fewer words, or paste a link to the right track.
              </EmptyState>
            ))}
        </>
      )}

      {mode === 'paste' && (
        <>
          <form onSubmit={submitRef} className="flex gap-2">
            <input
              className={inputClass}
              value={ref}
              onChange={(e) => setRef(e.target.value)}
              placeholder={
                dest === 'youtube'
                  ? 'https://music.youtube.com/watch?v=… or a video ID'
                  : 'https://open.spotify.com/track/… or a track ID'
              }
              aria-label={`${serviceName(dest)} link or ID`}
              autoFocus
            />
            <Button type="submit" variant="primary" loading={lookup.isFetching}>
              Look up
            </Button>
          </form>
          <p className="text-xs text-gray-500">
            Copy the link from {dest === 'youtube' ? 'YouTube or YouTube Music (Share → Copy link)' : 'Spotify (Share → Copy song link)'}.
          </p>
          {lookup.error && <Notice tone="error">{lookup.error.message}</Notice>}
          {lookup.data && <ul className="-mx-3">{hitRow(lookup.data)}</ul>}
        </>
      )}
    </div>
  )
}
