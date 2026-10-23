import { apiClient } from './client';
import type { ScoreBreakdown } from './tracks';

export type SyncItemOperation = 'add' | 'remove' | 'rename';
export type SyncItemService = 'spotify' | 'youtube';
export type SyncItemStatus = 'pending' | 'running' | 'done' | 'error' | 'skipped' | 'review';

/** A search result the executor considered (synccontext.SearchCandidate). */
export interface RunCandidate {
  rank: number;
  id: string;
  title: string;
  artist?: string;
  channel?: string;
  raw_title?: string;
  duration_ms?: number;
  image?: string;
  score: number;
  breakdown?: ScoreBreakdown;
}

/** What the executor did on its last attempt (synccontext.ExecutorRun). */
export interface ExecutorRun {
  source?: { platform?: string; id?: string; title?: string; artist?: string; channel?: string; duration_ms?: number };
  search_query?: string;
  candidates?: RunCandidate[];
  selected?: { id?: string; title?: string; artist?: string };
  already_in_playlist?: boolean;
  linked?: boolean;
  outcome?: string;
  error?: string;
}

export type ResolveAction =
  | { action: 'match'; target_id: string }
  | { action: 'unavailable' }
  | { action: 'retry' };

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
  analysis_context_json?: Record<string, unknown> | null;
  /** Latest executor run, when requested with include=last_run (always on getOne). */
  last_run?: ExecutorRun | null;
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
  /** Attach each item's latest executor run (candidates and scores). */
  include_last_run?: boolean;
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
    if (options.include_last_run) params.include = 'last_run';

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

  /** Decide an item the engine couldn't: pick the match, mark the song unavailable, or retry. */
  async resolve(id: string, body: ResolveAction): Promise<SyncItemExecuteResponse> {
    return apiClient.post<SyncItemExecuteResponse>(`/api/collections/sync_items/records/${id}/resolve`, body);
  },
};

export function isSyncItemExecutable(status: SyncItemStatus): boolean {
  return status === 'pending' || status === 'error';
}
