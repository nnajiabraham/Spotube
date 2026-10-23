import { apiClient } from './client';
import type {
  Mapping,
  MappingsResponse,
  CreateMappingRequest,
  UpdateMappingRequest,
  DeleteMappingResult,
  MirrorRequest,
  MirrorResult,
  ScheduleRecommendation,
} from './types';

export interface MappingsListOptions {
  page?: number;
  per_page?: number;
  sort?: 'created' | 'updated';
  order?: 'asc' | 'desc';
}

export const mappingsAPI = {
  // List mappings with pagination and sorting
  async getList(options: MappingsListOptions = {}): Promise<MappingsResponse> {
    const params = {
      page: options.page || 1,
      per_page: options.per_page || 20,
      sort: options.sort || 'created',
      order: options.order || 'desc',
    };

    const url = apiClient.buildURL('/api/collections/mappings/records', params);
    return apiClient.get<MappingsResponse>(url);
  },

  // Get a single mapping by ID
  async getOne(id: string): Promise<Mapping> {
    return apiClient.get<Mapping>(`/api/collections/mappings/records/${id}`);
  },

  // Create a new mapping
  async create(data: CreateMappingRequest): Promise<Mapping> {
    return apiClient.post<Mapping>('/api/collections/mappings/records', data);
  },

  // Update an existing mapping
  async update(id: string, data: UpdateMappingRequest): Promise<Mapping> {
    return apiClient.patch<Mapping>(`/api/collections/mappings/records/${id}`, data);
  },

  // Analyze now and apply the queue (when auto-apply is on); returns the refreshed mapping
  async syncNow(id: string): Promise<Mapping> {
    return apiClient.post<Mapping>(`/api/collections/mappings/records/${id}/sync`, {});
  },

  // Delete a mapping, and optionally the playlists on either side
  async delete(
    id: string,
    options: { spotify?: boolean; youtube?: boolean } = {},
  ): Promise<DeleteMappingResult> {
    const url = apiClient.buildURL(`/api/collections/mappings/records/${id}`, {
      delete_spotify_playlist: options.spotify ? 'true' : '',
      delete_youtube_playlist: options.youtube ? 'true' : '',
    });
    return apiClient.deleteJSON<DeleteMappingResult>(url);
  },

  // A schedule for a new mapping that keeps reads affordable and spaces it from the others
  async recommend(params: { spotify_tracks: number; youtube_videos: number; tz?: string }): Promise<ScheduleRecommendation> {
    const url = apiClient.buildURL('/api/collections/mappings/records/schedule-recommendation', {
      spotify_tracks: params.spotify_tracks,
      youtube_videos: params.youtube_videos,
      tz: params.tz ?? '',
    });
    return apiClient.get<ScheduleRecommendation>(url);
  },

  // Create the other side of each playlist, each as its own mapping
  async mirror(data: MirrorRequest): Promise<{ items: MirrorResult[] }> {
    return apiClient.post('/api/collections/mappings/records/mirror', data);
  },
};
