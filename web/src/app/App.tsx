import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import * as RTooltip from '@radix-ui/react-tooltip';
import { lazy, Suspense, useState, type ReactNode } from 'react';
import { BrowserRouter, Navigate, Route, Routes, useLocation } from 'react-router';
import { Lock, SearchX } from 'lucide-react';
import type { Role } from '@/shared/api/types';
import { shouldRetry } from '@/shared/api/queries';
import { ru } from '@/shared/i18n/ru';
import { ButtonLink, EmptyState, ErrorState, ToastProvider, useDocumentTitle } from '@/shared/ui';
import { AuthProvider, homePath, useAuth } from '@/features/auth/AuthProvider';
import LoginPage from '@/features/auth/LoginPage';
import { AppShell, PageFallback } from './AppShell';
import { appRoutes } from './routes';
import { Logo } from './Logo';

// CR2 public pages: reachable without a session (anonymous authors use them from any device).
const AnonymousLookupPage = lazy(() => import('@/features/anonymous/AnonymousPages').then((m) => ({ default: m.AnonymousLookupPage })));
const AnonymousRequestPage = lazy(() => import('@/features/anonymous/AnonymousPages').then((m) => ({ default: m.AnonymousRequestPage })));

function RequireAuth({ children }: { children: ReactNode }) {
  const { user, loading, error, refetch } = useAuth();
  const location = useLocation();
  if (loading) return <BootScreen />;
  if (error && !user)
    return (
      <div className="mx-auto max-w-lg px-4 py-16">
        <ErrorState error={error} onRetry={refetch} />
      </div>
    );
  if (!user) {
    const next = location.pathname + location.search;
    return <Navigate to={next === '/' ? '/login' : `/login?next=${encodeURIComponent(next)}`} replace />;
  }
  return <>{children}</>;
}

function RequireRole({ roles, children }: { roles: Role[]; children: ReactNode }) {
  const { user } = useAuth();
  if (user && !roles.includes(user.role)) return <Forbidden />;
  return <>{children}</>;
}

function Forbidden() {
  useDocumentTitle(ru.pages.forbidden);
  const { user } = useAuth();
  return (
    <EmptyState
      icon={<Lock className="h-6 w-6" />}
      title={ru.errors.forbiddenTitle}
      text={ru.errors.forbidden}
      action={<ButtonLink to={user ? homePath(user.role) : '/'}>{ru.errors.toHome}</ButtonLink>}
    />
  );
}

function NotFound() {
  useDocumentTitle(ru.pages.notFound);
  return (
    <EmptyState
      icon={<SearchX className="h-6 w-6" />}
      title={ru.errors.notFoundTitle}
      text={ru.errors.notFound}
      action={<ButtonLink to="/">{ru.errors.toHome}</ButtonLink>}
    />
  );
}

function HomeRedirect() {
  const { user } = useAuth();
  return <Navigate to={user ? homePath(user.role) : '/login'} replace />;
}

function BootScreen() {
  return (
    <div className="flex min-h-dvh items-center justify-center bg-bg" role="status" aria-busy="true">
      <span className="sr-only">{ru.common.loading}</span>
      <Logo className="animate-pulse motion-reduce:animate-none" />
    </div>
  );
}

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route
        path="/anonymous"
        element={
          <Suspense fallback={<BootScreen />}>
            <AnonymousLookupPage />
          </Suspense>
        }
      />
      <Route
        path="/anonymous/request"
        element={
          <Suspense fallback={<BootScreen />}>
            <AnonymousRequestPage />
          </Suspense>
        }
      />
      <Route
        element={
          <RequireAuth>
            <AppShell />
          </RequireAuth>
        }
      >
        <Route index element={<HomeRedirect />} />
        {appRoutes.map((r) => (
          <Route
            key={r.path}
            path={r.path}
            element={
              <RequireRole roles={r.roles}>
                <Suspense fallback={<PageFallback />}>
                  <r.component />
                </Suspense>
              </RequireRole>
            }
          />
        ))}
        <Route path="/admin/:tab" element={<Navigate to="/admin" replace />} />
        <Route path="*" element={<NotFound />} />
      </Route>
    </Routes>
  );
}

export function App() {
  const [client] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: { retry: shouldRetry, staleTime: 15_000, refetchOnWindowFocus: false },
          mutations: { retry: false },
        },
      }),
  );
  return (
    <QueryClientProvider client={client}>
      <RTooltip.Provider delayDuration={200}>
        <ToastProvider>
          <BrowserRouter>
            <AuthProvider>
              <AppRoutes />
            </AuthProvider>
          </BrowserRouter>
        </ToastProvider>
      </RTooltip.Provider>
    </QueryClientProvider>
  );
}
