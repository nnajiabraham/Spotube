import { apiClient } from './client';
import type { EstimatePair, QuotaEstimate, QuotaStatus } from './types';

export const quotaAPI = {
  // Today's YouTube quota budget and how long the queue needs
  async status(): Promise<QuotaStatus> {
    return apiClient.get<QuotaStatus>('/api/quota');
  },

  // Price proposed mappings: YouTube units and days to finish behind the queue
  async estimate(pairs: EstimatePair[]): Promise<QuotaEstimate> {
    return apiClient.post<QuotaEstimate>('/api/quota/estimate', { pairs });
  },
};
