export { apiClient, APIClientError, DateUtils } from './client';
export { setupAPI } from './setup';
export { oauthAPI } from './oauth';
export { mappingsAPI } from './mappings';
export { blacklistAPI } from './blacklist';
export { activityLogsAPI } from './activity-logs';
export { dashboardAPI } from './dashboard';
export { syncItemsAPI } from './sync-items';

export * from './types';

import { apiClient } from './client';

export const health = {
  async check(): Promise<{ status: string; timestamp: number; version: string; service: string }> {
    return apiClient.get('/api/health');
  },
};
