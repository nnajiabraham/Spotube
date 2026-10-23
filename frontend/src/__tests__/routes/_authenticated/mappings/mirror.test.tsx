import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { http, HttpResponse } from 'msw'
import { Route } from '../../../../routes/_authenticated/mappings/mirror.lazy'

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    Link: ({ to, children, className }: { to: string; children: React.ReactNode; className?: string }) => (
      <a href={to} className={className}>
        {children}
      </a>
    ),
  }
})

const Mirror = Route.options.component as React.ComponentType

function useApi() {
  const sent: Array<Record<string, unknown>> = []
  globalThis.mswServer?.use(
    http.get('*/api/auth/spotify/playlists', () =>
      HttpResponse.json([
        { id: 'sp-a', name: 'Alpha', track_count: 1200 },
        { id: 'sp-b', name: 'Bravo', track_count: 30 },
        { id: 'sp-paired', name: 'Paired', track_count: 5 },
      ]),
    ),
    http.get('*/api/auth/youtube/playlists', () =>
      HttpResponse.json([
        { id: 'yt-bravo', name: 'bravo', itemCount: 2 },
        { id: 'yt-x', name: 'X', itemCount: 1 },
      ]),
    ),
    http.get('*/api/collections/mappings/records', () =>
      HttpResponse.json({
        page: 1,
        perPage: 200,
        totalItems: 1,
        totalPages: 1,
        items: [{ id: 'm1', spotify_playlist_id: 'sp-paired', youtube_playlist_id: 'yt-x' }],
      }),
    ),
    http.post('*/api/collections/mappings/records/mirror', async ({ request }) => {
      const body = (await request.json()) as { playlists: Array<{ id: string; name: string }> }
      sent.push(body)
      return HttpResponse.json({
        items: body.playlists.map((p, i) =>
          i === 0
            ? { playlist_id: p.id, name: p.name, status: 'created' }
            : { playlist_id: p.id, name: p.name, status: 'waiting_quota', error: 'not enough YouTube quota left today' },
        ),
      })
    }),
  )
  return sent
}

const renderPage = () =>
  render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <Mirror />
    </QueryClientProvider>,
  )

describe('Mirror playlists', () => {
  it('flags paired playlists and same names on the other side', async () => {
    useApi()
    renderPage()
    expect(await screen.findByRole('checkbox', { name: 'Paired' })).toBeDisabled()
    expect(screen.getByText('Already paired')).toBeInTheDocument()
    expect(screen.getByText('Same name on YouTube')).toBeInTheDocument()
  })

  it('creates one mapping per selected playlist and reports each result', async () => {
    const sent = useApi()
    const user = userEvent.setup()
    renderPage()
    await screen.findByRole('checkbox', { name: 'Alpha' })

    await user.click(screen.getByRole('checkbox', { name: 'Select all' }))
    expect(screen.getByText('2 selected')).toBeInTheDocument()
    expect(await screen.findByLabelText('Quota estimate')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Create YouTube copies (2)' }))
    await waitFor(() => expect(sent).toHaveLength(1))
    expect(sent[0]).toMatchObject({
      source: 'spotify',
      sync_tracks: true,
      sync_now: true,
      playlists: [
        { id: 'sp-a', name: 'Alpha', tracks: 1200 },
        { id: 'sp-b', name: 'Bravo', tracks: 30 },
      ],
    })

    const results = await screen.findByRole('region', { name: 'Results' })
    expect(within(results).getByText('created')).toBeInTheDocument()
    expect(within(results).getByText('waiting for quota')).toBeInTheDocument()
    // The one that didn't fit stays selected for a retry after the reset.
    expect(screen.getByText('1 selected')).toBeInTheDocument()
  })

  it('switches direction to copy YouTube playlists to Spotify', async () => {
    useApi()
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByRole('radio', { name: /YouTube → Spotify/ }))
    expect(await screen.findByRole('checkbox', { name: 'bravo' })).toBeEnabled()
    expect(screen.getByRole('checkbox', { name: 'X' })).toBeDisabled()
  })
})
