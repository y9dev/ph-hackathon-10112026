import { useQuery } from '@tanstack/react-query';
import { api, ApiError } from './client';

export const qk = {
  me: ['me'] as const,
  meta: ['meta'] as const,
  demoUsers: ['demo-users'] as const,
  requests: ['requests'] as const,
  request: (id: number) => ['request', id] as const,
  notifications: ['notifications'] as const,
  analytics: ['analytics'] as const,
  groups: ['suggested-groups'] as const,
  themes: ['themes'] as const,
  admin: ['admin'] as const,
};

export function useMeta(enabled = true) {
  return useQuery({ queryKey: qk.meta, queryFn: api.meta, staleTime: 5 * 60_000, enabled });
}

/** Retry policy: never retry 4xx (auth, validation, not found). */
export function shouldRetry(count: number, error: unknown): boolean {
  if (error instanceof ApiError && error.status >= 400 && error.status < 500) return false;
  return count < 2;
}
