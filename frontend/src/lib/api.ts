// API client for making requests to the backend

import { 
  setupAPI, 
  oauthAPI, 
  mappingsAPI, 
  blacklistAPI, 
  activityLogsAPI, 
  dashboardAPI,
  syncItemsAPI
} from './api/index';
import type {
  SetupStatus,
  SpotifyPlaylist,
  YouTubePlaylist,
  Mapping,
  MappingsResponse,
  BlacklistResponse,
  DashboardStats,
  ActivityLogsResponse,
  SaveSettingsRequest,
} from './api/types';

// Re-export types from new API (for backward compatibility)
export type { DashboardStats, ActivityLog, ActivityLogsResponse, SpotifyPlaylist, YouTubePlaylist, Mapping } from './api/types';

// Legacy response types for backward compatibility
export interface PlaylistsResponse {
  items: SpotifyPlaylist[];
  total: number;
  limit: number;
  offset: number;
  next: string;
}

export interface YouTubePlaylistsResponse {
  items: YouTubePlaylist[];
}

export interface SyncItem {
  id: string;
  mapping_id: string;
  service: 'spotify' | 'youtube';
  action: 'add_track' | 'remove_track' | 'rename_playlist';
  status: 'pending' | 'running' | 'done' | 'error' | 'skipped';
  source_track_id?: string;
  source_track_title?: string;
  source_service?: 'spotify' | 'youtube';
  destination_service?: 'spotify' | 'youtube';
  payload: string;
  attempts: number;
  last_error?: string;
  created: string;
  updated: string;
}

function mapOperationToAction(operation: 'add' | 'remove' | 'rename'): SyncItem['action'] {
  switch (operation) {
    case 'add':
      return 'add_track';
    case 'remove':
      return 'remove_track';
    case 'rename':
      return 'rename_playlist';
    default:
      return 'add_track';
  }
}

// Backward-compatible error class
export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

// Helper to convert APIClientError to ApiError for backward compatibility
type APIErrorPayload = { error?: { code?: string; message?: string } };
type APIClientErrorShape = { code?: string; message?: string } & Record<string, unknown>;

function convertError(error: unknown): ApiError {
  if (error instanceof ApiError) {
    return error;
  }

  const payload: APIErrorPayload | APIClientErrorShape | undefined =
    typeof error === 'object' && error !== null ? (error as APIClientErrorShape) : undefined;

  const code = (payload?.error as APIClientErrorShape | undefined)?.code ?? payload?.code;
  const message = (payload?.error as APIClientErrorShape | undefined)?.message ?? payload?.message ?? String(error);

  switch (code) {
    case 'unauthorized':
      return new ApiError(401, message ?? 'Unauthorized');
    case 'not_found':
      return new ApiError(404, message ?? 'Not found');
    case 'validation_failed':
      return new ApiError(422, message ?? 'Validation failed');
    case 'conflict':
      return new ApiError(409, message ?? 'Conflict');
    default:
      return new ApiError(500, message ?? 'Request failed');
  }
}

export { oauthAPI };

