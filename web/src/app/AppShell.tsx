import { Suspense, useEffect, useRef, useState, type FormEvent } from 'react';
import { NavLink, Outlet, useLocation, useNavigate, Link } from 'react-router';
import { useQuery } from '@tanstack/react-query';
import { PanelLeftClose, PanelLeftOpen, Plus, Search, LogOut, Sun, Moon, Monitor, ChevronDown, FileText, FlaskConical } from 'lucide-react';
import { api, ApiError } from '@/shared/api/client';
import { qk } from '@/shared/api/queries';
import { cn } from '@/shared/lib/cn';
import { useTheme, type ThemePref } from '@/shared/lib/theme';
import { ru } from '@/shared/i18n/ru';
import {
  Avatar,
  Menu,
  MenuContent,
  MenuItem,
  MenuLabel,
  MenuRadioGroup,
  MenuRadioItem,
  MenuSeparator,
  MenuTrigger,
  Skeleton,
} from '@/shared/ui';
import { isStaff, useAuth, useUser } from '@/features/auth/AuthProvider';
import { NotificationsBell } from '@/features/notifications/NotificationsBell';
import { useNotifications } from '@/features/notifications/api';
import { Logo } from './Logo';
import { navFor } from './routes';

const SIDEBAR_KEY = 'mv-sidebar-collapsed';

function readCollapsed() {
  try {
    return localStorage.getItem(SIDEBAR_KEY) === '1';
  } catch {
    return false;
  }
}

