import type { InboxTab, ListParams, ListScope, ListSort, Status } from '@/shared/api/types';
import { STATUSES } from './status';

export interface InboxFilters {
  tab: InboxTab;
  status?: Status;
  category_id?: number;
  department_id?: number;
  assignee?: 'me' | 'none';
  overdue?: boolean;
  q?: string;
  sort: ListSort;
  page: number;
}

export const DEFAULT_FILTERS: InboxFilters = { tab: 'needs_action', sort: 'deadline', page: 1 };
export const PAGE_SIZE = 20;

const TABS: InboxTab[] = ['needs_action', 'in_progress', 'waiting_author', 'closed', 'all'];
const SORTS: ListSort[] = ['deadline', 'newest', 'oldest', 'updated'];

function posInt(v: string | null): number | undefined {
  if (!v) return undefined;
  const n = Number(v);
  return Number.isInteger(n) && n > 0 ? n : undefined;
}

/** URL query → filters. Unknown or malformed values fall back to defaults. */
export function filtersFromSearch(sp: URLSearchParams): InboxFilters {
  const tab = sp.get('tab') as InboxTab | null;
  const sort = sp.get('sort') as ListSort | null;
  const status = sp.get('status') as Status | null;
  const assignee = sp.get('assignee');
  const q = sp.get('q')?.trim();
  return {
    tab: tab && TABS.includes(tab) ? tab : DEFAULT_FILTERS.tab,
    sort: sort && SORTS.includes(sort) ? sort : DEFAULT_FILTERS.sort,
    page: posInt(sp.get('page')) ?? 1,
    status: status && STATUSES.includes(status) ? status : undefined,
    category_id: posInt(sp.get('category')),
    department_id: posInt(sp.get('department')),
    assignee: assignee === 'me' || assignee === 'none' ? assignee : undefined,
    overdue: sp.get('overdue') === '1' ? true : undefined,
    q: q ? q : undefined,
  };
}

/** Filters → URL query. Defaults are omitted to keep URLs short and shareable. */
export function filtersToSearch(f: InboxFilters): URLSearchParams {
  const sp = new URLSearchParams();
  if (f.tab !== DEFAULT_FILTERS.tab) sp.set('tab', f.tab);
  if (f.status) sp.set('status', f.status);
  if (f.category_id) sp.set('category', String(f.category_id));
  if (f.department_id) sp.set('department', String(f.department_id));
  if (f.assignee) sp.set('assignee', f.assignee);
  if (f.overdue) sp.set('overdue', '1');
  if (f.q) sp.set('q', f.q);
  if (f.sort !== DEFAULT_FILTERS.sort) sp.set('sort', f.sort);
  if (f.page > 1) sp.set('page', String(f.page));
  return sp;
}

export function filtersToParams(f: InboxFilters, scope: ListScope): ListParams {
  return {
    scope,
    tab: f.tab,
    status: f.status,
    category_id: f.category_id,
    department_id: f.department_id,
    assignee: f.assignee,
    overdue: f.overdue,
    q: f.q,
    sort: f.sort,
    page: f.page,
    page_size: PAGE_SIZE,
  };
}

/** Any filter narrowing the list beyond the tab. */
export function hasActiveFilters(f: InboxFilters): boolean {
  return Boolean(f.status || f.category_id || f.department_id || f.assignee || f.overdue || f.q);
}

/** Change one or more filters; anything other than page resets pagination. */
export function patchFilters(f: InboxFilters, patch: Partial<InboxFilters>): InboxFilters {
  const next = { ...f, ...patch };
  if (!('page' in patch)) next.page = 1;
  return next;
}
