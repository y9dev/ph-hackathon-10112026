// In-browser implementation of docs/api/openapi.yaml for VITE_MOCK=1.
// Follows the contract exactly: AuthorView masking, allowed_statuses,
// allowed_comment_kinds, SLA state, error envelope, scopes and tabs.
import type {
  AdminUser,
  AnalyticsSummary,
  CommentKind,
  CreateRequest,
  InboxTab,
  PersonView,
  RequestDetail,
  RequestSummary,
  RoutingRule,
  SlaState,
  Status,
  TimelineItem,
  User,
} from '../types';
import type { MockDb, MRequest, MUser } from './model';
import { SIMILAR_THRESHOLD, similarity } from './similarity';
import { DEMO_PASSWORD } from './seed';

const H = 3_600_000;
const TERMINAL: Status[] = ['implemented', 'declined', 'duplicate'];
const ANON_NAME = 'Автор (анонимно)';
const TRANSITIONS: Record<Status, Status[]> = {
  new: ['in_review', 'needs_info', 'accepted', 'declined', 'duplicate'],
  in_review: ['needs_info', 'accepted', 'declined', 'duplicate'],
  needs_info: ['in_review'],
  accepted: ['implemented', 'declined'],
  implemented: [],
  declined: [],
  duplicate: [],
};
const STATUS_LABEL: Record<Status, string> = {
  new: 'Новое',
  in_review: 'На рассмотрении',
  needs_info: 'Нужно уточнение',
  accepted: 'Принято',
  implemented: 'Реализовано',
  declined: 'Отклонено',
  duplicate: 'Дубликат',
};
export const LIMITS = {
  max_files: 5,
  max_file_mb: 10,
  max_total_mb: 10,
  allowed_extensions: ['png', 'jpg', 'jpeg', 'pdf', 'docx', 'xlsx', 'txt'],
  title_max: 140,
  body_max: 4000,
};

export class HttpError extends Error {
  status: number;
  code: string;
  fields?: Record<string, string>;
  constructor(status: number, code: string, message: string, fields?: Record<string, string>) {
    super(message);
    this.status = status;
    this.code = code;
    this.fields = fields;
  }
}

export interface MockRequest {
  method: string;
  path: string;
  query: URLSearchParams;
  json: () => unknown;
  form: FormData | null;
}
export interface MockResponse {
  status: number;
  body?: unknown;
  text?: string;
  contentType?: string;
  headers?: Record<string, string>;
}

const ok = (body: unknown, status = 200): MockResponse => ({ status, body });
const noContent = (): MockResponse => ({ status: 204 });
const bad = (fields: Record<string, string>, message = 'Проверьте заполнение полей') =>
  new HttpError(400, 'validation_failed', message, fields);
const notFound = () => new HttpError(404, 'not_found', 'Не найдено');
const forbidden = () => new HttpError(403, 'forbidden', 'Недостаточно прав');

