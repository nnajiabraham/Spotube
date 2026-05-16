// API client facade for the Spotube backend

import { 
  setupAPI, 
  oauthAPI, 
  mappingsAPI, 
  blacklistAPI, 
  activityLogsAPI, 
  dashboardAPI,
  syncItemsAPI,
  APIClientError,
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
  CreateMappingRequest,
  UpdateMappingRequest,
} from './api/types';
import type { SyncItemDetails } from './api/sync-items';

export type { 
  DashboardStats, ActivityLog, ActivityLogsResponse, 
  SpotifyPlaylist, YouTubePlaylist, Mapping,
  CreateMappingRequest, UpdateMappingRequest,
} from './api/types';
export type { SyncItemDetails } from './api/sync-items';

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

function toApiError(error: unknown): ApiError {
  if (error instanceof ApiError) return error;
  if (error instanceof APIClientError) {
    const statusMap: Record<string, number> = {
      unauthorized: 401, not_found: 404, validation_failed: 422, conflict: 409,
    };
    return new ApiError(statusMap[error.code] ?? 500, error.message);
  }
  return new ApiError(500, String(error));
}

function wrap<T>(fn: () => Promise<T>): Promise<T> {
  return fn().catch((e) => { throw toApiError(e); });
}

export { oauthAPI };

export const api = {
  getDashboardStats: (): Promise<DashboardStats> =>
    wrap(() => dashboardAPI.getStats()),

  getSetupStatus: (): Promise<SetupStatus> =>
    wrap(() => setupAPI.isRequired()),

  saveSetupCredentials: (payload: SaveSettingsRequest): Promise<void> =>
    wrap(() => setupAPI.save(payload)),

  getSpotifyPlaylists: (): Promise<SpotifyPlaylist[]> =>
    wrap(() => oauthAPI.spotify.getPlaylists()),

  getYouTubePlaylists: (): Promise<YouTubePlaylist[]> =>
    wrap(() => oauthAPI.youtube.getPlaylists()),

  getMappings: (params?: { page?: number; perPage?: number }): Promise<MappingsResponse> =>
    wrap(() => mappingsAPI.getList({
      page: params?.page ?? 1,
      per_page: params?.perPage ?? 30,
    })),

  getMapping: (id: string): Promise<Mapping> =>
    wrap(() => mappingsAPI.getOne(id)),

  createMapping: (data: CreateMappingRequest): Promise<Mapping> =>
    wrap(() => mappingsAPI.create(data)),

  updateMapping: (id: string, data: UpdateMappingRequest): Promise<Mapping> =>
    wrap(() => mappingsAPI.update(id, data)),

  deleteMapping: (id: string): Promise<void> =>
    wrap(() => mappingsAPI.delete(id)),

  getBlacklist: (mappingId?: string, params?: { page?: number; perPage?: number }): Promise<BlacklistResponse> =>
    wrap(() => blacklistAPI.getList({
      page: params?.page ?? 1,
      per_page: params?.perPage ?? 30,
      mapping_id: mappingId,
    })),

  deleteBlacklistEntry: (id: string): Promise<void> =>
    wrap(() => blacklistAPI.delete(id)),

  getActivityLogs: (params?: {
    page?: number; perPage?: number; level?: string; job_type?: string;
  }): Promise<ActivityLogsResponse> =>
    wrap(() => activityLogsAPI.getList({
      page: params?.page ?? 1,
      per_page: params?.perPage ?? 50,
      level: params?.level as 'info' | 'warn' | 'error' | undefined,
      job_type: params?.job_type as 'analysis' | 'executor' | 'system' | undefined,
    })),

  getSyncItem: (id: string): Promise<SyncItemDetails> =>
    wrap(() => syncItemsAPI.getOne(id)),
};
