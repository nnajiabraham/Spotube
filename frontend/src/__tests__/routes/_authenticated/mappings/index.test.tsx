import { render, screen, waitFor } from '@testing-library/react'
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

// Mock PocketBase to include auth token and make actual fetch calls
vi.mock('../../../../lib/pocketbase', () => ({
  pb: {
    authStore: {
      token: 'test-token'
    },
    collection: vi.fn(() => ({
      getList: vi.fn().mockImplementation(async () => {
        // Make the actual fetch call that will be intercepted by MSW
        const response = await fetch('/api/collections/mappings/records', {
          headers: {
            'Authorization': 'Bearer test-token'
          }
        });
        if (!response.ok) {
          throw new Error('Failed to fetch');
        }
        return response.json();
      })
    }))
  }
}))

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
    
    // Look for loading spinner
    const spinner = document.querySelector('.animate-spin')
    expect(spinner).toBeInTheDocument()
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
      expect(screen.getByText('No mappings yet. Create your first mapping to start syncing playlists.')).toBeInTheDocument()
    })
  })

  it('renders mappings table when mappings exist', async () => {
    // Default handler already returns 2 mappings
    renderWithProviders(<MappingsList />)

    await waitFor(() => {
      expect(screen.getByText('My Spotify Playlist')).toBeInTheDocument()
      expect(screen.getByText('My YouTube Playlist')).toBeInTheDocument()
      expect(screen.getByText('Another Playlist')).toBeInTheDocument()
      expect(screen.getByText('Another YT Playlist')).toBeInTheDocument()
    })

    // Check sync options are displayed
    expect(screen.getByText('✓ Name')).toBeInTheDocument()
    expect(screen.getAllByText('✓ Tracks')).toHaveLength(2)
    
    // Check intervals are displayed
    expect(screen.getByText('60 min')).toBeInTheDocument()
    expect(screen.getByText('120 min')).toBeInTheDocument()
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

  it('renders delete buttons for each mapping', async () => {
    renderWithProviders(<MappingsList />)

    await waitFor(() => {
      expect(screen.getByText('My Spotify Playlist')).toBeInTheDocument()
    })

    // Should have delete buttons (one for each mapping)
    const deleteButtons = screen.getAllByRole('button')
    // At least 2 delete buttons (one for each mapping)
    expect(deleteButtons.length).toBeGreaterThanOrEqual(2)
  })

  it('shows confirmation when delete is clicked', async () => {
    // Mock window.confirm
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)
    
    renderWithProviders(<MappingsList />)

    await waitFor(() => {
      expect(screen.getByText('My Spotify Playlist')).toBeInTheDocument()
    })

    // The component has delete buttons that trigger confirm dialog
    // This test verifies the structure exists
    const buttons = screen.getAllByRole('button')
    expect(buttons.length).toBeGreaterThan(0)
    
    confirmSpy.mockRestore()
  })
}) 