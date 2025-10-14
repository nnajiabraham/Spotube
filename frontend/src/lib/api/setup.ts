import { apiClient } from './client';
import type { SetupStatus, SaveSettingsRequest } from './types';

export const setupAPI = {
  // Check if setup wizard is required
  async isRequired(): Promise<SetupStatus> {
    return apiClient.get<SetupStatus>('/api/setup/required');
  },

  // Save OAuth credentials
  async save(settings: SaveSettingsRequest): Promise<void> {
    return apiClient.post<void>('/api/setup/save', settings);
  },
};
