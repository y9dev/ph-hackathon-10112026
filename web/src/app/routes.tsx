// The single place features are registered. Adding a feature = new folder under
// src/features + one entry here (route, allowed roles and optional nav item).
import { lazy, type ComponentType, type LazyExoticComponent } from 'react';
import { Inbox, FileText, PlusCircle, BarChart3, Settings2, Bell, type LucideIcon } from 'lucide-react';
import type { Role } from '@/shared/api/types';
import { ru } from '@/shared/i18n/ru';

export interface AppRoute {
  path: string;
  component: LazyExoticComponent<ComponentType>;
  roles: Role[];
  nav?: {
    label: string;
    short?: string;
    icon: LucideIcon;
    /** Show in the mobile bottom bar for these roles. */
    mobile?: Role[];
    /** Sort order in the sidebar per role (lower first). */
    order: Partial<Record<Role, number>>;
  };
}

const ALL: Role[] = ['employee', 'manager', 'admin'];
const STAFF: Role[] = ['manager', 'admin'];

export const appRoutes: AppRoute[] = [
  {
    path: '/inbox',
    component: lazy(() => import('@/features/inbox/InboxPage')),
    roles: STAFF,
    nav: { label: ru.nav.inbox, icon: Inbox, mobile: STAFF, order: { manager: 1, admin: 1 } },
  },
  {
    path: '/my',
    component: lazy(() => import('@/features/my-requests/MyRequestsPage')),
    roles: ALL,
    nav: { label: ru.nav.mine, short: 'Мои', icon: FileText, mobile: ['employee', 'manager'], order: { employee: 1, manager: 2, admin: 4 } },
  },
  {
    path: '/new',
    component: lazy(() => import('@/features/submit/SubmitPage')),
    roles: ALL,
    nav: { label: ru.nav.new, short: ru.nav.newShort, icon: PlusCircle, mobile: ['employee'], order: { employee: 2, manager: 3, admin: 5 } },
  },
  { path: '/new/done/:id', component: lazy(() => import('@/features/submit/SuccessPage')), roles: ALL },
  { path: '/requests/:id', component: lazy(() => import('@/features/request-detail/RequestDetailPage')), roles: ALL },
  {
    path: '/analytics',
    component: lazy(() => import('@/features/analytics/AnalyticsPage')),
    roles: STAFF,
    nav: { label: ru.nav.analytics, icon: BarChart3, mobile: STAFF, order: { manager: 4, admin: 2 } },
  },
  {
    path: '/admin',
    component: lazy(() => import('@/features/admin/AdminPage')),
    roles: ['admin'],
    nav: { label: ru.nav.admin, short: ru.nav.adminShort, icon: Settings2, mobile: ['admin'], order: { admin: 3 } },
  },
  {
    path: '/notifications',
    component: lazy(() => import('@/features/notifications/NotificationsPage')),
    roles: ALL,
    nav: { label: ru.nav.notifications, icon: Bell, mobile: ALL, order: { employee: 3, manager: 5, admin: 6 } },
  },
];

export function navFor(role: Role) {
  return appRoutes
    .filter((r) => r.nav && r.roles.includes(role) && r.nav.order[role] !== undefined)
    .sort((a, b) => a.nav!.order[role]! - b.nav!.order[role]!);
}
