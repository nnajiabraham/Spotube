import { apiClient } from './client';
import type { AppSetting } from './types';

export const settingsAPI = {
  async list(): Promise<AppSetting[]> {
    return apiClient.get<{ items: AppSetting[] }>('/api/settings').then((r) => r.items);
  },

  // Change settings; all are validated before any is applied
  async update(values: Record<string, boolean | number | string>): Promise<AppSetting[]> {
    return apiClient.patch<{ items: AppSetting[] }>('/api/settings', values).then((r) => r.items);
  },
};