export const api = {
  // Dashboard API
  getDashboardStats: async (): Promise<DashboardStats> => {
    try {
      return await dashboardAPI.getStats();
    } catch (error) {
      throw convertError(error);
    }
  },

  // Setup API
  getSetupStatus: async (): Promise<SetupStatus> => {
    try {
      return await setupAPI.isRequired();
    } catch (error) {
      throw convertError(error);
    }
  },

  saveSetupCredentials: async (payload: SaveSettingsRequest): Promise<void> => {
    try {
      await setupAPI.save(payload);
    } catch (error) {
      throw convertError(error);
    }
  },
  
  // Spotify API  
  getSpotifyPlaylists: async (params?: { limit?: number; offset?: number }): Promise<PlaylistsResponse> => {
    try {
      const playlists = await oauthAPI.spotify.getPlaylists();
      // Convert to legacy format
      return {
        items: playlists,
        total: playlists.length,
        limit: params?.limit ?? 50,
        offset: params?.offset ?? 0,
        next: '', // Not supported in new API
      };
    } catch (error) {
      throw convertError(error);
    }
  },

  // YouTube API
  getYouTubePlaylists: async (): Promise<YouTubePlaylistsResponse> => {
    try {
      const playlists = await oauthAPI.youtube.getPlaylists();
      return {
        items: playlists,
      };
    } catch (error) {
      throw convertError(error);
    }
  },

  // Mappings API
  getMappings: async (params?: { page?: number; perPage?: number }): Promise<MappingsResponse> => {
    try {
      return await mappingsAPI.getList({
        page: params?.page ?? 1,
        per_page: params?.perPage ?? 30,
      });
    } catch (error) {
      throw convertError(error);
    }
  },

  getMapping: async (id: string): Promise<Mapping> => {
    try {
      return await mappingsAPI.getOne(id);
    } catch (error) {
      throw convertError(error);
    }
  },

  createMapping: async (data: Partial<Mapping>): Promise<Mapping> => {
    try {
      // Convert to new API format
      const createData = {
        spotify_playlist_id: data.spotify_playlist_id!,
        youtube_playlist_id: data.youtube_playlist_id!,
        spotify_playlist_name: data.spotify_playlist_name,
        youtube_playlist_name: data.youtube_playlist_name,
        sync_name: data.sync_name,
        sync_tracks: data.sync_tracks,
        interval_minutes: data.interval_minutes,
      };
      return await mappingsAPI.create(createData);
    } catch (error) {
      throw convertError(error);
    }
  },

  updateMapping: async (id: string, data: Partial<Mapping>): Promise<Mapping> => {
    try {
      return await mappingsAPI.update(id, {
        spotify_playlist_name: data.spotify_playlist_name,
        youtube_playlist_name: data.youtube_playlist_name,
        sync_name: data.sync_name,
        sync_tracks: data.sync_tracks,
        interval_minutes: data.interval_minutes,
      });
    } catch (error) {
      throw convertError(error);
    }
  },

  deleteMapping: async (id: string): Promise<boolean> => {
    try {
      await mappingsAPI.delete(id);
      return true;
    } catch (error) {
      throw convertError(error);
    }
  },

  // Blacklist API
  getBlacklist: async (mappingId?: string, params?: { page?: number; perPage?: number }): Promise<BlacklistResponse> => {
    try {
      return await blacklistAPI.getList({
        page: params?.page ?? 1,
        per_page: params?.perPage ?? 30,
        mapping_id: mappingId,
      });
    } catch (error) {
      throw convertError(error);
    }
  },

  deleteBlacklistEntry: async (id: string): Promise<boolean> => {
    try {
      await blacklistAPI.delete(id);
      return true;
    } catch (error) {
      throw convertError(error);
    }
  },

  // Activity Logs API
  getActivityLogs: async (params?: {
    page?: number;
    perPage?: number;
    level?: string;
    job_type?: string;
    sync_item_id?: string;
  }): Promise<ActivityLogsResponse> => {
    try {
      return await activityLogsAPI.getList({
        page: params?.page ?? 1,
        per_page: params?.perPage ?? 50,
        level: params?.level as 'info' | 'warn' | 'error' | undefined,
        job_type: params?.job_type as 'analysis' | 'executor' | 'system' | undefined,
        sync_item_id: params?.sync_item_id,
      });
    } catch (error) {
      throw convertError(error);
    }
  },

  // Get sync item details for modal display
  getSyncItem: async (id: string): Promise<SyncItem> => {
    try {
      const details = await syncItemsAPI.getOne(id);
      // Convert to legacy format expected by the component
      return {
        id: details.id,
        mapping_id: details.mapping_id,
        service: details.service,
        action: mapOperationToAction(details.operation),
        status: details.status,
        source_track_id: details.track_id || '',
        source_track_title: details.track_title || '',
        source_service: details.source_service,
        destination_service: details.destination_service,
        payload: '',
        attempts: details.attempt_count,
        last_error: details.error_message || '',
        created: new Date(details.created * 1000).toISOString(),
        updated: new Date(details.updated * 1000).toISOString(),
      };
    } catch (error) {
      throw convertError(error);
    }
  },
}; 