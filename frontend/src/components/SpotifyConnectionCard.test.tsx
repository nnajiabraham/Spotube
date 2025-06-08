import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { SpotifyConnectionCard } from './SpotifyConnectionCard';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { server } from '../test/mocks/node';
import { http, HttpResponse } from 'msw';

// Mock the router Link component
vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, children, className }: any) => (
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
  it('shows loading state initially', () => {
    renderWithProviders(<SpotifyConnectionCard />);
    
    expect(screen.getByRole('status')).toBeInTheDocument();
  });

  it('shows connected state when authenticated', async () => {
    // Override the default handler to return authenticated response
    server.use(
      http.get('http://localhost:8090/api/spotify/playlists', () => {
        return HttpResponse.json({
          items: [{
            id: 'playlist1',
            name: 'Test Playlist',
            description: 'Test',
            public: true,
            track_count: 10,
            owner: { id: 'user1', display_name: 'Test User' },
            images: []
          }],
          total: 1,
          limit: 20,
          offset: 0,
          next: ''
        });
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
    server.use(
      http.get('http://localhost:8090/api/spotify/playlists', () => {
        return HttpResponse.json(
          { error: 'Not authenticated with Spotify' },
          { status: 401 }
        );
      })
    );
    
    renderWithProviders(<SpotifyConnectionCard />);
    
    await waitFor(() => {
      expect(screen.getByRole('link', { name: 'Connect Spotify' })).toBeInTheDocument();
    });
  });
}); 