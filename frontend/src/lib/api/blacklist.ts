import { apiClient } from './client';
import type { BlacklistEntry, BlacklistResponse, CreateBlacklistRequest } from './types';

export interface BlacklistListOptions {
  page?: number;
  per_page?: number;
  mapping_id?: string;
}

export const blacklistAPI = {
  // List blacklist entries with filtering
  async getList(options: BlacklistListOptions = {}): Promise<BlacklistResponse> {
    const params: Record<string, string | number> = {
      page: options.page || 1,
      per_page: options.per_page || 20,
    };

    if (options.mapping_id) {
      params.mapping_id = options.mapping_id;
    }

    const url = apiClient.buildURL('/api/collections/blacklist/records', params);
    return apiClient.get<BlacklistResponse>(url);
  },

  // Create a blacklist entry
  async create(data: CreateBlacklistRequest): Promise<BlacklistEntry> {
    return apiClient.post<BlacklistEntry>('/api/collections/blacklist/records', data);
  },

  // Delete a blacklist entry
  async delete(id: string): Promise<void> {
    return apiClient.delete(`/api/collections/blacklist/records/${id}`);
  },
};