export function AppShell() {
  const user = useUser();
  const location = useLocation();
  const [collapsed, setCollapsed] = useState(readCollapsed);
  const mainRef = useRef<HTMLElement>(null);
  const nav = navFor(user.role);
  const notif = useNotifications(8);
  const unread = notif.data?.unread_count ?? 0;
  const demo = useQuery({
    queryKey: qk.demoUsers,
    queryFn: async () => {
      try {
        return await api.demoUsers();
      } catch (e) {
        if (e instanceof ApiError && e.status === 404) return null;
        throw e;
      }
    },
    retry: false,
    staleTime: Infinity,
  });

  useEffect(() => {
    try {
      localStorage.setItem(SIDEBAR_KEY, collapsed ? '1' : '0');
    } catch {
      // non-essential preference
    }
  }, [collapsed]);

  // Move focus to the page on route change so keyboard and screen-reader users land on new content.
  const first = useRef(true);
  useEffect(() => {
    if (first.current) {
      first.current = false;
      return;
    }
    mainRef.current?.focus({ preventScroll: true });
    window.scrollTo({ top: 0 });
  }, [location.pathname]);

  const mobileNav = nav.filter((r) => r.nav!.mobile?.includes(user.role));
  const showFab = !location.pathname.startsWith('/new') && user.role !== 'employee';

  return (
    <div className="min-h-dvh bg-bg">
      <a
        href="#main"
        className="sr-only z-[70] rounded-lg bg-surface px-4 py-2 text-sm font-medium text-text shadow-md focus:not-sr-only focus:fixed focus:left-4 focus:top-4"
      >
        {ru.app.skipToContent}
      </a>

      {/* Sidebar (desktop) */}
      <aside
        className={cn(
          'fixed inset-y-0 left-0 z-30 hidden flex-col border-r border-border bg-surface transition-[width] duration-200 ease-out md:flex',
          collapsed ? 'w-[72px]' : 'w-64',
        )}
      >
        <div className={cn('flex h-16 items-center border-b border-border', collapsed ? 'justify-center px-2' : 'px-4')}>
          <Link to="/" aria-label={ru.app.name} className="rounded-lg">
            <Logo compact={collapsed} />
          </Link>
        </div>
        <nav aria-label={ru.nav.main} className="flex-1 overflow-y-auto p-3">
          <ul className="flex flex-col gap-1">
            {nav.map((r) => {
              const Icon = r.nav!.icon;
              const badge = r.path === '/notifications' && unread > 0 ? unread : null;
              return (
                <li key={r.path}>
                  <NavLink
                    to={r.path}
                    end={r.path === '/new'}
                    title={collapsed ? r.nav!.label : undefined}
                    className={({ isActive }) =>
                      cn(
                        'group relative flex h-10 items-center gap-3 rounded-lg text-sm font-medium transition-colors duration-150',
                        collapsed ? 'justify-center px-0' : 'px-3',
                        isActive ? 'bg-brand-soft text-brand' : 'text-muted hover:bg-surface-2 hover:text-text',
                      )
                    }
                  >
                    <Icon className="h-5 w-5 shrink-0" aria-hidden="true" />
                    <span className={cn(collapsed && 'sr-only')}>{r.nav!.label}</span>
                    {badge !== null && (
                      <span
                        className={cn(
                          'tabular ml-auto inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-brand px-1.5 text-[11px] font-semibold text-on-brand',
                          collapsed && 'absolute right-2 top-1 ml-0 h-4 min-w-4 px-1',
                        )}
                      >
                        {badge}
                        <span className="sr-only"> {ru.notifications.unreadSuffix}</span>
                      </span>
                    )}
                  </NavLink>
                </li>
              );
            })}
          </ul>
        </nav>
        <div className="border-t border-border p-3">
          {demo.data && !collapsed && (
            <p className="mb-2 flex items-center gap-2 rounded-lg bg-surface-2 px-3 py-2 text-xs text-muted">
              <FlaskConical className="h-4 w-4 shrink-0" aria-hidden="true" />
              {ru.app.demoBanner}
            </p>
          )}
          <button
            type="button"
            onClick={() => setCollapsed((c) => !c)}
            className={cn(
              'flex h-10 w-full items-center gap-3 rounded-lg text-sm text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-text',
              collapsed ? 'justify-center' : 'px-3',
            )}
            aria-label={collapsed ? ru.nav.expand : ru.nav.collapse}
            aria-expanded={!collapsed}
          >
            {collapsed ? <PanelLeftOpen className="h-5 w-5" aria-hidden="true" /> : <PanelLeftClose className="h-5 w-5" aria-hidden="true" />}
            {!collapsed && <span>{ru.nav.collapse}</span>}
          </button>
        </div>
      </aside>

      <div className={cn('flex min-h-dvh flex-col transition-[padding] duration-200 ease-out', collapsed ? 'md:pl-[72px]' : 'md:pl-64')}>
        <TopBar />
        <main
          id="main"
          ref={mainRef}
          tabIndex={-1}
          className="w-full flex-1 px-4 pb-24 pt-6 outline-none md:px-8 md:pb-12 md:pt-8"
        >
          <div className="mx-auto w-full max-w-[1200px]">
            <Suspense fallback={<PageFallback />}>
              <Outlet />
            </Suspense>
          </div>
        </main>
      </div>

      {/* FAB (mobile) */}
      {showFab && (
        <Link
          to="/new"
          aria-label={ru.nav.new}
          className="fixed bottom-20 right-4 z-30 inline-flex h-14 w-14 items-center justify-center rounded-full bg-brand text-on-brand shadow-lg transition-transform duration-150 hover:bg-brand-hover active:scale-95 md:hidden"
        >
          <Plus className="h-6 w-6" aria-hidden="true" />
        </Link>
      )}

      {/* Bottom tab bar (mobile) */}
      <nav
        aria-label={ru.nav.main}
        className="fixed inset-x-0 bottom-0 z-30 border-t border-border bg-surface pb-[env(safe-area-inset-bottom)] md:hidden"
      >
        <ul className="grid" style={{ gridTemplateColumns: `repeat(${mobileNav.length}, minmax(0, 1fr))` }}>
          {mobileNav.map((r) => {
            const Icon = r.nav!.icon;
            const badge = r.path === '/notifications' && unread > 0;
            return (
              <li key={r.path}>
                <NavLink
                  to={r.path}
                  end={r.path === '/new'}
                  className={({ isActive }) =>
                    cn(
                      'relative flex h-16 flex-col items-center justify-center gap-1 text-[11px] font-medium transition-colors duration-150',
                      isActive ? 'text-brand' : 'text-muted',
                    )
                  }
                >
                  <span className="relative">
                    <Icon className="h-5 w-5" aria-hidden="true" />
                    {badge && <span className="absolute -right-1 -top-1 h-2.5 w-2.5 rounded-full bg-brand ring-2 ring-surface" aria-hidden="true" />}
                  </span>
                  <span className="max-w-full truncate px-1">{r.nav!.short ?? r.nav!.label}</span>
                  {badge && <span className="sr-only">, {unread} {ru.notifications.unreadSuffix}</span>}
                </NavLink>
              </li>
            );
          })}
        </ul>
      </nav>
    </div>
  );
}

