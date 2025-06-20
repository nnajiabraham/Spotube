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

export class ApiError extends Error {
  status: number;
  
  constructor(status: number, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

export const api = {
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
}; 