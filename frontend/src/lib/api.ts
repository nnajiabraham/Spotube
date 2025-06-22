// API client for making requests to the backend

import { pb } from './pocketbase';
import type {
  SetupStatus,
  PlaylistsResponse,
  YouTubePlaylistsResponse,
  Mapping,
  MappingsResponse,
  BlacklistResponse,
} from './pocketbase';

// Dashboard types
export interface DashboardStats {
  mappings: {
    total: number;
  };
  queue: {
    pending: number;
    running: number;
    errors: number;
    skipped: number;
    done: number;
  };
  recent_runs: Array<{
    timestamp: string;
    job_type: 'analysis' | 'execution' | 'system';
    status: 'success' | 'error' | 'info';
    message: string;
  }>;
  youtube_quota: {
    used: number;
    limit: number;
  };
}

// Activity logs types
export interface ActivityLog {
  id: string;
  level: 'info' | 'warn' | 'error';
  message: string;
  sync_item_id?: string;
  job_type: 'analysis' | 'execution' | 'system';
  created: string;
  updated: string;
}

export interface ActivityLogsResponse {
  page: number;
  perPage: number;
  totalItems: number;
  totalPages: number;
  items: ActivityLog[];
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

export class ApiError extends Error {
  status: number;
  
  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

export const api = {
  // Dashboard API
  getDashboardStats: async (): Promise<DashboardStats> => {
    try {
      const response = await pb.send('/api/dashboard/stats', {
        method: 'GET',
      });
      return response as DashboardStats;
    } catch (error: unknown) {
      const err = error as { status?: number, message?: string };
      throw new ApiError(err.status ?? 500, err.message ?? 'Request failed');
    }
  },

  // Setup API
  getSetupStatus: async (): Promise<SetupStatus> => {
    try {
      const response = await pb.send('/api/setup/status', {
        method: 'GET',
      });
      return response as SetupStatus;
    } catch (error: unknown) {
      const err = error as { status?: number, message?: string };
      throw new ApiError(err.status ?? 500, err.message ?? 'Request failed');
    }
  },
  
  // Spotify API
  getSpotifyPlaylists: async (params?: { limit?: number; offset?: number }): Promise<PlaylistsResponse> => {
    try {
      const searchParams = new URLSearchParams();
      if (params?.limit) searchParams.set('limit', params.limit.toString());
      if (params?.offset) searchParams.set('offset', params.offset.toString());
      
      const response = await pb.send('/api/spotify/playlists', {
        method: 'GET',
        query: searchParams,
      });
      return response as PlaylistsResponse;
    } catch (error: unknown) {
      if (error instanceof Error) {
        const apiError = error as ApiError;
        throw new ApiError(apiError.status || 500, apiError.message || 'Request failed');
      }
      throw new ApiError(500, 'An unknown error occurred');
    }
  },

  // YouTube API
  getYouTubePlaylists: async (): Promise<YouTubePlaylistsResponse> => {
    try {
      const response = await pb.send('/api/youtube/playlists', {
        method: 'GET',
      });
      return response as YouTubePlaylistsResponse;
    } catch (error: unknown) {
      const err = error as { status?: number, message?: string };
      throw new ApiError(err.status ?? 500, err.message ?? 'Request failed');
    }
  },

  // Mappings API
  getMappings: async (params?: { page?: number; perPage?: number }): Promise<MappingsResponse> => {
    try {
      return await pb.collection('mappings').getList(params?.page || 1, params?.perPage || 30);
    } catch (error: unknown) {
      const err = error as { status?: number, message?: string };
      throw new ApiError(err.status ?? 500, err.message ?? 'Request failed');
    }
  },

  getMapping: async (id: string): Promise<Mapping> => {
    try {
      return await pb.collection('mappings').getOne(id);
    } catch (error: unknown) {
      const err = error as { status?: number, message?: string };
      throw new ApiError(err.status ?? 500, err.message ?? 'Request failed');
    }
  },

  createMapping: async (data: Partial<Mapping>): Promise<Mapping> => {
    try {
      return await pb.collection('mappings').create(data);
    } catch (error: unknown) {
      const err = error as { status?: number, message?: string };
      throw new ApiError(err.status ?? 500, err.message ?? 'Request failed');
    }
  },

  updateMapping: async (id: string, data: Partial<Mapping>): Promise<Mapping> => {
    try {
      return await pb.collection('mappings').update(id, data);
    } catch (error: unknown) {
      const err = error as { status?: number, message?: string };
      throw new ApiError(err.status ?? 500, err.message ?? 'Request failed');
    }
  },

  deleteMapping: async (id: string): Promise<boolean> => {
    try {
      await pb.collection('mappings').delete(id);
      return true;
    } catch (error: unknown) {
      const err = error as { status?: number, message?: string };
      throw new ApiError(err.status ?? 500, err.message ?? 'Request failed');
    }
  },

  // Blacklist API
  getBlacklist: async (mappingId?: string, params?: { page?: number; perPage?: number }): Promise<BlacklistResponse> => {
    try {
      const filter = mappingId ? `mapping_id = "${mappingId}"` : '';
      return await pb.collection('blacklist').getList(params?.page || 1, params?.perPage || 30, {
        filter: filter,
        sort: '-created',
      });
    } catch (error: unknown) {
      const err = error as { status?: number, message?: string };
      throw new ApiError(err.status ?? 500, err.message ?? 'Request failed');
    }
  },

  deleteBlacklistEntry: async (id: string): Promise<boolean> => {
    try {
      await pb.collection('blacklist').delete(id);
      return true;
    } catch (error: unknown) {
      const err = error as { status?: number, message?: string };
      throw new ApiError(err.status ?? 500, err.message ?? 'Request failed');
    }
  },

  // Activity Logs API
  getActivityLogs: async (params?: {
    page?: number;
    perPage?: number;
    level?: string;
    job_type?: string;
  }): Promise<ActivityLogsResponse> => {
    try {
      const filters = [];
      if (params?.level) {
        filters.push(`level = "${params.level}"`);
      }
      if (params?.job_type) {
        filters.push(`job_type = "${params.job_type}"`);
      }
      const filter = filters.length > 0 ? filters.join(' && ') : '';

      return await pb.collection('activity_logs').getList(
        params?.page || 1, 
        params?.perPage || 50,
        {
          filter: filter,
          sort: '-created',
        }
      );
    } catch (error: unknown) {
      const err = error as { status?: number, message?: string };
      throw new ApiError(err.status ?? 500, err.message ?? 'Request failed');
    }
  },

  getSyncItem: async (id: string): Promise<SyncItem> => {
    try {
      return await pb.collection('sync_items').getOne(id);
    } catch (error: unknown) {
      const err = error as { status?: number, message?: string };
      throw new ApiError(err.status ?? 500, err.message ?? 'Request failed');
    }
  },
}; 