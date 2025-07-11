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

  // Dashboard stats endpoint
  http.get(`${API_BASE_URL}/api/dashboard/stats`, () => {
    return HttpResponse.json({
      mappings: { total: 3 },
      queue: { 
        pending: 8, 
        running: 0, 
        errors: 1, 
        skipped: 2, 
        done: 45 
      },
      recent_runs: [
        {
          timestamp: '2024-01-01T12:00:00Z',
          job_type: 'analysis',
          status: 'success',
          message: 'Analysis completed successfully'
        },
        {
          timestamp: '2024-01-01T11:30:00Z',
          job_type: 'execution',
          status: 'success',
          message: 'Sync completed for mapping'
        }
      ],
      youtube_quota: { used: 2500, limit: 10000 }
    });
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

  // Mappings handlers
  http.get('*/api/collections/mappings/records', ({ request }) => {
    const authHeader = request.headers.get('authorization')
    
    if (!authHeader) {
      return new HttpResponse(null, { status: 401 })
    }

    return HttpResponse.json({
      page: 1,
      perPage: 30,
      totalItems: 2,
      totalPages: 1,
      items: [
        {
          id: 'mapping1',
          spotify_playlist_id: 'spotify123',
          youtube_playlist_id: 'youtube456',
          spotify_playlist_name: 'My Spotify Playlist',
          youtube_playlist_name: 'My YouTube Playlist',
          sync_name: true,
          sync_tracks: true,
          interval_minutes: 60,
          created: '2024-01-01T00:00:00Z',
          updated: '2024-01-01T00:00:00Z',
        },
        {
          id: 'mapping2',
          spotify_playlist_id: 'spotify789',
          youtube_playlist_id: 'youtube012',
          spotify_playlist_name: 'Another Playlist',
          youtube_playlist_name: 'Another YT Playlist',
          sync_name: false,
          sync_tracks: true,
          interval_minutes: 120,
          created: '2024-01-02T00:00:00Z',
          updated: '2024-01-02T00:00:00Z',
        },
      ],
    })
  }),

  http.get('*/api/collections/mappings/records/:id', ({ params, request }) => {
    const authHeader = request.headers.get('authorization')
    
    if (!authHeader) {
      return new HttpResponse(null, { status: 401 })
    }

    const { id } = params
    if (id === 'mapping1') {
      return HttpResponse.json({
        id: 'mapping1',
        spotify_playlist_id: 'spotify123',
        youtube_playlist_id: 'youtube456',
        spotify_playlist_name: 'My Spotify Playlist',
        youtube_playlist_name: 'My YouTube Playlist',
        sync_name: true,
        sync_tracks: true,
        interval_minutes: 60,
        created: '2024-01-01T00:00:00Z',
        updated: '2024-01-01T00:00:00Z',
      })
    }

    return new HttpResponse(null, { status: 404 })
  }),

  http.post('*/api/collections/mappings/records', async ({ request }) => {
    const authHeader = request.headers.get('authorization')
    
    if (!authHeader) {
      return new HttpResponse(null, { status: 401 })
    }

    const body = await request.json() as Record<string, unknown>
    return HttpResponse.json({
      id: 'newmapping',
      ...body,
      created: new Date().toISOString(),
      updated: new Date().toISOString(),
    })
  }),

  http.patch('*/api/collections/mappings/records/:id', async ({ params, request }) => {
    const authHeader = request.headers.get('authorization')
    
    if (!authHeader) {
      return new HttpResponse(null, { status: 401 })
    }

    const { id } = params
    const body = await request.json() as Record<string, unknown>
    
    return HttpResponse.json({
      id,
      spotify_playlist_id: 'spotify123',
      youtube_playlist_id: 'youtube456',
      spotify_playlist_name: 'My Spotify Playlist',
      youtube_playlist_name: 'My YouTube Playlist',
      ...body,
      updated: new Date().toISOString(),
    })
  }),

  http.delete('*/api/collections/mappings/records/:id', ({ request }) => {
    const authHeader = request.headers.get('authorization')
    
    if (!authHeader) {
      return new HttpResponse(null, { status: 401 })
    }

    return new HttpResponse(null, { status: 204 })
  }),

  // Blacklist handlers
  http.get('*/api/collections/blacklist/records', ({ request }) => {
    const authHeader = request.headers.get('authorization')
    
    if (!authHeader) {
      return new HttpResponse(null, { status: 401 })
    }

    const url = new URL(request.url)
    const filter = url.searchParams.get('filter')
    
    // Mock blacklist data
    const allBlacklistEntries = [
      {
        id: 'blacklist1',
        mapping_id: 'mapping1',
        service: 'spotify',
        track_id: 'spotify_track_456',
        reason: 'not_found',
        skip_counter: 2,
        last_skipped_at: '2024-01-01T12:00:00Z',
        created: '2024-01-01T10:00:00Z',
        updated: '2024-01-01T12:00:00Z',
      },
      {
        id: 'blacklist2',
        mapping_id: 'mapping1',
        service: 'youtube',
        track_id: 'youtube_video_789',
        reason: 'forbidden',
        skip_counter: 1,
        last_skipped_at: '2024-01-01T14:00:00Z',
        created: '2024-01-01T14:00:00Z',
        updated: '2024-01-01T14:00:00Z',
      },
      {
        id: 'blacklist3',
        mapping_id: 'mapping2',
        service: 'spotify',
        track_id: 'spotify_track_123',
        reason: 'unauthorized',
        skip_counter: 3,
        last_skipped_at: '2024-01-02T08:00:00Z',
        created: '2024-01-01T20:00:00Z',
        updated: '2024-01-02T08:00:00Z',
      },
    ]

    // Filter by mapping_id if specified
    let filteredEntries = allBlacklistEntries
    if (filter) {
      const mappingIdMatch = filter.match(/mapping_id = "([^"]+)"/)
      if (mappingIdMatch) {
        const mappingId = mappingIdMatch[1]
        filteredEntries = allBlacklistEntries.filter(entry => entry.mapping_id === mappingId)
      }
    }

    return HttpResponse.json({
      page: 1,
      perPage: 30,
      totalItems: filteredEntries.length,
      totalPages: 1,
      items: filteredEntries,
    })
  }),

  http.delete('*/api/collections/blacklist/records/:id', ({ request }) => {
    const authHeader = request.headers.get('authorization')
    
    if (!authHeader) {
      return new HttpResponse(null, { status: 401 })
    }

    return new HttpResponse(null, { status: 204 })
  }),

  // Activity Logs handlers
  http.get('*/api/collections/activity_logs/records', ({ request }) => {
    const authHeader = request.headers.get('authorization')
    
    if (!authHeader) {
      return new HttpResponse(null, { status: 401 })
    }

    const url = new URL(request.url)
    const filter = url.searchParams.get('filter')
    
    // Mock activity log data
    const allActivityLogs = [
      {
        id: 'log1',
        level: 'info',
        message: 'Starting sync analysis job',
        sync_item_id: '',
        job_type: 'analysis',
        created: '2024-01-01T12:00:00Z',
        updated: '2024-01-01T12:00:00Z',
      },
      {
        id: 'log2',
        level: 'info',
        message: 'Processing add_track action for "Song Title" (ID: track123) on spotify',
        sync_item_id: 'sync_item_1',
        job_type: 'execution',
        created: '2024-01-01T12:05:00Z',
        updated: '2024-01-01T12:05:00Z',
      },
      {
        id: 'log3',
        level: 'warn',
        message: 'Rate limit encountered, retrying in 30 seconds',
        sync_item_id: 'sync_item_2',
        job_type: 'execution',
        created: '2024-01-01T12:10:00Z',
        updated: '2024-01-01T12:10:00Z',
      },
      {
        id: 'log4',
        level: 'error',
        message: 'Failed to add track: Track not found',
        sync_item_id: 'sync_item_3',
        job_type: 'execution',
        created: '2024-01-01T12:15:00Z',
        updated: '2024-01-01T12:15:00Z',
      },
      {
        id: 'log5',
        level: 'info',
        message: 'Analysis complete: Found 5 items to sync',
        sync_item_id: '',
        job_type: 'analysis',
        created: '2024-01-01T12:20:00Z',
        updated: '2024-01-01T12:20:00Z',
      },
    ]

    // Filter by level and job_type if specified
    let filteredLogs = allActivityLogs
    if (filter) {
      const levelMatch = filter.match(/level = "([^"]+)"/)
      const jobTypeMatch = filter.match(/job_type = "([^"]+)"/)
      
      if (levelMatch) {
        const level = levelMatch[1]
        filteredLogs = filteredLogs.filter(log => log.level === level)
      }
      
      if (jobTypeMatch) {
        const jobType = jobTypeMatch[1]
        filteredLogs = filteredLogs.filter(log => log.job_type === jobType)
      }
    }

    return HttpResponse.json({
      page: 1,
      perPage: 50,
      totalItems: filteredLogs.length,
      totalPages: 1,
      items: filteredLogs,
    })
  }),

  // Sync Items handlers
  http.get('*/api/collections/sync_items/records/:id', ({ params, request }) => {
    const authHeader = request.headers.get('authorization')
    
    if (!authHeader) {
      return new HttpResponse(null, { status: 401 })
    }

    const { id } = params
    
    // Mock sync item data
    const syncItems: Record<string, {
      id: string;
      mapping_id: string;
      service: string;
      action: string;
      status: string;
      source_track_id: string;
      source_track_title: string;
      source_service: string;
      destination_service: string;
      payload: string;
      attempts: number;
      last_error: string;
      created: string;
      updated: string;
    }> = {
      'sync_item_1': {
        id: 'sync_item_1',
        mapping_id: 'mapping1',
        service: 'spotify',
        action: 'add_track',
        status: 'done',
        source_track_id: 'spotify_track_123',
        source_track_title: 'Song Title',
        source_service: 'youtube',
        destination_service: 'spotify',
        payload: '{"track_id":"spotify_track_123"}',
        attempts: 1,
        last_error: '',
        created: '2024-01-01T11:00:00Z',
        updated: '2024-01-01T12:05:00Z',
      },
      'sync_item_2': {
        id: 'sync_item_2',
        mapping_id: 'mapping1',
        service: 'spotify',
        action: 'add_track',
        status: 'running',
        source_track_id: 'youtube_video_456',
        source_track_title: 'Another Song',
        source_service: 'youtube',
        destination_service: 'spotify',
        payload: '',
        attempts: 2,
        last_error: 'Rate limit exceeded',
        created: '2024-01-01T11:30:00Z',
        updated: '2024-01-01T12:10:00Z',
      },
      'sync_item_3': {
        id: 'sync_item_3',
        mapping_id: 'mapping1',
        service: 'spotify',
        action: 'add_track',
        status: 'error',
        source_track_id: 'youtube_video_789',
        source_track_title: 'Missing Song',
        source_service: 'youtube',
        destination_service: 'spotify',
        payload: '',
        attempts: 3,
        last_error: 'Track not found on Spotify',
        created: '2024-01-01T11:45:00Z',
        updated: '2024-01-01T12:15:00Z',
      },
    }

    if (syncItems[id as string]) {
      return HttpResponse.json(syncItems[id as string])
    }

    return new HttpResponse(null, { status: 404 })
  }),
];

// Handler for simulating unauthorized state - can be used to override the default
export const unauthorizedHandler = http.get(`${API_BASE_URL}/api/spotify/playlists`, () => {
  return HttpResponse.json(
    { error: 'Not authenticated with Spotify' },
    { status: 401 }
  );
}); 