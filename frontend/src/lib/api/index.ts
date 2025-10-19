// Main API exports - replaces PocketBase SDK
export { apiClient, APIClientError, DateUtils } from './client';
export { setupAPI } from './setup';
export { oauthAPI } from './oauth';
export { mappingsAPI } from './mappings';
export { blacklistAPI } from './blacklist';
export { activityLogsAPI } from './activity-logs';
export { dashboardAPI } from './dashboard';
export { syncItemsAPI } from './sync-items';

// Import for legacy interface
import { mappingsAPI } from './mappings';
import { blacklistAPI } from './blacklist';
import { activityLogsAPI } from './activity-logs';
import { apiClient } from './client';

// Re-export types for convenience
export * from './types';

// Legacy PocketBase-style collection interface for easier migration
export const collections = {
  mappings: {
    getList: mappingsAPI.getList,
    getOne: mappingsAPI.getOne,
    create: mappingsAPI.create,
    update: mappingsAPI.update,
    delete: mappingsAPI.delete,
  },
  
  blacklist: {
    getList: blacklistAPI.getList,
    create: blacklistAPI.create,
    delete: blacklistAPI.delete,
  },
  
  activity_logs: {
    getList: activityLogsAPI.getList,
  },
};

// Health check endpoint
export const health = {
  async check(): Promise<{ status: string; timestamp: number; version: string; service: string }> {
    return apiClient.get('/api/health');
  },
};
