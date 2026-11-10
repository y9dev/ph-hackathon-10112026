import { useQuery, useQueryClient } from '@tanstack/react-query';
import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { api, ApiError, onUnauthorized } from '@/shared/api/client';
import { qk, shouldRetry } from '@/shared/api/queries';
import type { Role, User } from '@/shared/api/types';
import { clearAllDrafts } from '@/features/submit/draft';

interface AuthState {
  user: User | null;
  loading: boolean;
  error: unknown;
  /** True when the session ended while the user was working (shown on the login page). */
  expired: boolean;
  setUser: (u: User | null) => void;
  logout: () => Promise<void>;
  refetch: () => void;
}

const Ctx = createContext<AuthState | null>(null);

async function fetchMe(): Promise<User | null> {
  try {
    return await api.me();
  } catch (e) {
    if (e instanceof ApiError && e.status === 401) return null;
    throw e;
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const qc = useQueryClient();
  const [expired, setExpired] = useState(false);
  const me = useQuery({ queryKey: qk.me, queryFn: fetchMe, staleTime: Infinity, retry: shouldRetry });

  useEffect(
    () =>
      onUnauthorized(() => {
        if (qc.getQueryData(qk.me)) setExpired(true);
        qc.setQueryData(qk.me, null);
      }),
    [qc],
  );

  const value = useMemo<AuthState>(
    () => ({
      user: me.data ?? null,
      loading: me.isPending,
      error: me.error,
      expired,
      setUser: (u) => {
        setExpired(false);
        // Drop data cached for a previous account before switching.
        qc.removeQueries({ predicate: (q) => q.queryKey[0] !== qk.me[0] });
        qc.setQueryData(qk.me, u);
      },
      logout: async () => {
        try {
          await api.logout();
        } finally {
          clearAllDrafts();
          setExpired(false);
          qc.setQueryData(qk.me, null);
          qc.removeQueries({ predicate: (q) => q.queryKey[0] !== qk.me[0] });
        }
      },
      refetch: () => void me.refetch(),
    }),
    [me, expired, qc],
  );
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useAuth(): AuthState {
  const c = useContext(Ctx);
  if (!c) throw new Error('useAuth must be used inside <AuthProvider>');
  return c;
}

/** The current user; only call inside routes guarded by <RequireAuth>. */
export function useUser(): User {
  const { user } = useAuth();
  if (!user) throw new Error('useUser called without a session');
  return user;
}

export const isStaff = (role: Role) => role === 'manager' || role === 'admin';

export function homePath(role: Role): string {
  return isStaff(role) ? '/inbox' : '/my';
}
