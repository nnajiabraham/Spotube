import { describe, it, expect, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { YoutubeConnectionCard } from './YoutubeConnectionCard'
import { api, ApiError } from '../lib/api'

// Mock the API module
vi.mock('../lib/api', () => ({
  api: {
    getYouTubePlaylists: vi.fn()
  },
  ApiError: class ApiError extends Error {
    status: number
    constructor(status: number, message: string) {
      super(message)
      this.status = status
    }
  }
}))

// Mock the router
vi.mock('@tanstack/react-router', () => ({
  Link: ({ children, to }: { children: React.ReactNode; to: string }) => (
    <a href={to}>{children}</a>
  )
}))

describe('YoutubeConnectionCard', () => {
  const createWrapper = () => {
    const queryClient = new QueryClient({
      defaultOptions: {
        queries: {
          retry: false,
        },
      },
    })
    return ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>
        {children}
      </QueryClientProvider>
    )
  }

  it('shows connect button when not connected', async () => {
    vi.mocked(api.getYouTubePlaylists).mockRejectedValueOnce(
      new ApiError(401, 'Not connected')
    )

    render(<YoutubeConnectionCard />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: /connect youtube/i })).toBeInTheDocument()
    })

    const connectLink = screen.getByRole('link', { name: /connect youtube/i })
    expect(connectLink).toBeInTheDocument()
    expect(connectLink).toHaveAttribute('href', '/api/auth/google/login')
  })

  it('shows view playlists button when connected', async () => {
    vi.mocked(api.getYouTubePlaylists).mockResolvedValueOnce({
      items: [
        {
          id: 'PLxxxxxxxxxxxxxxxxxxxx',
          title: 'Test Playlist',
          itemCount: 42,
          description: 'A test playlist'
        }
      ]
    })

    render(<YoutubeConnectionCard />, { wrapper: createWrapper() })

    await waitFor(() => {
      expect(screen.getByText('YouTube Connected')).toBeInTheDocument()
    })

    const viewPlaylistsLink = screen.getByRole('link', { name: /view playlists/i })
    expect(viewPlaylistsLink).toBeInTheDocument()
    expect(viewPlaylistsLink).toHaveAttribute('href', '/settings/youtube')
  })

  it('shows loading state initially', () => {
    vi.mocked(api.getYouTubePlaylists).mockImplementation(
      () => new Promise(() => {}) // Never resolves
    )

    render(<YoutubeConnectionCard />, { wrapper: createWrapper() })

    expect(screen.getByRole('status')).toBeInTheDocument()
    expect(screen.queryByText('Connect YouTube')).not.toBeInTheDocument()
    expect(screen.queryByText('YouTube Connected')).not.toBeInTheDocument()
  })
}) 