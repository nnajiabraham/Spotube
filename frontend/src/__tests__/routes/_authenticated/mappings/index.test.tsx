import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Route } from '../../../../routes/_authenticated/mappings/index.lazy'
import { http, HttpResponse } from 'msw'

// Mock TanStack Router - partial mock to preserve createLazyFileRoute
vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    Link: ({ to, children, className }: { to: string; children: React.ReactNode; className?: string }) => (
      <a href={to} className={className}>{children}</a>
    ),
    useNavigate: () => vi.fn(),
  }
})

// Extract the component from the Route
const MappingsList = Route.options.component as React.ComponentType

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  })

  return function Wrapper({ children }: { children: React.ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        {children}
      </QueryClientProvider>
    )
  }
}

const renderWithProviders = (component: React.ReactElement) => {
  return render(component, { wrapper: createWrapper() })
}

describe('MappingsList', () => {
  const server = globalThis.mswServer

  it('renders loading state initially', async () => {
    renderWithProviders(<MappingsList />)

    expect(screen.getByRole('status')).toHaveTextContent('Loading mappings')
  })

  it('renders empty state when no mappings exist', async () => {
    server?.use(
      http.get('*/api/collections/mappings/records', () => {
        return HttpResponse.json({
          page: 1,
          perPage: 30,
          totalItems: 0,
          totalPages: 0,
          items: [],
        })
      })
    )

    renderWithProviders(<MappingsList />)

    await waitFor(() => {
      expect(screen.getByText(/No mappings yet/)).toBeInTheDocument()
    })
  })

  it('renders a card per mapping with options and links to both playlists', async () => {
    // Default handler already returns 2 mappings
    renderWithProviders(<MappingsList />)

    await waitFor(() => {
      expect(screen.getByText('My Spotify Playlist')).toBeInTheDocument()
      expect(screen.getByText('My YouTube Playlist')).toBeInTheDocument()
      expect(screen.getByText('Another Playlist')).toBeInTheDocument()
      expect(screen.getByText('Another YT Playlist')).toBeInTheDocument()
    })

    expect(screen.getByText('My Spotify Playlist').closest('a')).toHaveAttribute(
      'href',
      expect.stringContaining('https://open.spotify.com/playlist/'),
    )
    expect(screen.getByText('My YouTube Playlist').closest('a')).toHaveAttribute(
      'href',
      expect.stringContaining('https://www.youtube.com/playlist?list='),
    )
    expect(screen.getAllByText('Name')).toHaveLength(1)
    expect(screen.getAllByText('Tracks')).toHaveLength(2)
    expect(screen.getByText(/Every hour from 00:00, every day/)).toBeInTheDocument()
    expect(screen.getByText(/Every 2 hours from 00:00, every day/)).toBeInTheDocument()
  })

  it('shows error message on API failure', async () => {
    server?.use(
      http.get('*/api/collections/mappings/records', () => {
        return new HttpResponse(null, { status: 500 })
      })
    )

    renderWithProviders(<MappingsList />)

    await waitFor(() => {
      expect(screen.getByText(/Error loading mappings/)).toBeInTheDocument()
    })
  })

  it('asks which playlists to delete, keeping both by default', async () => {
    const user = userEvent.setup()
    let deleteURL = ''
    server?.use(
      http.delete('*/api/collections/mappings/records/:id', ({ request }) => {
        deleteURL = request.url
        return HttpResponse.json({ deleted: true, playlists: [] })
      }),
    )

    renderWithProviders(<MappingsList />)

    const deleteButtons = await screen.findAllByRole('button', { name: 'Delete mapping' })
    expect(deleteButtons).toHaveLength(2)
    expect(screen.getAllByRole('button', { name: 'Sync now' })).toHaveLength(2)

    await user.click(deleteButtons[0])
    const dialog = await screen.findByRole('dialog')
    const spotifyBox = within(dialog).getByRole('checkbox', { name: 'Also delete the Spotify playlist' })
    const youtubeBox = within(dialog).getByRole('checkbox', { name: 'Also delete the YouTube playlist' })
    expect(spotifyBox).not.toBeChecked()
    expect(youtubeBox).not.toBeChecked()

    await user.click(youtubeBox)
    await user.click(within(dialog).getByRole('button', { name: 'Delete mapping and playlists' }))

    await waitFor(() => expect(deleteURL).toContain('/mapping1'))
    expect(deleteURL).toContain('delete_youtube_playlist=true')
    expect(deleteURL).not.toContain('delete_spotify_playlist=true')
  })
})
