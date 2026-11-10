import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { api } from '@/shared/api/client';
import { qk, shouldRetry } from '@/shared/api/queries';

export const POLL_MS = 30_000;

/** Latest notifications + unread count, polled every 30 s (also refreshed on focus). */
export function useNotifications(limit = 8) {
  return useQuery({
    queryKey: [...qk.notifications, { limit }],
    queryFn: () => api.notifications({ limit }),
    refetchInterval: POLL_MS,
    refetchOnWindowFocus: true,
    retry: shouldRetry,
  });
}

export function useMarkRead() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: api.markRead,
    onSuccess: () => qc.invalidateQueries({ queryKey: qk.notifications }),
  });
}
