import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ConnectionCard } from './ConnectionCard'
import { api, ApiError } from '../lib/api'

vi.mock('../lib/api', () => ({
  api: { getSpotifyPlaylists: vi.fn(), getYouTubePlaylists: vi.fn() },
  oauthAPI: {
    spotify: { getLoginURL: () => '/api/auth/spotify/login' },
    youtube: { getLoginURL: () => '/api/auth/youtube/login' },
  },
  ApiError: class ApiError extends Error {
    status: number
    constructor(status: number, message: string) {
      super(message)
      this.status = status
    }
  },
}))

vi.mock('@tanstack/react-router', () => ({
  Link: ({ children, to }: { children: React.ReactNode; to: string }) => <a href={to}>{children}</a>,
}))

function renderCard(service: 'spotify' | 'youtube') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={client}>
      <ConnectionCard service={service} />
    </QueryClientProvider>,
  )
}

describe('ConnectionCard', () => {
  it('offers to connect when the provider rejects the probe', async () => {
    vi.mocked(api.getYouTubePlaylists).mockRejectedValueOnce(new ApiError(401, 'reconnect required'))
    renderCard('youtube')

    expect(await screen.findByText('Not connected')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /connect youtube/i })).toBeInTheDocument()
  })

  it('links to playlists and allows reconnecting when connected', async () => {
    vi.mocked(api.getSpotifyPlaylists).mockResolvedValueOnce({ items: [], total: 0, limit: 1, offset: 0, next: '' })
    renderCard('spotify')

    expect(await screen.findByText('Connected')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Playlists' })).toHaveAttribute('href', '/settings/spotify')
    expect(screen.getByRole('button', { name: /reconnect/i })).toBeInTheDocument()
  })

  it('shows a loading state while probing', () => {
    vi.mocked(api.getSpotifyPlaylists).mockImplementation(() => new Promise(() => {}))
    renderCard('spotify')

    expect(screen.getByRole('status')).toBeInTheDocument()
  })
})
