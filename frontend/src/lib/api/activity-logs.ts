import { apiClient } from './client';
import type { ActivityLogsResponse } from './types';

export interface ActivityLogsListOptions {
  page?: number;
  per_page?: number;
  job_type?: 'analysis' | 'executor' | 'system';
  level?: 'info' | 'warn' | 'error';
  mapping_id?: string;
  sync_item_id?: string;
}

export const activityLogsAPI = {
  // List activity logs with filtering
  async getList(options: ActivityLogsListOptions = {}): Promise<ActivityLogsResponse> {
    const params: Record<string, string | number> = {
      page: options.page || 1,
      per_page: options.per_page || 20,
    };

    if (options.job_type) {
      params.job_type = options.job_type;
    }
    if (options.level) {
      params.level = options.level;
    }
    if (options.mapping_id) {
      params.mapping_id = options.mapping_id;
    }
    if (options.sync_item_id) {
      params.sync_item_id = options.sync_item_id;
    }

    const url = apiClient.buildURL('/api/collections/activity_logs/records', params);
    return apiClient.get<ActivityLogsResponse>(url);
  },
};
