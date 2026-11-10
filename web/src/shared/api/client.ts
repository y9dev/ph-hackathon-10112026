import type {
  AdminUser,
  AdminUserInput,
  AnalyticsSummary,
  Category,
  CategoryInput,
  CommentKind,
  CreateRequest,
  DemoUsers,
  Department,
  ItemsOf,
  ListParams,
  Meta,
  NotificationList,
  RequestDetail,
  RequestList,
  RequestType,
  RoutingPreview,
  RoutingRule,
  RoutingRuleInput,
  Status,
  SuggestedGroup,
  Theme,
  User,
  ApiErrorBody,
} from './types';
import { ru } from '@/shared/i18n/ru';

export const API_BASE = '/api/v1';

/** Error thrown for any non-2xx response. `message` is safe to show (Russian). */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields: Record<string, string>;
  constructor(status: number, code: string, message: string, fields?: Record<string, string> | null) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.fields = fields ?? {};
  }
}

type UnauthorizedListener = () => void;
const unauthorizedListeners = new Set<UnauthorizedListener>();
/** Subscribe to 401 responses (used by the auth provider to redirect to /login). */
export function onUnauthorized(fn: UnauthorizedListener): () => void {
  unauthorizedListeners.add(fn);
  return () => unauthorizedListeners.delete(fn);
}

function defaultMessage(status: number): string {
  if (status === 401) return ru.errors.unauthorized;
  if (status === 403) return ru.errors.forbidden;
  if (status === 404) return ru.errors.notFound;
  if (status === 409) return ru.errors.conflict;
  if (status === 429) return ru.errors.tooMany;
  if (status >= 500) return ru.errors.server;
  return ru.errors.generic;
}

export function toQuery(params: Record<string, string | number | boolean | undefined | null>): string {
  const sp = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === null || v === '') continue;
    sp.set(k, String(v));
  }
  const s = sp.toString();
  return s ? `?${s}` : '';
}

interface RequestOptions {
  method?: string;
  body?: unknown;
  form?: FormData;
  /** Do not broadcast 401 (used by /auth/me and /auth/login). */
  silent401?: boolean;
}

export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' };
  let body: BodyInit | undefined;
  if (opts.form) {
    body = opts.form;
  } else if (opts.body !== undefined) {
    headers['Content-Type'] = 'application/json';
    body = JSON.stringify(opts.body);
  }
  let res: Response;
  try {
    res = await fetch(`${API_BASE}${path}`, {
      method: opts.method ?? (body ? 'POST' : 'GET'),
      headers,
      body,
      credentials: 'same-origin',
    });
  } catch {
    throw new ApiError(0, 'network', ru.errors.network);
  }
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  let data: unknown = undefined;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = undefined;
    }
  }
  if (!res.ok) {
    const err = (data as ApiErrorBody | undefined)?.error;
    if (res.status === 401 && !opts.silent401) unauthorizedListeners.forEach((fn) => fn());
    throw new ApiError(res.status, err?.code ?? String(res.status), err?.message || defaultMessage(res.status), err?.fields);
  }
  return data as T;
}

export const api = {
  login: (email: string, password: string) =>
    request<User>('/auth/login', { method: 'POST', body: { email, password }, silent401: true }),
  logout: () => request<void>('/auth/logout', { method: 'POST' }),
  me: () => request<User>('/auth/me', { silent401: true }),
  demoUsers: () => request<DemoUsers>('/auth/demo-users', { silent401: true }),
  meta: () => request<Meta>('/meta'),
  routingPreview: (category_id: number, type: RequestType) =>
    request<RoutingPreview>(`/routing/preview${toQuery({ category_id, type })}`),

  listRequests: (p: ListParams) => request<RequestList>(`/requests${toQuery({ ...p })}`),
  createRequest: (payload: CreateRequest, files: File[]) => {
    const fd = new FormData();
    fd.set('payload', JSON.stringify(payload));
    for (const f of files) fd.append('files', f, f.name);
    return request<RequestDetail>('/requests', { method: 'POST', form: fd });
  },
  exportCsvUrl: (p: Omit<ListParams, 'page' | 'page_size'>) => `${API_BASE}/requests/export.csv${toQuery({ ...p })}`,
  getRequest: (id: number) => request<RequestDetail>(`/requests/${id}`),
  changeStatus: (id: number, body: { status: Status; result_text?: string; duplicate_of_id?: number }) =>
    request<RequestDetail>(`/requests/${id}/status`, { method: 'POST', body }),
  assign: (id: number, body: { department_id?: number; assignee_id?: number | null }) =>
    request<RequestDetail>(`/requests/${id}/assign`, { method: 'POST', body }),
  addComment: (id: number, body: { kind: CommentKind; body: string }) =>
    request<RequestDetail>(`/requests/${id}/comments`, { method: 'POST', body }),
  linkTheme: (id: number, body: { theme_id?: number; title?: string }) =>
    request<RequestDetail>(`/requests/${id}/theme`, { method: 'POST', body }),
  attachmentUrl: (id: number) => `${API_BASE}/attachments/${id}`,

  notifications: (p: { unread?: boolean; limit?: number } = {}) =>
    request<NotificationList>(`/notifications${toQuery(p)}`),
  markRead: (body: { ids?: number[]; all?: boolean }) =>
    request<void>('/notifications/read', { method: 'POST', body }),

  analytics: (p: { from?: string; to?: string; department_id?: number }) =>
    request<AnalyticsSummary>(`/analytics/summary${toQuery(p)}`),
  suggestedGroups: () => request<ItemsOf<SuggestedGroup>>('/analytics/suggested-groups'),
  themes: () => request<ItemsOf<Theme>>('/themes'),

  adminCategories: () => request<ItemsOf<Category>>('/admin/categories'),
  createCategory: (b: CategoryInput) => request<Category>('/admin/categories', { method: 'POST', body: b }),
  updateCategory: (id: number, b: CategoryInput) =>
    request<Category>(`/admin/categories/${id}`, { method: 'PATCH', body: b }),
  adminRules: () => request<ItemsOf<RoutingRule>>('/admin/routing-rules'),
  createRule: (b: RoutingRuleInput) => request<RoutingRule>('/admin/routing-rules', { method: 'POST', body: b }),
  updateRule: (id: number, b: RoutingRuleInput) =>
    request<RoutingRule>(`/admin/routing-rules/${id}`, { method: 'PATCH', body: b }),
  deleteRule: (id: number) => request<void>(`/admin/routing-rules/${id}`, { method: 'DELETE' }),
  adminUsers: () => request<ItemsOf<AdminUser>>('/admin/users'),
  createUser: (b: AdminUserInput) => request<AdminUser>('/admin/users', { method: 'POST', body: b }),
  updateUser: (id: number, b: AdminUserInput) => request<AdminUser>(`/admin/users/${id}`, { method: 'PATCH', body: b }),
  adminDepartments: () => request<ItemsOf<Department>>('/admin/departments'),
};
