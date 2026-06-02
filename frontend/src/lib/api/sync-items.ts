import { apiClient } from './client';

export type SyncItemOperation = 'add' | 'remove' | 'rename';
export type SyncItemService = 'spotify' | 'youtube';
export type SyncItemStatus = 'pending' | 'running' | 'done' | 'error' | 'skipped';

export interface SyncItemDetails {
  id: string;
  mapping_id: string;
  operation: SyncItemOperation;
  service: SyncItemService;
  track_id?: string;
  track_title?: string;
  track_artist?: string;
  status: SyncItemStatus;
  error_message?: string;
  attempt_count: number;
  last_attempt_at?: number;
  created: number;
  updated: number;
  source_service: SyncItemService;
  destination_service: SyncItemService;
  source_playlist_name?: string;
  destination_playlist_name?: string;
}

export interface SyncItemsListResponse {
  items: SyncItemDetails[];
  page: number;
  perPage: number;
  totalItems: number;
  totalPages: number;
}

export interface SyncItemsListOptions {
  page?: number;
  per_page?: number;
  status?: SyncItemStatus;
  service?: SyncItemService;
  operation?: SyncItemOperation;
  mapping_id?: string;
  sort?: 'created';
  order?: 'asc' | 'desc';
}

export interface SyncItemExecuteResponse extends SyncItemDetails {
  execution_log?: string;
}

export const syncItemsAPI = {
  async getList(options: SyncItemsListOptions = {}): Promise<SyncItemsListResponse> {
    const params: Record<string, string | number> = {
      page: options.page ?? 1,
      per_page: options.per_page ?? 20,
      sort: options.sort ?? 'created',
      order: options.order ?? 'desc',
    };
    if (options.status) params.status = options.status;
    if (options.service) params.service = options.service;
    if (options.operation) params.operation = options.operation;
    if (options.mapping_id) params.mapping_id = options.mapping_id;

    const url = apiClient.buildURL('/api/collections/sync_items/records', params);
    return apiClient.get<SyncItemsListResponse>(url);
  },

  async getOne(id: string): Promise<SyncItemDetails> {
    return apiClient.get<SyncItemDetails>(`/api/collections/sync_items/records/${id}`);
  },

  async execute(id: string): Promise<SyncItemExecuteResponse> {
    return apiClient.post<SyncItemExecuteResponse>(
      `/api/collections/sync_items/records/${id}/execute`,
    );
  },
};

export function isSyncItemExecutable(status: SyncItemStatus): boolean {
  return status === 'pending' || status === 'error';
}
