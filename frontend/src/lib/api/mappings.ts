import { apiClient } from './client';
import type { 
  Mapping, 
  MappingsResponse, 
  CreateMappingRequest, 
  UpdateMappingRequest 
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

  // Delete a mapping
  async delete(id: string): Promise<void> {
    return apiClient.delete(`/api/collections/mappings/records/${id}`);
  },
};
