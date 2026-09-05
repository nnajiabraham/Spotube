import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { SpotifyConnectionCard } from './SpotifyConnectionCard';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { TEST_API_BASE_URL } from '../test/apiBaseUrl';

// Mock the router Link component
vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, children, className }: { to: string; children: React.ReactNode; className?: string }) => (
    <a href={to} className={className}>{children}</a>
  ),
}));

const renderWithProviders = (ui: React.ReactElement) => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
  
  return render(
    <QueryClientProvider client={queryClient}>
      {ui}
    </QueryClientProvider>
  );
};

describe('SpotifyConnectionCard', () => {
  const server = globalThis.mswServer;

  it('shows loading state initially', () => {
    renderWithProviders(<SpotifyConnectionCard />);
    
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('shows connected state when authenticated', async () => {
    // Override the default handler to return authenticated response
    server?.use(
      http.get(`${TEST_API_BASE_URL}/api/auth/spotify/playlists`, () => {
        return HttpResponse.json([{
          id: 'playlist1',
          name: 'Test Playlist',
        }]);
      })
    );
    
    renderWithProviders(<SpotifyConnectionCard />);
    
    await waitFor(() => {
      expect(screen.getByText('Spotify Connected')).toBeInTheDocument();
    });

    expect(screen.getByText('Your Spotify account is connected and ready to sync.')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'View Playlists' })).toBeInTheDocument();
  });

  it('shows connect button when not authenticated', async () => {
    // Override with unauthorized handler
    server?.use(
      http.get(`${TEST_API_BASE_URL}/api/auth/spotify/playlists`, () => {
        return HttpResponse.json(
          { error: { code: 'unauthorized', message: 'Not authenticated with Spotify' } },
          { status: 401 }
        );
      })
    );
    
    renderWithProviders(<SpotifyConnectionCard />);
    
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Connect Spotify' })).toBeInTheDocument()
    })
  });

  it('shows connect button for Echo-style 401 responses without retrying', async () => {
    server?.use(
      http.get(`${TEST_API_BASE_URL}/api/auth/spotify/playlists`, () => {
        return HttpResponse.json(
          { message: 'spotify account not connected' },
          { status: 401 },
        );
      }),
    );

    renderWithProviders(<SpotifyConnectionCard />);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Connect Spotify' })).toBeInTheDocument();
    });
    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });
}); 