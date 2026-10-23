import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ConnectionCard } from './ConnectionCard'
import { oauthAPI } from '../lib/api'
import { APIClientError } from '../lib/api/client'

vi.mock('../lib/api', () => ({
  oauthAPI: {
    spotify: { getLoginURL: () => '/api/auth/spotify/login', getPlaylists: vi.fn() },
    youtube: { getLoginURL: () => '/api/auth/youtube/login', getPlaylists: vi.fn() },
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
    vi.mocked(oauthAPI.youtube.getPlaylists).mockRejectedValueOnce(new APIClientError('unauthorized', 'reconnect required', 401))
    renderCard('youtube')

    expect(await screen.findByText('Not connected')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /connect youtube/i })).toBeInTheDocument()
  })

  it('links to playlists and allows reconnecting when connected', async () => {
    vi.mocked(oauthAPI.spotify.getPlaylists).mockResolvedValueOnce([])
    renderCard('spotify')

    expect(await screen.findByText('Connected')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Playlists' })).toHaveAttribute('href', '/settings/spotify')
    expect(screen.getByRole('button', { name: /reconnect/i })).toBeInTheDocument()
  })

  it('shows a loading state while probing', () => {
    vi.mocked(oauthAPI.spotify.getPlaylists).mockImplementation(() => new Promise(() => {}))
    renderCard('spotify')

    expect(screen.getByRole('status')).toBeInTheDocument()
  })
})
