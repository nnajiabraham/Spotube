import { useMutation, useQueryClient } from '@tanstack/react-query'
import { mappingsAPI } from './api/index'

/** Runs analysis + apply for one mapping and refreshes everything that depends on it. */
export function useSyncNow() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (id: string) => mappingsAPI.syncNow(id),
    onSettled: () => {
      for (const key of ['mappings', 'sync-items', 'dashboard-stats', 'activity-logs']) {
        queryClient.invalidateQueries({ queryKey: [key] })
      }
    },
  })
}
