import { apiClient } from './client';
import type { DashboardStats } from './types';

export const dashboardAPI = {
  // Get dashboard statistics (unauthenticated)
  async getStats(): Promise<DashboardStats> {
    return apiClient.get<DashboardStats>('/api/dashboard/stats');
  },
};
