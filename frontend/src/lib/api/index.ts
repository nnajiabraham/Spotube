// Main API exports
export { apiClient, APIClientError, DateUtils } from './client';
export { setupAPI } from './setup';
export { oauthAPI } from './oauth';
export { mappingsAPI } from './mappings';
export { blacklistAPI } from './blacklist';
export { activityLogsAPI } from './activity-logs';
export { dashboardAPI } from './dashboard';
export { syncItemsAPI } from './sync-items';
import { apiClient } from './client';

// Re-export types for convenience
export * from './types';

// Health check endpoint
export const health = {
  async check(): Promise<{ status: string; timestamp: number; version: string; service: string }> {
    return apiClient.get('/api/health');
  },
};
