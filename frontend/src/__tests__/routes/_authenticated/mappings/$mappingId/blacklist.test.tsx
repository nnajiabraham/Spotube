import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Route } from '../../../../../routes/_authenticated/mappings/$mappingId/blacklist.lazy';
import { http, HttpResponse } from 'msw';

// Mock TanStack Router - partial mock to preserve createLazyFileRoute
vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    Link: ({ to, children, className }: { to: string; children: React.ReactNode; className?: string }) => (
      <a href={to} className={className}>{children}</a>
    ),
    useParams: () => ({ mappingId: 'mapping1' }),
    useNavigate: () => vi.fn(),
  }
})

// Mock PocketBase to include auth token and make actual fetch calls
vi.mock('../../../../../lib/pocketbase', () => ({
  pb: {
    authStore: {
      token: 'test-token'
    },
    collection: vi.fn((name: string) => ({
      getList: vi.fn().mockImplementation(async (page: number, perPage: number, options?: Record<string, string>) => {
        // Make the actual fetch call that will be intercepted by MSW
        const params = new URLSearchParams({
          page: page.toString(),
          perPage: perPage.toString(),
        });
        if (options?.filter) {
          params.set('filter', options.filter);
        }
        if (options?.sort) {
          params.set('sort', options.sort);
        }
        
        const response = await fetch(`/api/collections/${name}/records?${params}`, {
          headers: {
            'Authorization': 'Bearer test-token'
          }
        });
        if (!response.ok) {
          throw new Error('Failed to fetch');
        }
        return response.json();
      }),
      getOne: vi.fn().mockImplementation(async (id: string) => {
        const response = await fetch(`/api/collections/${name}/records/${id}`, {
          headers: {
            'Authorization': 'Bearer test-token'
          }
        });
        if (!response.ok) {
          throw new Error('Failed to fetch');
        }
        return response.json();
      }),
      delete: vi.fn().mockImplementation(async (id: string) => {
        const response = await fetch(`/api/collections/${name}/records/${id}`, {
          method: 'DELETE',
          headers: {
            'Authorization': 'Bearer test-token'
          }
        });
        if (!response.ok) {
          throw new Error('Failed to delete');
        }
        return;
      })
    }))
  }
}))

// Extract the component from the Route
const BlacklistPage = Route.options.component as React.ComponentType;

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
};

