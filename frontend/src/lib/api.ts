// API client for making requests to the backend

import { pb } from './pocketbase';
import type { PlaylistsResponse, SetupStatus } from './pocketbase';

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
      const err = error as { status?: number, message?: string };
      throw new ApiError(err.status ?? 500, err.message ?? 'Request failed');
    }
  },
}; 