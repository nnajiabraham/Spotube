import { apiClient } from './client';

// Simplified sync item type for modal display
export interface SyncItemDetails {
  id: string;
  mapping_id: string;
  operation: 'add' | 'remove' | 'rename';
  service: 'spotify' | 'youtube';
  track_id?: string;
  track_title?: string;
  track_artist?: string;
  status: 'pending' | 'running' | 'done' | 'error' | 'skipped';
  error_message?: string;
  attempt_count: number;
  last_attempt_at?: number; // epoch seconds
  created: number; // epoch seconds
  updated: number; // epoch seconds
}

export const syncItemsAPI = {
  // Get sync item details (for debugging/monitoring)
  async getOne(id: string): Promise<SyncItemDetails> {
    return apiClient.get<SyncItemDetails>(`/api/collections/sync_items/records/${id}`);
  },
};