export function createHandlers(db: MockDb, now: () => number = Date.now) {
  const iso = (t = now()) => new Date(t).toISOString();
  const nextId = () => ++db.seq;
  const user = (id: number | null) => (id === null ? undefined : db.users.find((u) => u.id === id));
  const dept = (id: number | null) => (id === null ? undefined : db.departments.find((d) => d.id === id));
  const cat = (id: number) => db.categories.find((c) => c.id === id);
  const ledIds = (u: MUser) => db.departments.filter((d) => d.head_user_id === u.id).map((d) => d.id);
  const isStaff = (u: MUser) => u.role === 'manager' || u.role === 'admin';

  function me(): MUser {
    const u = user(db.sessionUserId);
    if (!u || !u.active) throw new HttpError(401, 'unauthorized', 'Требуется вход');
    return u;
  }
  function staffOnly(): MUser {
    const u = me();
    if (!isStaff(u)) throw forbidden();
    return u;
  }
  function adminOnly(): MUser {
    const u = me();
    if (u.role !== 'admin') throw forbidden();
    return u;
  }

  function toUser(u: MUser): User {
    return {
      id: u.id,
      name: u.name,
      email: u.email,
      role: u.role,
      department_id: u.department_id,
      department_name: dept(u.department_id)?.name ?? null,
      led_department_ids: ledIds(u),
    };
  }

  function inStaffScope(u: MUser, r: MRequest): boolean {
    if (u.role === 'admin') return true;
    if (u.role !== 'manager') return false;
    return (r.department_id !== null && ledIds(u).includes(r.department_id)) || r.assignee_id === u.id;
  }
  const canSee = (u: MUser, r: MRequest) => r.author_id === u.id || inStaffScope(u, r);

  function staffRole(u: MUser): string {
    if (u.role === 'admin') return 'Администратор';
    return ledIds(u).length ? 'Руководитель' : 'Ответственный';
  }

  /** The single AuthorView mapper: every person in every response goes through here. */
  function personView(viewer: MUser, r: MRequest, personId: number | null): PersonView | null {
    if (personId === null) return null;
    const p = user(personId);
    if (personId === r.author_id) {
      if (viewer.id === personId) return { name: p?.name ?? '', anonymous: r.is_anonymous, is_me: true, role: null };
      if (r.is_anonymous) return { name: ANON_NAME, anonymous: true, is_me: false, role: null };
      return { name: p?.name ?? '', anonymous: false, is_me: false, role: null };
    }
    return { name: p?.name ?? '', anonymous: false, is_me: viewer.id === personId, role: p ? staffRole(p) : null };
  }

  function sla(r: MRequest): SlaState {
    const t = now();
    const base = {
      first_response_due: r.first_response_due,
      resolution_due: r.resolution_due,
      first_responded_at: r.first_responded_at,
    };
    if (TERMINAL.includes(r.status)) return { ...base, state: 'done', overdue_kind: null, overdue_hours: 0 };
    if (r.status === 'needs_info') return { ...base, state: 'paused', overdue_kind: null, overdue_hours: 0 };
    const fr = Date.parse(r.first_response_due);
    const rs = Date.parse(r.resolution_due);
    if (!r.first_responded_at && t > fr)
      return { ...base, state: 'overdue', overdue_kind: 'first_response', overdue_hours: Math.floor((t - fr) / H) };
    if (t > rs) return { ...base, state: 'overdue', overdue_kind: 'resolution', overdue_hours: Math.floor((t - rs) / H) };
    const due = r.first_responded_at ? rs : fr;
    const total = due - Date.parse(r.created_at);
    const left = due - t;
    const state = total > 0 && left / total < 0.25 ? 'soon' : 'ok';
    return { ...base, state, overdue_kind: null, overdue_hours: 0 };
  }
  const isOverdue = (r: MRequest) => sla(r).state === 'overdue';

  function summary(viewer: MUser, r: MRequest): RequestSummary {
    const c = cat(r.category_id);
    const d = dept(r.department_id);
    const a = user(r.assignee_id);
    const commentsCount = db.comments.filter(
      (cm) => cm.request_id === r.id && (cm.kind !== 'internal' || inStaffScope(viewer, r)),
    ).length;
    return {
      id: r.id,
      number: r.number,
      type: r.type,
      category: { id: r.category_id, name: c?.name ?? '' },
      title: r.title,
      status: r.status,
      author: personView(viewer, r, r.author_id)!,
      department: d ? { id: d.id, name: d.name } : null,
      assignee: a ? { id: a.id, name: a.name } : null,
      routing_source: r.routing_source,
      sla: sla(r),
      comments_count: commentsCount,
      awaiting_author: r.status === 'needs_info',
      created_at: r.created_at,
      updated_at: r.updated_at,
      closed_at: r.closed_at,
    };
  }

  function similarTo(r: MRequest) {
    return db.requests
      .filter((o) => o.id !== r.id)
      .map((o) => ({ o, score: similarity(r, o) }))
      .filter((x) => x.score >= SIMILAR_THRESHOLD)
      .sort((a, b) => b.score - a.score);
  }

  function detail(viewer: MUser, r: MRequest): RequestDetail {
    const staff = inStaffScope(viewer, r);
    const isAuthor = r.author_id === viewer.id;
    const items: TimelineItem[] = [];
    for (const e of db.events.filter((e) => e.request_id === r.id)) {
      items.push({
        id: `e-${e.id}`,
        at: e.created_at,
        actor: personView(viewer, r, e.actor_id),
        kind: e.kind,
        from_status: e.from_status,
        to_status: e.to_status,
        text: e.text ?? null,
      });
    }
    for (const c of db.comments.filter((c) => c.request_id === r.id)) {
      if (c.kind === 'internal' && !staff) continue; // authors never receive internal notes
      items.push({ id: `c-${c.id}`, at: c.created_at, actor: personView(viewer, r, c.author_id), kind: c.kind, text: c.body });
    }
    items.sort((a, b) => a.at.localeCompare(b.at) || a.id.localeCompare(b.id));

    const terminal = TERMINAL.includes(r.status);
    let allowed: Status[] = [];
    const kinds: CommentKind[] = [];
    if (staff) {
      allowed = TRANSITIONS[r.status].filter((s) => s !== 'needs_info');
      if (!terminal) kinds.push('comment');
      if (r.status === 'new' || r.status === 'in_review') {
        kinds.push('question');
        allowed = [...allowed, 'needs_info'];
      }
      kinds.push('internal');
    }
    if (isAuthor && !terminal) {
      if (!kinds.includes('comment')) kinds.unshift('comment');
      if (r.status === 'needs_info') kinds.push('answer');
    }
    const sims = similarTo(r);
    const dup = r.duplicate_of ? db.requests.find((x) => x.id === r.duplicate_of) : undefined;
    const theme = r.theme_id ? db.themes.find((t) => t.id === r.theme_id) : undefined;
    return {
      ...summary(viewer, r),
      body: r.body,
      proposed_solution: r.proposed_solution,
      result_text: r.result_text,
      duplicate_of: dup ? { id: dup.id, number: dup.number } : null,
      theme: theme ? { id: theme.id, title: theme.title } : null,
      attachments: r.attachments,
      timeline: items,
      allowed_statuses: allowed,
      allowed_comment_kinds: kinds,
      can_assign: staff && !terminal,
      similar: {
        count: sims.length,
        items: staff
          ? sims
              .filter((x) => inStaffScope(viewer, x.o))
              .slice(0, 5)
              .map((x) => ({ id: x.o.id, number: x.o.number, title: x.o.title, status: x.o.status, score: Math.round(x.score * 100) / 100 }))
          : undefined,
      },
    };
  }

  function notify(userId: number | null | undefined, r: MRequest, kind: string, title: string, body: string, exceptId?: number) {
    if (!userId || userId === exceptId) return;
    db.notifications.push({ id: nextId(), user_id: userId, kind, request_id: r.id, title, body, read: false, created_at: iso() });
  }
  /** Staff owners of a request: assignee, else department head, else admins. */
  function owners(r: MRequest): number[] {
    if (r.assignee_id) return [r.assignee_id];
    const head = dept(r.department_id)?.head_user_id;
    if (head) return [head];
    return db.users.filter((u) => u.role === 'admin' && u.active).map((u) => u.id);
  }
  function touchFirstResponse(actor: MUser, r: MRequest) {
    if (!r.first_responded_at && actor.id !== r.author_id) r.first_responded_at = iso();
  }
  function getVisible(viewer: MUser, id: number): MRequest {
    const r = db.requests.find((x) => x.id === id);
    if (!r || !canSee(viewer, r)) throw notFound();
    return r;
  }
  function getStaffScoped(viewer: MUser, id: number): MRequest {
    const r = getVisible(viewer, id);
    if (!inStaffScope(viewer, r)) throw forbidden();
    return r;
  }
  function resumeFromPause(r: MRequest) {
    if (!r.paused_at) return;
    const shift = now() - Date.parse(r.paused_at);
    r.first_response_due = iso(Date.parse(r.first_response_due) + shift);
    r.resolution_due = iso(Date.parse(r.resolution_due) + shift);
    r.paused_at = null;
  }
  function setStatus(actor: MUser, r: MRequest, to: Status, text: string | null) {
    const from = r.status;
    r.status = to;
    r.updated_at = iso();
    if (to === 'needs_info') r.paused_at = iso();
    if (from === 'needs_info' && to !== 'needs_info') resumeFromPause(r);
    if (TERMINAL.includes(to)) r.closed_at = iso();
    db.events.push({ id: nextId(), request_id: r.id, actor_id: actor.id, kind: 'status_changed', from_status: from, to_status: to, text, created_at: iso() });
  }

  function matchTab(r: MRequest, tab: InboxTab): boolean {
    switch (tab) {
      case 'needs_action':
        return r.status === 'new' || ((r.status === 'in_review' || r.status === 'accepted') && isOverdue(r));
      case 'in_progress':
        return r.status === 'in_review' || r.status === 'accepted';
      case 'waiting_author':
        return r.status === 'needs_info';
      case 'closed':
        return TERMINAL.includes(r.status);
      default:
        return true;
    }
  }

  function route(category_id: number, type: string) {
    return (
      db.rules.find((r) => r.active && r.category_id === category_id && r.request_type === type) ??
      db.rules.find((r) => r.active && r.category_id === category_id && r.request_type === null)
    );
  }

  function validateCreate(p: Partial<CreateRequest>): Record<string, string> {
    const f: Record<string, string> = {};
    if (p.type !== 'problem' && p.type !== 'idea') f.type = 'Выберите тип обращения';
    const c = typeof p.category_id === 'number' ? cat(p.category_id) : undefined;
    if (!c || !c.active) f.category_id = 'Выберите категорию';
    const title = (p.title ?? '').trim();
    if (title.length < 5) f.title = 'Минимум 5 символов';
    else if (title.length > LIMITS.title_max) f.title = `Не больше ${LIMITS.title_max} символов`;
    const body = (p.body ?? '').trim();
    if (body.length < 20) f.body = 'Минимум 20 символов';
    else if (body.length > LIMITS.body_max) f.body = `Не больше ${LIMITS.body_max} символов`;
    if ((p.proposed_solution ?? '').length > 2000) f.proposed_solution = 'Не больше 2000 символов';
    return f;
  }

  function ruleView(r: (typeof db.rules)[number]): RoutingRule {
    return {
      ...r,
      category_name: cat(r.category_id)?.name ?? '',
      department_name: dept(r.department_id)?.name ?? '',
      assignee_name: user(r.assignee_user_id)?.name ?? null,
    };
  }
  function validateRule(b: Record<string, unknown>): Record<string, string> {
    const f: Record<string, string> = {};
    if (!cat(Number(b.category_id))) f.category_id = 'Выберите категорию';
    if (!dept(Number(b.department_id))) f.department_id = 'Выберите отдел';
    if (b.request_type !== null && b.request_type !== undefined && b.request_type !== 'problem' && b.request_type !== 'idea')
      f.request_type = 'Неверный тип';
    for (const k of ['first_response_hours', 'resolution_hours']) {
      const v = Number(b[k]);
      if (!Number.isInteger(v) || v < 1) f[k] = 'Не меньше 1 часа';
    }
    return f;
  }
  const adminUser = (u: MUser): AdminUser => ({ id: u.id, name: u.name, email: u.email, role: u.role, department_id: u.department_id, active: u.active });

  function analytics(viewer: MUser, q: URLSearchParams): AnalyticsSummary {
    const from = q.get('from') ? Date.parse(`${q.get('from')}T00:00:00`) : -Infinity;
    const to = q.get('to') ? Date.parse(`${q.get('to')}T23:59:59`) : Infinity;
    const depId = q.get('department_id') ? Number(q.get('department_id')) : null;
    const scoped = db.requests.filter((r) => inStaffScope(viewer, r) && (depId === null || r.department_id === depId));
    const inPeriod = scoped.filter((r) => {
      const t = Date.parse(r.created_at);
      return t >= from && t <= to;
    });
    const total = inPeriod.length;
    const responded = inPeriod.filter((r) => r.first_responded_at);
    const hours = responded
      .map((r) => (Date.parse(r.first_responded_at!) - Date.parse(r.created_at)) / H)
      .sort((a, b) => a - b);
    const pct = (p: number) => (hours.length ? Math.round(hours[Math.min(hours.length - 1, Math.floor(p * (hours.length - 1)))] * 10) / 10 : null);
    const closed = inPeriod.filter((r) => TERMINAL.includes(r.status));
    const statuses: Status[] = ['new', 'in_review', 'needs_info', 'accepted', 'implemented', 'declined', 'duplicate'];
    const catNames = [...new Set(inPeriod.map((r) => r.category_id))];
    const depNames = [...new Set(inPeriod.map((r) => r.department_id))];
    // time series: daily for <= 31 days, weekly otherwise
    const start = Number.isFinite(from) ? from : Math.min(...inPeriod.map((r) => Date.parse(r.created_at)), now());
    const end = Number.isFinite(to) ? Math.min(to, now()) : now();
    const days = Math.max(1, Math.ceil((end - start) / (24 * H)));
    const step = days > 31 ? 7 : 1;
    const series: AnalyticsSummary['timeseries'] = [];
    for (let t = startOfDay(start); t <= end; t += step * 24 * H) {
      const tEnd = t + step * 24 * H;
      series.push({
        date: localDate(t),
        created: scoped.filter((r) => inRange(r.created_at, t, tEnd)).length,
        closed: scoped.filter((r) => r.closed_at && inRange(r.closed_at, t, tEnd)).length,
      });
    }
    return {
      kpis: {
        total,
        reviewed_share: total ? responded.length / total : 0,
        median_first_response_hours: pct(0.5),
        p90_first_response_hours: pct(0.9),
        outcome_share: closed.length ? closed.filter((r) => r.result_text).length / closed.length : 0,
        implemented_count: inPeriod.filter((r) => r.status === 'implemented').length,
        overdue_count: scoped.filter(isOverdue).length,
        auto_routed_share: total ? inPeriod.filter((r) => r.routing_source === 'rule').length / total : 0,
      },
      by_status: statuses.map((s) => ({ status: s, count: inPeriod.filter((r) => r.status === s).length })),
      by_category: catNames
        .map((id) => {
          const rs = inPeriod.filter((r) => r.category_id === id);
          return {
            category: cat(id)?.name ?? '',
            count: rs.length,
            implemented: rs.filter((r) => r.status === 'implemented').length,
            declined: rs.filter((r) => r.status === 'declined').length,
            open: rs.filter((r) => !TERMINAL.includes(r.status)).length,
          };
        })
        .sort((a, b) => b.count - a.count),
      by_department: depNames
        .map((id) => {
          const rs = inPeriod.filter((r) => r.department_id === id);
          return {
            department: dept(id)?.name ?? 'Не распределено',
            count: rs.length,
            open: rs.filter((r) => !TERMINAL.includes(r.status)).length,
            overdue: rs.filter(isOverdue).length,
          };
        })
        .sort((a, b) => b.count - a.count),
      timeseries: series,
      overdue: scoped
        .filter(isOverdue)
        .sort((a, b) => sla(b).overdue_hours - sla(a).overdue_hours)
        .map((r) => summary(viewer, r)),
    };
  }

  function suggestedGroups(viewer: MUser) {
    const pool = db.requests.filter((r) => inStaffScope(viewer, r) && !r.theme_id && r.status !== 'duplicate');
    const used = new Set<number>();
    const groups = [];
    for (const r of pool) {
      if (used.has(r.id)) continue;
      const members = [r, ...pool.filter((o) => o.id !== r.id && !used.has(o.id) && similarity(r, o) >= SIMILAR_THRESHOLD)];
      if (members.length < 2) continue;
      members.forEach((m) => used.add(m.id));
      let sum = 0;
      let n = 0;
      for (let i = 0; i < members.length; i++)
        for (let j = i + 1; j < members.length; j++) {
          sum += similarity(members[i], members[j]);
          n += 1;
        }
      groups.push({
        key: `g-${members.map((m) => m.id).join('-')}`,
        title: r.title,
        count: members.length,
        score: Math.round((sum / n) * 100) / 100,
        categories: [...new Set(members.map((m) => cat(m.category_id)?.name ?? ''))],
        requests: members.map((m) => ({ id: m.id, number: m.number, title: m.title, status: m.status })),
      });
    }
    return groups.sort((a, b) => b.count - a.count || b.score - a.score);
  }

  type Handler = (req: MockRequest, m: RegExpMatchArray) => MockResponse;
  const routes: [string, RegExp, Handler][] = [
    [
      'POST',
      /^\/auth\/login$/,
      (req) => {
        const b = req.json() as { email?: string; password?: string };
        if (db.failedLogins >= 5) {
          throw new HttpError(429, 'too_many_requests', 'Слишком много попыток входа. Подождите минуту.');
        }
        const u = db.users.find((x) => x.email.toLowerCase() === (b.email ?? '').trim().toLowerCase());
        if (!u || u.password !== b.password || !u.active) {
          db.failedLogins += 1;
          throw new HttpError(401, 'unauthorized', 'Неверная почта или пароль');
        }
        db.failedLogins = 0;
        db.sessionUserId = u.id;
        return ok(toUser(u));
      },
    ],
    [
      'POST',
      /^\/auth\/logout$/,
      () => {
        db.sessionUserId = null;
        return noContent();
      },
    ],
    ['GET', /^\/auth\/me$/, () => ok(toUser(me()))],
    [
      'GET',
      /^\/auth\/demo-users$/,
      () =>
        ok({
          password_hint: DEMO_PASSWORD,
          users: [1, 2, 3, 4].map((id) => {
            const u = user(id)!;
            return { email: u.email, name: u.name, role: u.role, title: u.title };
          }),
        }),
    ],
    [
      'GET',
      /^\/meta$/,
      () => {
        const u = me();
        return ok({
          categories: db.categories.filter((c) => c.active).sort((a, b) => a.sort - b.sort),
          departments: db.departments.map((d) => ({ ...d, head_name: user(d.head_user_id)?.name ?? null })),
          statuses: (Object.keys(STATUS_LABEL) as Status[]).map((s) => ({ id: s, label: STATUS_LABEL[s], terminal: TERMINAL.includes(s) })),
          limits: LIMITS,
          assignable_users: isStaff(u)
            ? db.users
                .filter((x) => x.active && isStaff(x) && x.department_id !== null)
                .map((x) => ({ id: x.id, name: x.name, department_id: x.department_id! }))
            : [],
        });
      },
    ],
    [
      'GET',
      /^\/routing\/preview$/,
      (req) => {
        me();
        const cid = Number(req.query.get('category_id'));
        const type = req.query.get('type') ?? '';
        if (!cat(cid)) throw bad({ category_id: 'Выберите категорию' });
        if (type !== 'problem' && type !== 'idea') throw bad({ type: 'Выберите тип' });
        const rule = route(cid, type);
        const d = rule ? dept(rule.department_id) : undefined;
        const fr = rule?.first_response_hours ?? 24;
        const monthAgo = now() - 30 * 24 * H;
        return ok({
          routed: Boolean(rule),
          department: d ? { ...d, head_name: user(d.head_user_id)?.name ?? null } : null,
          assignee_name: rule ? (user(rule.assignee_user_id ?? d?.head_user_id ?? null)?.name ?? null) : null,
          first_response_hours: fr,
          resolution_hours: rule?.resolution_hours ?? 120,
          first_response_due: iso(now() + fr * H),
          similar_count: db.requests.filter((r) => r.category_id === cid && Date.parse(r.created_at) > monthAgo).length,
        });
      },
    ],
    [
      'GET',
      /^\/requests$/,
      (req) => {
        const u = me();
        const q = req.query;
        const scope = q.get('scope');
        if (scope !== 'mine' && scope !== 'inbox' && scope !== 'all') throw bad({ scope: 'Неверный scope' });
        if (scope !== 'mine' && !isStaff(u)) throw forbidden();
        let base =
          scope === 'mine'
            ? db.requests.filter((r) => r.author_id === u.id)
            : db.requests.filter((r) => inStaffScope(u, r));
        const status = q.get('status');
        const cid = q.get('category_id');
        const did = q.get('department_id');
        const assignee = q.get('assignee');
        const text = q.get('q')?.trim().toLowerCase();
        if (status) base = base.filter((r) => r.status === status);
        if (cid) base = base.filter((r) => r.category_id === Number(cid));
        if (did) base = base.filter((r) => r.department_id === Number(did));
        if (assignee === 'me') base = base.filter((r) => r.assignee_id === u.id);
        if (assignee === 'none') base = base.filter((r) => r.assignee_id === null);
        if (q.get('overdue') === 'true') base = base.filter(isOverdue);
        if (text)
          base = base.filter((r) =>
            [r.number, r.title, r.body, r.result_text ?? ''].some((s) => s.toLowerCase().includes(text)),
          );
        const counts =
          scope === 'mine'
            ? null
            : {
                needs_action: base.filter((r) => matchTab(r, 'needs_action')).length,
                in_progress: base.filter((r) => matchTab(r, 'in_progress')).length,
                waiting_author: base.filter((r) => matchTab(r, 'waiting_author')).length,
                closed: base.filter((r) => matchTab(r, 'closed')).length,
                all: base.length,
                overdue: base.filter(isOverdue).length,
              };
        const tab = (q.get('tab') as InboxTab | null) ?? 'all';
        let items = scope === 'mine' ? base : base.filter((r) => matchTab(r, tab));
        const sort = q.get('sort') ?? 'deadline';
        const due = (r: MRequest) => Date.parse(r.first_responded_at ? r.resolution_due : r.first_response_due);
        items = [...items].sort((a, b) => {
          if (sort === 'newest') return b.created_at.localeCompare(a.created_at);
          if (sort === 'oldest') return a.created_at.localeCompare(b.created_at);
          if (sort === 'updated') return b.updated_at.localeCompare(a.updated_at);
          // deadline: overdue first, then nearest due; closed last
          const ta = TERMINAL.includes(a.status) ? 1 : 0;
          const tb = TERMINAL.includes(b.status) ? 1 : 0;
          if (ta !== tb) return ta - tb;
          const oa = isOverdue(a) ? 0 : 1;
          const ob = isOverdue(b) ? 0 : 1;
          if (oa !== ob) return oa - ob;
          return due(a) - due(b);
        });
        const page = Math.max(1, Number(q.get('page') ?? 1));
        const size = Math.min(100, Math.max(1, Number(q.get('page_size') ?? 20)));
        return ok({
          items: items.slice((page - 1) * size, page * size).map((r) => summary(u, r)),
          total: items.length,
          page,
          page_size: size,
          counts,
        });
      },
    ],
    [
      'POST',
      /^\/requests$/,
      (req) => {
        const u = me();
        const form = req.form;
        const raw = form?.get('payload');
        let p: Partial<CreateRequest>;
        try {
          p = JSON.parse(typeof raw === 'string' ? raw : '{}');
        } catch {
          throw bad({ payload: 'Неверный формат' });
        }
        const fields = validateCreate(p);
        const files = (form?.getAll('files') ?? []).filter((f): f is File => f instanceof File);
        if (files.length > LIMITS.max_files) fields.files = `Не больше ${LIMITS.max_files} файлов`;
        let total = 0;
        for (const f of files) {
          total += f.size;
          const ext = f.name.split('.').pop()?.toLowerCase() ?? '';
          if (!LIMITS.allowed_extensions.includes(ext)) fields.files = `Файл «${f.name}»: такой формат не поддерживается`;
          else if (f.size > LIMITS.max_file_mb * 1024 * 1024) fields.files = `Файл «${f.name}» больше ${LIMITS.max_file_mb} МБ`;
        }
        if (!fields.files && total > LIMITS.max_total_mb * 1024 * 1024) fields.files = `Общий размер файлов больше ${LIMITS.max_total_mb} МБ`;
        if (Object.keys(fields).length) throw bad(fields);

        const rule = route(p.category_id!, p.type!);
        const d = rule ? dept(rule.department_id) : undefined;
        const created = now();
        db.numberCounter += 1;
        const id = db.numberCounter;
        const r: MRequest = {
          id,
          number: `MV-${String(id).padStart(4, '0')}`,
          type: p.type!,
          category_id: p.category_id!,
          title: p.title!.trim(),
          body: p.body!.trim(),
          proposed_solution: p.proposed_solution?.trim() || null,
          author_id: u.id,
          is_anonymous: Boolean(p.is_anonymous),
          department_id: d?.id ?? null,
          assignee_id: rule ? (rule.assignee_user_id ?? d?.head_user_id ?? null) : null,
          status: 'new',
          routing_source: rule ? 'rule' : 'unrouted',
          first_response_due: iso(created + (rule?.first_response_hours ?? 24) * H),
          resolution_due: iso(created + (rule?.resolution_hours ?? 120) * H),
          first_responded_at: null,
          paused_at: null,
          closed_at: null,
          result_text: null,
          duplicate_of: null,
          theme_id: null,
          created_at: iso(created),
          updated_at: iso(created),
          attachments: files.map((f) => ({ id: nextId(), filename: f.name, mime: f.type || 'application/octet-stream', size: f.size })),
        };
        db.requests.push(r);
        db.events.push({ id: nextId(), request_id: id, actor_id: u.id, kind: 'created', created_at: iso(created) });
        db.events.push({
          id: nextId(),
          request_id: id,
          actor_id: null,
          kind: 'routed',
          text: d ? `Передано в отдел «${d.name}» по правилу маршрутизации` : 'Правило не найдено — передано администратору',
          created_at: iso(created + 1),
        });
        for (const o of owners(r)) notify(o, r, rule ? 'created' : 'unrouted', `Новое обращение ${r.number}`, r.title, u.id);
        return ok(detail(u, r), 201);
      },
    ],
    [
      'GET',
      /^\/requests\/export\.csv$/,
      (req) => {
        const u = staffOnly();
        const rows = db.requests.filter((r) => inStaffScope(u, r));
        const q = req.query;
        const filtered = rows.filter(
          (r) =>
            (!q.get('status') || r.status === q.get('status')) &&
            (!q.get('category_id') || r.category_id === Number(q.get('category_id'))) &&
            (!q.get('department_id') || r.department_id === Number(q.get('department_id'))),
        );
        const esc = (s: string) => `"${s.replace(/"/g, '""')}"`;
        const lines = [
          'number,type,category,title,status,author,department,created_at',
          ...filtered.map((r) => {
            const s = summary(u, r);
            return [s.number, s.type, s.category.name, s.title, s.status, s.author.name, s.department?.name ?? '', s.created_at].map(esc).join(',');
          }),
        ];
        return { status: 200, text: '﻿' + lines.join('\n'), contentType: 'text/csv; charset=utf-8', headers: { 'Content-Disposition': 'attachment; filename="requests.csv"' } };
      },
    ],
    ['GET', /^\/requests\/(\d+)$/, (_req, m) => {
      const u = me();
      return ok(detail(u, getVisible(u, Number(m[1]))));
    }],
    [
      'POST',
      /^\/requests\/(\d+)\/status$/,
      (req, m) => {
        const u = me();
        const r = getStaffScoped(u, Number(m[1]));
        const b = req.json() as { status?: Status; result_text?: string; duplicate_of_id?: number };
        const to = b.status as Status;
        const allowed = detail(u, r).allowed_statuses;
        if (!to || !allowed.includes(to) || to === 'needs_info') {
          throw new HttpError(409, 'conflict', `Нельзя перевести обращение из статуса «${STATUS_LABEL[r.status]}» в «${STATUS_LABEL[to] ?? to}»`);
        }
        const text = (b.result_text ?? '').trim();
        if (TERMINAL.includes(to)) {
          if (!text) throw bad({ result_text: 'Без решения закрыть нельзя — опишите результат' });
          if (text.length > 4000) throw bad({ result_text: 'Не больше 4000 символов' });
        }
        if (to === 'duplicate') {
          const orig = db.requests.find((x) => x.id === b.duplicate_of_id);
          if (!orig || orig.id === r.id) throw bad({ duplicate_of_id: 'Укажите существующее обращение' });
          r.duplicate_of = orig.id;
        }
        touchFirstResponse(u, r);
        if (TERMINAL.includes(to)) r.result_text = text;
        setStatus(u, r, to, TERMINAL.includes(to) ? text : null);
        notify(r.author_id, r, 'status_changed', `${r.number}: ${STATUS_LABEL[to]}`, TERMINAL.includes(to) ? text : `Статус обращения «${r.title}» изменён`, u.id);
        return ok(detail(u, r));
      },
    ],
    [
      'POST',
      /^\/requests\/(\d+)\/assign$/,
      (req, m) => {
        const u = me();
        const r = getStaffScoped(u, Number(m[1]));
        if (TERMINAL.includes(r.status)) throw new HttpError(409, 'conflict', 'Закрытое обращение нельзя переназначить');
        const b = req.json() as { department_id?: number; assignee_id?: number | null };
        const fields: Record<string, string> = {};
        if (b.department_id !== undefined && !dept(b.department_id)) fields.department_id = 'Выберите отдел';
        if (b.assignee_id) {
          const a = user(b.assignee_id);
          if (!a || !isStaff(a) || !a.active) fields.assignee_id = 'Выберите сотрудника';
        }
        if (Object.keys(fields).length) throw bad(fields);
        if (b.department_id !== undefined) r.department_id = b.department_id;
        if (b.assignee_id !== undefined) r.assignee_id = b.assignee_id;
        if (r.routing_source !== 'rule' || b.department_id !== undefined) r.routing_source = 'manual';
        r.updated_at = iso();
        touchFirstResponse(u, r);
        const d = dept(r.department_id);
        const a = user(r.assignee_id);
        db.events.push({
          id: nextId(),
          request_id: r.id,
          actor_id: u.id,
          kind: 'assigned',
          text: [d && `Отдел «${d.name}»`, a ? `ответственный: ${a.name}` : 'без ответственного'].filter(Boolean).join(', '),
          created_at: iso(),
        });
        notify(r.assignee_id, r, 'assigned', `Вам назначено ${r.number}`, r.title, u.id);
        return ok(detail(u, r));
      },
    ],
    [
      'POST',
      /^\/requests\/(\d+)\/comments$/,
      (req, m) => {
        const u = me();
        const r = getVisible(u, Number(m[1]));
        const b = req.json() as { kind?: CommentKind; body?: string };
        const body = (b.body ?? '').trim();
        if (!body) throw bad({ body: 'Напишите текст сообщения' });
        if (body.length > 4000) throw bad({ body: 'Не больше 4000 символов' });
        const kinds = detail(u, r).allowed_comment_kinds;
        if (!b.kind || !kinds.includes(b.kind)) throw new HttpError(409, 'conflict', 'Сейчас нельзя отправить такое сообщение');
        db.comments.push({ id: nextId(), request_id: r.id, author_id: u.id, kind: b.kind, body, created_at: iso() });
        r.updated_at = iso();
        if (b.kind !== 'internal') touchFirstResponse(u, r);
        if (b.kind === 'question') {
          setStatus(u, r, 'needs_info', null);
          notify(r.author_id, r, 'question', `Нужен ваш ответ по ${r.number}`, body.slice(0, 120), u.id);
        } else if (b.kind === 'answer') {
          setStatus(u, r, 'in_review', null);
          for (const o of owners(r)) notify(o, r, 'answer', `Автор ответил: ${r.number}`, body.slice(0, 120), u.id);
        } else if (b.kind === 'comment') {
          if (u.id === r.author_id) for (const o of owners(r)) notify(o, r, 'comment', `Новый комментарий в ${r.number}`, body.slice(0, 120), u.id);
          else notify(r.author_id, r, 'comment', `Новый комментарий в ${r.number}`, body.slice(0, 120), u.id);
        }
        return ok(detail(u, r), 201);
      },
    ],
    [
      'POST',
      /^\/requests\/(\d+)\/theme$/,
      (req, m) => {
        const u = me();
        const r = getStaffScoped(u, Number(m[1]));
        const b = req.json() as { theme_id?: number; title?: string };
        let theme = b.theme_id ? db.themes.find((t) => t.id === b.theme_id) : undefined;
        if (!theme) {
          const title = (b.title ?? '').trim();
          if (!title) throw bad({ title: 'Выберите тему или введите название новой' });
          theme = { id: nextId(), title, created_at: iso() };
          db.themes.push(theme);
        }
        r.theme_id = theme.id;
        r.updated_at = iso();
        db.events.push({ id: nextId(), request_id: r.id, actor_id: u.id, kind: 'theme_linked', text: theme.title, created_at: iso() });
        return ok(detail(u, r));
      },
    ],
    [
      'GET',
      /^\/attachments\/(\d+)$/,
      (_req, m) => {
        const u = me();
        const id = Number(m[1]);
        const r = db.requests.find((x) => x.attachments.some((a) => a.id === id));
        if (!r || !canSee(u, r)) throw notFound();
        const a = r.attachments.find((x) => x.id === id)!;
        return { status: 200, text: `Демо-вложение «${a.filename}» (${a.size} байт)`, contentType: 'text/plain; charset=utf-8', headers: { 'Content-Disposition': `attachment; filename="${encodeURIComponent(a.filename)}"` } };
      },
    ],
    [
      'GET',
      /^\/notifications$/,
      (req) => {
        const u = me();
        const mine = db.notifications.filter((n) => n.user_id === u.id).sort((a, b) => b.created_at.localeCompare(a.created_at));
        const unreadOnly = req.query.get('unread') === 'true';
        const limit = Number(req.query.get('limit') ?? 20);
        const items = (unreadOnly ? mine.filter((n) => !n.read) : mine).slice(0, limit).map((n) => ({
          id: n.id,
          kind: n.kind,
          request_id: n.request_id,
          request_number: db.requests.find((r) => r.id === n.request_id)?.number ?? '',
          title: n.title,
          body: n.body,
          read: n.read,
          created_at: n.created_at,
        }));
        return ok({ unread_count: mine.filter((n) => !n.read).length, items });
      },
    ],
    [
      'POST',
      /^\/notifications\/read$/,
      (req) => {
        const u = me();
        const b = req.json() as { ids?: number[]; all?: boolean };
        for (const n of db.notifications) if (n.user_id === u.id && (b.all || b.ids?.includes(n.id))) n.read = true;
        return noContent();
      },
    ],
    ['GET', /^\/analytics\/summary$/, (req) => ok(analytics(staffOnly(), req.query))],
    ['GET', /^\/analytics\/suggested-groups$/, () => ok({ items: suggestedGroups(staffOnly()) })],
    [
      'GET',
      /^\/themes$/,
      () => {
        const u = staffOnly();
        return ok({
          items: db.themes.map((t) => {
            const rs = db.requests.filter((r) => r.theme_id === t.id && inStaffScope(u, r));
            return {
              id: t.id,
              title: t.title,
              count: rs.length,
              open_count: rs.filter((r) => !TERMINAL.includes(r.status)).length,
              categories: [...new Set(rs.map((r) => cat(r.category_id)?.name ?? ''))],
            };
          }),
        });
      },
    ],
    ['GET', /^\/admin\/categories$/, () => (adminOnly(), ok({ items: [...db.categories].sort((a, b) => a.sort - b.sort) }))],
    [
      'POST',
      /^\/admin\/categories$/,
      (req) => {
        adminOnly();
        const b = req.json() as Record<string, unknown>;
        const name = String(b.name ?? '').trim();
        if (!name) throw bad({ name: 'Укажите название' });
        if (db.categories.some((c) => c.name.toLowerCase() === name.toLowerCase()))
          throw new HttpError(409, 'conflict', 'Категория с таким названием уже есть', { name: 'Категория с таким названием уже есть' });
        const c = { id: nextId(), name, description: String(b.description ?? ''), icon: String(b.icon ?? 'folder'), sort: Number(b.sort ?? 100), active: b.active !== false };
        db.categories.push(c);
        return ok(c, 201);
      },
    ],
    [
      'PATCH',
      /^\/admin\/categories\/(\d+)$/,
      (req, m) => {
        adminOnly();
        const c = db.categories.find((x) => x.id === Number(m[1]));
        if (!c) throw notFound();
        const b = req.json() as Record<string, unknown>;
        if (b.name !== undefined) {
          const name = String(b.name).trim();
          if (!name) throw bad({ name: 'Укажите название' });
          if (db.categories.some((x) => x.id !== c.id && x.name.toLowerCase() === name.toLowerCase()))
            throw new HttpError(409, 'conflict', 'Категория с таким названием уже есть', { name: 'Категория с таким названием уже есть' });
          c.name = name;
        }
        if (b.description !== undefined) c.description = String(b.description);
        if (b.icon !== undefined) c.icon = String(b.icon);
        if (b.sort !== undefined) c.sort = Number(b.sort);
        if (b.active !== undefined) c.active = Boolean(b.active);
        return ok(c);
      },
    ],
    ['GET', /^\/admin\/routing-rules$/, () => (adminOnly(), ok({ items: db.rules.map(ruleView) }))],
    [
      'POST',
      /^\/admin\/routing-rules$/,
      (req) => {
        adminOnly();
        const b = req.json() as Record<string, unknown>;
        const f = validateRule(b);
        if (Object.keys(f).length) throw bad(f);
        const rule = {
          id: nextId(),
          category_id: Number(b.category_id),
          request_type: (b.request_type as 'problem' | 'idea' | null) ?? null,
          department_id: Number(b.department_id),
          assignee_user_id: (b.assignee_user_id as number | null) ?? null,
          first_response_hours: Number(b.first_response_hours),
          resolution_hours: Number(b.resolution_hours),
          active: b.active !== false,
        };
        if (rule.active && db.rules.some((r) => r.active && r.category_id === rule.category_id && r.request_type === rule.request_type))
          throw new HttpError(409, 'conflict', 'Для этой категории и типа уже есть действующее правило');
        db.rules.push(rule);
        return ok(ruleView(rule), 201);
      },
    ],
    [
      'PATCH',
      /^\/admin\/routing-rules\/(\d+)$/,
      (req, m) => {
        adminOnly();
        const rule = db.rules.find((x) => x.id === Number(m[1]));
        if (!rule) throw notFound();
        const b = { ...rule, ...(req.json() as Record<string, unknown>) };
        const f = validateRule(b);
        if (Object.keys(f).length) throw bad(f);
        const next = {
          ...rule,
          category_id: Number(b.category_id),
          request_type: (b.request_type as 'problem' | 'idea' | null) ?? null,
          department_id: Number(b.department_id),
          assignee_user_id: (b.assignee_user_id as number | null) ?? null,
          first_response_hours: Number(b.first_response_hours),
          resolution_hours: Number(b.resolution_hours),
          active: Boolean(b.active),
        };
        if (next.active && db.rules.some((r) => r.id !== rule.id && r.active && r.category_id === next.category_id && r.request_type === next.request_type))
          throw new HttpError(409, 'conflict', 'Для этой категории и типа уже есть действующее правило');
        Object.assign(rule, next);
        return ok(ruleView(rule));
      },
    ],
    [
      'DELETE',
      /^\/admin\/routing-rules\/(\d+)$/,
      (_req, m) => {
        adminOnly();
        const i = db.rules.findIndex((x) => x.id === Number(m[1]));
        if (i < 0) throw notFound();
        db.rules.splice(i, 1);
        return noContent();
      },
    ],
    ['GET', /^\/admin\/users$/, () => (adminOnly(), ok({ items: db.users.map(adminUser) }))],
    [
      'POST',
      /^\/admin\/users$/,
      (req) => {
        adminOnly();
        const b = req.json() as Record<string, unknown>;
        const f: Record<string, string> = {};
        const name = String(b.name ?? '').trim();
        const email = String(b.email ?? '').trim().toLowerCase();
        if (!name) f.name = 'Укажите имя';
        if (!/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(email)) f.email = 'Неверный адрес почты';
        if (!['employee', 'manager', 'admin'].includes(String(b.role))) f.role = 'Выберите роль';
        if (String(b.password ?? '').length < 8) f.password = 'Минимум 8 символов';
        if (Object.keys(f).length) throw bad(f);
        if (db.users.some((u) => u.email.toLowerCase() === email))
          throw new HttpError(409, 'conflict', 'Пользователь с такой почтой уже есть', { email: 'Пользователь с такой почтой уже есть' });
        const u: MUser = {
          id: nextId(),
          name,
          email,
          password: String(b.password),
          role: b.role as MUser['role'],
          department_id: (b.department_id as number | null) ?? null,
          active: b.active !== false,
          title: '',
        };
        db.users.push(u);
        return ok(adminUser(u), 201);
      },
    ],
    [
      'PATCH',
      /^\/admin\/users\/(\d+)$/,
      (req, m) => {
        const admin = adminOnly();
        const u = db.users.find((x) => x.id === Number(m[1]));
        if (!u) throw notFound();
        const b = req.json() as Record<string, unknown>;
        const f: Record<string, string> = {};
        if (b.name !== undefined && !String(b.name).trim()) f.name = 'Укажите имя';
        if (b.email !== undefined && !/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(String(b.email).trim())) f.email = 'Неверный адрес почты';
        if (b.password !== undefined && b.password !== '' && String(b.password).length < 8) f.password = 'Минимум 8 символов';
        if (Object.keys(f).length) throw bad(f);
        if (b.email !== undefined && db.users.some((x) => x.id !== u.id && x.email.toLowerCase() === String(b.email).trim().toLowerCase()))
          throw new HttpError(409, 'conflict', 'Пользователь с такой почтой уже есть', { email: 'Пользователь с такой почтой уже есть' });
        if (u.id === admin.id && (b.active === false || (b.role !== undefined && b.role !== 'admin')))
          throw new HttpError(409, 'conflict', 'Нельзя отключить или понизить собственную учётную запись');
        if (b.name !== undefined) u.name = String(b.name).trim();
        if (b.email !== undefined) u.email = String(b.email).trim().toLowerCase();
        if (b.role !== undefined) u.role = b.role as MUser['role'];
        if (b.department_id !== undefined) u.department_id = (b.department_id as number | null) ?? null;
        if (b.active !== undefined) u.active = Boolean(b.active);
        if (b.password) u.password = String(b.password);
        return ok(adminUser(u));
      },
    ],
    [
      'GET',
      /^\/admin\/departments$/,
      () => (adminOnly(), ok({ items: db.departments.map((d) => ({ ...d, head_name: user(d.head_user_id)?.name ?? null })) })),
    ],
  ];

  return function handle(req: MockRequest): MockResponse {
    for (const [method, re, fn] of routes) {
      if (method !== req.method) continue;
      const m = req.path.match(re);
      if (m) return fn(req, m);
    }
    throw notFound();
  };
}

function inRange(isoStr: string, from: number, to: number) {
  const t = Date.parse(isoStr);
  return t >= from && t < to;
}
function startOfDay(t: number) {
  const d = new Date(t);
  d.setHours(0, 0, 0, 0);
  return d.getTime();
}
function localDate(t: number) {
  const d = new Date(t);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}
