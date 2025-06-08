import { http, HttpResponse } from 'msw';

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8090';

// Helper to check if we should simulate authenticated state
function isAuthenticated() {
  // Check if we're in a browser environment and have the auth flag
  if (typeof window !== 'undefined') {
    return localStorage.getItem('msw-spotify-authenticated') === 'true';
  }
  return false;
}

export const handlers = [
  // Setup status endpoint
  http.get(`${API_BASE_URL}/api/setup/status`, () => {
    return HttpResponse.json({ required: false });
  }),

  // Spotify auth endpoints
  http.get(`${API_BASE_URL}/api/auth/spotify/login`, () => {
    // Mock redirect to Spotify
    return new HttpResponse(null, {
      status: 302,
      headers: {
        'Location': 'https://accounts.spotify.com/authorize?client_id=mock',
      },
    });
  }),

  http.get(`${API_BASE_URL}/api/auth/spotify/callback`, () => {
    // Mock successful callback
    return new HttpResponse(null, {
      status: 302,
      headers: {
        'Location': '/dashboard?spotify=connected',
      },
    });
  }),

  // Spotify playlists endpoint - dynamic based on auth state
  http.get(`${API_BASE_URL}/api/spotify/playlists`, ({ request }) => {
    // If not authenticated, return 401
    if (!isAuthenticated()) {
      return HttpResponse.json(
        { error: 'Not authenticated with Spotify' },
        { status: 401 }
      );
    }

    // Return authenticated response
    const url = new URL(request.url);
    const limit = parseInt(url.searchParams.get('limit') || '20');
    const offset = parseInt(url.searchParams.get('offset') || '0');

    // Mock playlist data
    const mockPlaylists = [
      {
        id: 'playlist1',
        name: 'My Awesome Playlist',
        description: 'A collection of my favorite songs',
        public: true,
        track_count: 42,
        owner: {
          id: 'user123',
          display_name: 'Test User',
        },
        images: [
          {
            url: 'https://via.placeholder.com/300',
            height: 300,
            width: 300,
          },
        ],
      },
      {
        id: 'playlist2',
        name: 'Chill Vibes',
        description: 'Perfect for relaxing',
        public: false,
        track_count: 28,
        owner: {
          id: 'user123',
          display_name: 'Test User',
        },
        images: [
          {
            url: 'https://via.placeholder.com/300',
            height: 300,
            width: 300,
          },
        ],
      },
    ];

    return HttpResponse.json({
      items: mockPlaylists.slice(offset, offset + limit),
      total: mockPlaylists.length,
      limit,
      offset,
      next: offset + limit < mockPlaylists.length ? `${API_BASE_URL}/api/spotify/playlists?limit=${limit}&offset=${offset + limit}` : '',
    });
  }),
];

// Handler for simulating unauthorized state - can be used to override the default
export const unauthorizedHandler = http.get(`${API_BASE_URL}/api/spotify/playlists`, () => {
  return HttpResponse.json(
    { error: 'Not authenticated with Spotify' },
    { status: 401 }
  );
}); 