describe('BlacklistPage', () => {
  const server = globalThis.mswServer

  it('displays loading state initially', async () => {
    renderWithProviders(<BlacklistPage />);

    // Look for loading spinner
    const spinner = document.querySelector('.animate-spin')
    expect(spinner).toBeInTheDocument()
  });

  it('displays blacklist entries for a mapping', async () => {
    renderWithProviders(<BlacklistPage />);

    await waitFor(() => {
      expect(screen.getByText('Blacklisted Tracks')).toBeInTheDocument();
    });

    // Check for service badges
    expect(screen.getByText('Spotify')).toBeInTheDocument();
    expect(screen.getByText('YouTube')).toBeInTheDocument();

    // Check for track IDs
    expect(screen.getByText('spotify_track_456')).toBeInTheDocument();
    expect(screen.getByText('youtube_video_789')).toBeInTheDocument();

    // Check for reasons
    expect(screen.getByText('not found')).toBeInTheDocument();
    expect(screen.getByText('forbidden')).toBeInTheDocument();

    // Check for skip counters
    expect(screen.getByText('2')).toBeInTheDocument();
    expect(screen.getByText('1')).toBeInTheDocument();
  });

  it('displays empty state when no blacklist entries', async () => {
    server?.use(
      http.get('*/api/collections/blacklist/records', () => {
        return HttpResponse.json({
          page: 1,
          perPage: 30,
          totalItems: 0,
          totalPages: 0,
          items: [],
        })
      })
    )

    renderWithProviders(<BlacklistPage />);

    await waitFor(() => {
      expect(screen.getByText('No blacklisted tracks')).toBeInTheDocument();
    });

    expect(screen.getByText('All tracks in this mapping are syncing successfully.')).toBeInTheDocument();
  });

  it('shows mapping information in the header', async () => {
    renderWithProviders(<BlacklistPage />);

    await waitFor(() => {
      expect(screen.getByText(/My Spotify Playlist/)).toBeInTheDocument();
      expect(screen.getByText(/My YouTube Playlist/)).toBeInTheDocument();
    });
  });

  it('displays service badges with correct colors', async () => {
    renderWithProviders(<BlacklistPage />);

    await waitFor(() => {
      const spotifyBadge = screen.getByText('Spotify');
      const youtubeBadge = screen.getByText('YouTube');

      expect(spotifyBadge).toHaveClass('bg-green-100', 'text-green-800');
      expect(youtubeBadge).toHaveClass('bg-red-100', 'text-red-800');
    });
  });

  it('displays reason badges with correct colors', async () => {
    renderWithProviders(<BlacklistPage />);

    await waitFor(() => {
      const notFoundBadge = screen.getByText('not found');
      const forbiddenBadge = screen.getByText('forbidden');

      expect(notFoundBadge).toHaveClass('bg-gray-100', 'text-gray-800');
      expect(forbiddenBadge).toHaveClass('bg-yellow-100', 'text-yellow-800');
    });
  });

  it('handles delete blacklist entry', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false)
    const user = userEvent.setup();
    renderWithProviders(<BlacklistPage />);

    await waitFor(() => {
      expect(screen.getByText('Blacklisted Tracks')).toBeInTheDocument();
    });

    // Find delete buttons
    const deleteButtons = screen.getAllByTitle('Remove from blacklist');
    expect(deleteButtons).toHaveLength(2);

    // Click first delete button
    await user.click(deleteButtons[0]);

    // Should show confirmation dialog
    expect(window.confirm).toHaveBeenCalledWith(
      'Remove this track from the blacklist? It will be retried in future sync attempts.'
    );
    
    confirmSpy.mockRestore()
  });

  it('formats dates correctly', async () => {
    renderWithProviders(<BlacklistPage />);

    await waitFor(() => {
      expect(screen.getByText('Blacklisted Tracks')).toBeInTheDocument();
    });

    // Check that dates are formatted properly (should contain Jan 1, 2024) - multiple entries expected
    const dateElements = screen.getAllByText(/Jan 1, 2024/);
    expect(dateElements.length).toBeGreaterThan(0);
  });

  it('displays error state when API fails', async () => {
    server?.use(
      http.get('*/api/collections/blacklist/records', () => {
        return new HttpResponse(null, { status: 500 })
      })
    )

    renderWithProviders(<BlacklistPage />);

    await waitFor(() => {
      expect(screen.getByText(/Error loading blacklist/)).toBeInTheDocument();
    });
  });

  it('has back link to mapping edit page', async () => {
    renderWithProviders(<BlacklistPage />);

    await waitFor(() => {
      expect(screen.getByText('Back to mapping')).toBeInTheDocument();
    });

    const backLink = screen.getByText('Back to mapping');
    // The mocked Link component returns the template, not the actual resolved path
    expect(backLink.closest('a')).toHaveAttribute('href', '/mappings/$mappingId/edit');
  });

  it('renders track IDs with proper styling', async () => {
    renderWithProviders(<BlacklistPage />);

    await waitFor(() => {
      expect(screen.getByText('Blacklisted Tracks')).toBeInTheDocument();
    });

    // Track IDs should be rendered and their table cells should have monospace font
    const trackIdCells = screen.getAllByText(/spotify_track_|youtube_video_/);
    expect(trackIdCells.length).toBeGreaterThan(0);
    
    trackIdCells.forEach(cell => {
      // Find the table cell that should have font-mono
      const tableCell = cell.closest('td');
      expect(tableCell).toHaveClass('font-mono');
    });
  });
}); 