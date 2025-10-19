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
  // Setup required endpoint (updated path)
  http.get(`${API_BASE_URL}/api/setup/required`, () => {
    return HttpResponse.json({ required: false });
  }),

  // Dashboard stats endpoint
  http.get(`${API_BASE_URL}/api/dashboard/stats`, () => {
    return HttpResponse.json({
      mappings: { total: 3 },
      queue: { 
        pending: 8, 
        running: 0, 
        error: 1, 
        skipped: 2, 
        done: 45 
      },
      recent_runs: [
        {
          timestamp: 1704110400, // 2024-01-01T12:00:00Z as epoch seconds
          job_type: 'analysis',
          level: 'info',
          message: 'Analysis completed successfully',
          mapping_id: 'mapping1'
        },
        {
          timestamp: 1704108600, // 2024-01-01T11:30:00Z as epoch seconds
          job_type: 'executor',
          level: 'info',
          message: 'Sync completed for mapping',
          mapping_id: 'mapping1'
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

  // Spotify playlists endpoint - updated path
  http.get(`${API_BASE_URL}/api/auth/spotify/playlists`, () => {
    // If not authenticated, return 401
    if (!isAuthenticated()) {
      return HttpResponse.json(
        { error: { code: 'unauthorized', message: 'Not authenticated with Spotify' } },
        { status: 401 }
      );
    }

    // Return authenticated response - simplified format for new API
    const mockPlaylists = [
      {
        id: 'playlist1',
        name: 'My Awesome Playlist',
      },
      {
        id: 'playlist2',
        name: 'Chill Vibes',
      },
    ];

    return HttpResponse.json(mockPlaylists);
  }),

  // YouTube playlists endpoint
  http.get(`${API_BASE_URL}/api/auth/youtube/playlists`, () => {
    // If not authenticated, return 401
    if (!isAuthenticated()) {
      return HttpResponse.json(
        { error: { code: 'unauthorized', message: 'Not authenticated with YouTube' } },
        { status: 401 }
      );
    }

    // Return authenticated response
    const mockPlaylists = [
      {
        id: 'yt_playlist1',
        name: 'My YouTube Playlist',
      },
      {
        id: 'yt_playlist2',  
        name: 'Another YT Playlist',
      },
    ];

    return HttpResponse.json(mockPlaylists);
  }),

  // Mappings handlers - updated for new API (no auth headers, epoch timestamps)
  http.get('*/api/collections/mappings/records', () => {
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
          last_analysis_at: null,
          tracks_count: 0,
          created: 1704067200, // 2024-01-01T00:00:00Z as epoch seconds
          updated: 1704067200,
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
          last_analysis_at: null,
          tracks_count: 0,
          created: 1704153600, // 2024-01-02T00:00:00Z as epoch seconds
          updated: 1704153600,
        },
      ],
    })
  }),

  http.get('*/api/collections/mappings/records/:id', ({ params }) => {
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
        last_analysis_at: null,
        tracks_count: 0,
        created: 1704067200,
        updated: 1704067200,
      })
    }

    return new HttpResponse(null, { status: 404 })
  }),

  http.post('*/api/collections/mappings/records', async ({ request }) => {
    const body = await request.json() as Record<string, unknown>
    const now = Math.floor(Date.now() / 1000);
    return HttpResponse.json({
      id: 'newmapping',
      last_analysis_at: null,
      tracks_count: 0,
      ...body,
      created: now,
      updated: now,
    })
  }),

  http.patch('*/api/collections/mappings/records/:id', async ({ params, request }) => {
    const { id } = params
    const body = await request.json() as Record<string, unknown>
    
    return HttpResponse.json({
      id,
      spotify_playlist_id: 'spotify123',
      youtube_playlist_id: 'youtube456',
      spotify_playlist_name: 'My Spotify Playlist',
      youtube_playlist_name: 'My YouTube Playlist',
      last_analysis_at: null,
      tracks_count: 0,
      ...body,
      created: 1704067200,
      updated: Math.floor(Date.now() / 1000),
    })
  }),

  http.delete('*/api/collections/mappings/records/:id', () => {
    return new HttpResponse(null, { status: 204 })
  }),

  // Blacklist handlers - updated for new API
  http.get('*/api/collections/blacklist/records', ({ request }) => {
    const url = new URL(request.url)
    const mappingId = url.searchParams.get('mapping_id')
    
    // Mock blacklist data with epoch timestamps
    const allBlacklistEntries = [
      {
        id: 'blacklist1',
        mapping_id: 'mapping1',
        service: 'spotify',
        track_id: 'spotify_track_456',
        reason: 'not found',
        skip_counter: 2,
        last_skipped_at: 1704110400, // 2024-01-01T12:00:00Z
        created: 1704067200, // 2024-01-01T00:00:00Z
        updated: 1704110400,
      },
      {
        id: 'blacklist2',
        mapping_id: 'mapping1',
        service: 'youtube',
        track_id: 'youtube_video_789',
        reason: 'forbidden',
        skip_counter: 1,
        last_skipped_at: 1704117600, // 2024-01-01T14:00:00Z
        created: 1704067200, // 2024-01-01T00:00:00Z
        updated: 1704117600,
      },
      {
        id: 'blacklist3',
        mapping_id: 'mapping2',
        service: 'spotify',
        track_id: 'spotify_track_123',
        reason: 'unauthorized',
        skip_counter: 3,
        last_skipped_at: 1704182400, // 2024-01-02T08:00:00Z
        created: 1704153600, // 2024-01-02T00:00:00Z
        updated: 1704182400,
      },
    ]

    // Filter by mapping_id if specified
    let filteredEntries = allBlacklistEntries
    if (mappingId) {
      filteredEntries = allBlacklistEntries.filter(entry => entry.mapping_id === mappingId)
    }

    return HttpResponse.json({
      page: 1,
      perPage: 30,
      totalItems: filteredEntries.length,
      totalPages: 1,
      items: filteredEntries,
    })
  }),

  http.delete('*/api/collections/blacklist/records/:id', () => {
    return new HttpResponse(null, { status: 204 })
  }),

  // Activity Logs handlers - updated for new API
  http.get('*/api/collections/activity_logs/records', ({ request }) => {
    const url = new URL(request.url)
    const level = url.searchParams.get('level')
    const jobType = url.searchParams.get('job_type')
    
    // Mock activity log data with epoch timestamps
    const allActivityLogs = [
      {
        id: 'log1',
        level: 'info',
        message: 'Starting sync analysis job',
        mapping_id: '',
        job_type: 'analysis',
        created: 1704110400, // 2024-01-01T12:00:00Z
      },
      {
        id: 'log2',
        level: 'info',
        message: 'Processing add_track action for "Song Title" (ID: track123) on spotify',
        mapping_id: 'mapping1',
        job_type: 'executor',
        created: 1704110700, // 2024-01-01T12:05:00Z
      },
      {
        id: 'log3',
        level: 'warn',
        message: 'Rate limit encountered, retrying in 30 seconds',
        mapping_id: 'mapping1',
        job_type: 'executor',
        created: 1704111000, // 2024-01-01T12:10:00Z
      },
      {
        id: 'log4',
        level: 'error',
        message: 'Failed to add track: Track not found',
        mapping_id: 'mapping1',
        job_type: 'executor',
        created: 1704111300, // 2024-01-01T12:15:00Z
      },
      {
        id: 'log5',
        level: 'info',
        message: 'Analysis complete: Found 5 items to sync',
        mapping_id: '',
        job_type: 'analysis',
        created: 1704111600, // 2024-01-01T12:20:00Z
      },
    ]

    // Filter by level and job_type if specified
    let filteredLogs = allActivityLogs
    if (level) {
      filteredLogs = filteredLogs.filter(log => log.level === level)
    }
    if (jobType) {
      filteredLogs = filteredLogs.filter(log => log.job_type === jobType)
    }

    return HttpResponse.json({
      page: 1,
      perPage: 50,
      totalItems: filteredLogs.length,
      totalPages: 1,
      items: filteredLogs,
    })
  }),

  // Sync Items handlers - updated for new API structure
  http.get('*/api/collections/sync_items/records/:id', ({ params }) => {
    const { id } = params
    
    // Mock sync item data matching new API structure
    const syncItems: Record<string, {
      id: string;
      mapping_id: string;
      operation: string;
      service: string;
      track_id: string;
      track_title: string;
      track_artist: string;
      status: string;
      error_message: string;
      attempt_count: number;
      last_attempt_at: number;
      created: number;
      updated: number;
    }> = {
      'sync_item_1': {
        id: 'sync_item_1',
        mapping_id: 'mapping1',
        operation: 'add',
        service: 'spotify',
        track_id: 'spotify_track_123',
        track_title: 'Test Song',
        track_artist: 'Test Artist',
        status: 'done',
        error_message: '',
        attempt_count: 1,
        last_attempt_at: 1704110700, // 2024-01-01T12:05:00Z
        created: 1704107200, // 2024-01-01T11:00:00Z
        updated: 1704110700,
      },
      'sync_item_2': {
        id: 'sync_item_2',
        mapping_id: 'mapping1',
        operation: 'add',
        service: 'spotify',
        track_id: 'youtube_video_456',
        track_title: 'Another Song',
        track_artist: 'Another Artist',
        status: 'running',
        error_message: 'Rate limit exceeded',
        attempt_count: 2,
        last_attempt_at: 1704111000, // 2024-01-01T12:10:00Z
        created: 1704109000, // 2024-01-01T11:30:00Z
        updated: 1704111000,
      },
    }

    if (syncItems[id as string]) {
      return HttpResponse.json(syncItems[id as string])
    }

    return new HttpResponse(null, { status: 404 })
  }),
];

// Handler for simulating unauthorized state - can be used to override the default
export const unauthorizedHandler = http.get(`${API_BASE_URL}/api/auth/spotify/playlists`, () => {
  return HttpResponse.json(
    { error: { code: 'unauthorized', message: 'Not authenticated with Spotify' } },
    { status: 401 }
  );
}); 