function TopBar() {
  const user = useUser();
  const { logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const { pref, setPref } = useTheme();
  const staff = isStaff(user.role);
  const onList = location.pathname === '/inbox' || location.pathname === '/my';
  const urlQ = onList ? (new URLSearchParams(location.search).get('q') ?? '') : '';
  const [q, setQ] = useState(urlQ);
  const [lastUrlQ, setLastUrlQ] = useState(urlQ);
  if (urlQ !== lastUrlQ) {
    setLastUrlQ(urlQ);
    setQ(urlQ);
  }

  function onSearch(e: FormEvent) {
    e.preventDefault();
    const term = q.trim();
    const sp = new URLSearchParams();
    if (term) sp.set('q', term);
    if (staff) {
      sp.set('tab', 'all');
      navigate(`/inbox?${sp}`);
    } else navigate(`/my${term ? `?${sp}` : ''}`);
  }

  const ThemeIcon = pref === 'dark' ? Moon : pref === 'light' ? Sun : Monitor;

  return (
    <header className="sticky top-0 z-20 flex h-16 items-center gap-3 border-b border-border bg-surface/90 px-4 backdrop-blur-md md:px-8">
      <Link to="/" className="rounded-lg md:hidden" aria-label={ru.app.name}>
        <Logo compact />
      </Link>
      <form role="search" onSubmit={onSearch} className="relative min-w-0 flex-1 md:max-w-md">
        <label htmlFor="global-search" className="sr-only">
          {ru.nav.search}
        </label>
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" aria-hidden="true" />
        <input
          id="global-search"
          type="search"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder={ru.nav.searchPlaceholder}
          className="h-10 w-full rounded-lg border border-border bg-surface-2 pl-9 pr-3 text-sm text-text placeholder:text-muted transition-[border-color,background-color,box-shadow] duration-150 hover:border-border-strong focus-visible:border-focus focus-visible:bg-surface focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus/30 max-md:h-11"
        />
      </form>
      <div className="ml-auto flex items-center gap-1">
        <NotificationsBell />
        <Menu>
          <MenuTrigger
            aria-label={ru.nav.userMenu}
            className="flex h-10 items-center gap-2 rounded-lg px-2 text-left transition-colors duration-150 hover:bg-surface-2 data-[state=open]:bg-surface-2 max-md:h-11"
          >
            <Avatar name={user.name} />
            <span className="hidden min-w-0 flex-col leading-tight lg:flex">
              <span className="max-w-40 truncate text-sm font-medium text-text">{user.name}</span>
              <span className="max-w-40 truncate text-xs text-muted">
                {ru.roles[user.role]}
                {user.department_name ? ` · ${user.department_name}` : ''}
              </span>
            </span>
            <ChevronDown className="hidden h-4 w-4 text-muted lg:block" aria-hidden="true" />
          </MenuTrigger>
          <MenuContent align="end">
            <MenuLabel>
              <span className="block text-sm font-medium text-text">{user.name}</span>
              <span className="block font-normal">{user.email}</span>
            </MenuLabel>
            <MenuSeparator />
            {user.role === 'admin' && (
              <MenuItem onSelect={() => navigate('/my')} className="md:hidden">
                <FileText className="h-4 w-4 text-muted" aria-hidden="true" />
                {ru.nav.mine}
              </MenuItem>
            )}
            <MenuLabel className="flex items-center gap-2">
              <ThemeIcon className="h-3.5 w-3.5" aria-hidden="true" />
              {ru.nav.theme}
            </MenuLabel>
            <MenuRadioGroup value={pref} onValueChange={(v) => setPref(v as ThemePref)}>
              <MenuRadioItem value="light">{ru.nav.themeLight}</MenuRadioItem>
              <MenuRadioItem value="dark">{ru.nav.themeDark}</MenuRadioItem>
              <MenuRadioItem value="system">{ru.nav.themeSystem}</MenuRadioItem>
            </MenuRadioGroup>
            <MenuSeparator />
            <MenuItem
              onSelect={async () => {
                await logout();
                navigate('/login', { replace: true });
              }}
            >
              <LogOut className="h-4 w-4 text-muted" aria-hidden="true" />
              {ru.nav.logout}
            </MenuItem>
          </MenuContent>
        </Menu>
      </div>
    </header>
  );
}

export function PageFallback() {
  return (
    <div role="status" aria-busy="true">
      <span className="sr-only">{ru.common.loading}</span>
      <Skeleton className="h-9 w-64" />
      <Skeleton className="mt-3 h-5 w-96 max-w-full" />
      <div className="mt-8 grid gap-4">
        {[0, 1, 2].map((i) => (
          <Skeleton key={i} className="h-24" />
        ))}
      </div>
    </div>
  );
}
