// Contract checks for the in-browser mock: it must behave like the frozen API and the backend
// (CR1 dataset, CR2 anonymity by token, CR5 KPIs, CR6 observer rules), otherwise the UI is tested
// against the wrong thing.
import { describe, expect, it } from 'vitest';
import { createHandlers, linkCode, type MockRequest } from './handlers';
import { buildSeed, DEMO_PASSWORD, DEMO_TOKENS } from './seed';
import type { AnalyticsSummary, Meta, RequestDetail, RequestList, TelegramLinkCode, TelegramStatus } from '../types';

const NOW = Date.UTC(2026, 10, 10, 12);

function setup() {
  const db = buildSeed(NOW);
  const handle = createHandlers(db, () => NOW);
  const call = <T,>(method: string, path: string, body?: unknown, headers: Record<string, string> = {}): { status: number; body: T } => {
    const url = new URL(path, 'http://x');
    const req: MockRequest = {
      method,
      path: url.pathname,
      query: url.searchParams,
      json: () => body ?? {},
      form: null,
      header: (n) => headers[n] ?? null,
    };
    try {
      const r = handle(req);
      return { status: r.status, body: r.body as T };
    } catch (e) {
      const err = e as { status: number; code: string; message: string };
      return { status: err.status, body: { error: { code: err.code, message: err.message } } as T };
    }
  };
  const login = (login: string) => call('POST', '/auth/login', { email: `${login}@modus.demo`, password: DEMO_PASSWORD });
  const create = (payload: Record<string, unknown>) => {
    const form = new FormData();
    form.set('payload', JSON.stringify(payload));
    const req: MockRequest = { method: 'POST', path: '/requests', query: new URLSearchParams(), json: () => ({}), form };
    return handle(req) as { status: number; body: RequestDetail };
  };
  return { db, call, login, create };
}

describe('mock seed = dataset (CR1)', () => {
  it('has exactly the 20 dataset requests, 18 categories and role-named users', () => {
    const { db } = setup();
    expect(db.requests).toHaveLength(20);
    expect(db.requests.map((r) => r.number)).toEqual(Array.from({ length: 20 }, (_, i) => `MV-${String(i + 1).padStart(4, '0')}`));
    expect(db.categories).toHaveLength(18);
    expect(db.users.find((u) => u.email === 'purchasing.employee@modus.demo')?.name).toBe('Сотрудник закупок');
    expect(db.requests.filter((r) => r.type === 'feedback')).toHaveLength(1);
    expect(new Set(db.requests.map((r) => r.priority))).toEqual(new Set(['low', 'medium', 'high']));
    // A-005, A-017, A-018 are anonymous: no author link stored.
    expect(db.requests.filter((r) => r.is_anonymous).map((r) => r.number)).toEqual(['MV-0005', 'MV-0017', 'MV-0018']);
    expect(db.requests.filter((r) => r.is_anonymous).every((r) => r.author_id === null)).toBe(true);
  });

  it('demo users panel lists the six quick logins', () => {
    const { call } = setup();
    const r = call<{ users: { name: string }[] }>('GET', '/auth/demo-users').body;
    expect(r.users.map((u) => u.name)).toEqual([
      'Сотрудник закупок',
      'Менеджер по продажам',
      'Руководитель закупок',
      'IT-специалист',
      'Руководитель проектирования',
      'Администратор системы',
    ]);
  });
});

describe('mock API contract', () => {
  it('returns 401 with the error envelope without a session', () => {
    const { call } = setup();
    const r = call<{ error: { code: string } }>('GET', '/auth/me');
    expect(r.status).toBe(401);
    expect(r.body.error.code).toBe('unauthorized');
  });

  it('masks anonymous authors for everyone with a session (no author link exists)', () => {
    const { call, login } = setup();
    for (const who of ['accounting.head', 'admin']) {
      login(who);
      const d = call<RequestDetail>('GET', '/requests/5').body;
      expect(d.author).toEqual({ name: 'Автор (анонимно)', anonymous: true, is_me: false, role: null });
      expect(JSON.stringify(d)).not.toContain('Сотрудник бухгалтерии');
    }
    // The former author's session cannot see it at all.
    login('accounting.employee');
    expect(call('GET', '/requests/5').status).toBe(404);
    expect(call<RequestList>('GET', '/requests?scope=mine&page_size=100').body.items.some((r) => r.id === 5)).toBe(false);
  });

  it('CR2: creates an anonymous request, returns the token once and serves it only via /anonymous/*', () => {
    const { call, login, create } = setup();
    login('purchasing.employee');
    const res = create({ type: 'problem', category_id: 1, title: 'Анонимная проблема', body: 'Описание проблемы достаточно длинное для проверки.', is_anonymous: true, priority: 'high' });
    expect(res.status).toBe(201);
    const token = res.body.anonymous_token!;
    expect(token).toMatch(/^([0-9A-Z]{4}-){6}[0-9A-Z]{4}$/);
    expect(res.body.author).toMatchObject({ anonymous: true, is_me: true });
    expect(res.body.priority).toBe('high');
    // The creator's session has no link.
    expect(call('GET', `/requests/${res.body.id}`).status).toBe(404);
    // Lookup without a session, case/separator insensitive; wrong tokens get the same 404.
    call('POST', '/auth/logout');
    const look = call<RequestDetail>('POST', '/anonymous/lookup', { token: token.toLowerCase().replace(/-/g, ' ') });
    expect(look.status).toBe(200);
    expect(look.body.anonymous_token).toBeUndefined();
    expect(look.body.author.is_me).toBe(true);
    expect(call('POST', '/anonymous/lookup', { token: 'AAAA-BBBB' }).status).toBe(404);
    // Staff see an anonymous author only.
    login('purchasing.head');
    expect(call<RequestDetail>('GET', `/requests/${res.body.id}`).body.author.name).toBe('Автор (анонимно)');
  });

  it('CR2: the anonymous author answers a clarification by token; internal notes stay hidden', () => {
    const { call } = setup();
    const before = call<RequestDetail>('POST', '/anonymous/lookup', { token: DEMO_TOKENS.A005 }).body;
    expect(before.status).toBe('needs_info');
    expect(before.allowed_comment_kinds).toContain('answer');
    expect(before.timeline.some((t) => t.kind === 'internal')).toBe(false);
    const after = call<RequestDetail>('POST', '/anonymous/comments', { token: DEMO_TOKENS.A005, body: 'Счета и акты, этапы: бухгалтерия → юристы.' });
    expect(after.status).toBe(201);
    expect(after.body.status).toBe('in_review');
    expect(after.body.timeline.at(-1)?.actor).toMatchObject({ anonymous: true, is_me: true });
    // Closed requests: 409.
    expect(call('POST', '/anonymous/comments', { token: DEMO_TOKENS.A018, body: 'Спасибо' }).status).toBe(409);
  });

  it('CR2: anonymous attachments need the token header', () => {
    const { call, db } = setup();
    db.requests[4].attachments.push({ id: 9001, filename: 'scan.pdf', mime: 'application/pdf', size: 10 });
    expect(call('GET', '/anonymous/attachments/9001').status).toBe(404);
    expect(call('GET', '/anonymous/attachments/9001', undefined, { 'X-Anonymous-Token': DEMO_TOKENS.A005 }).status).toBe(200);
    expect(call('GET', '/anonymous/attachments/9001', undefined, { 'X-Anonymous-Token': DEMO_TOKENS.A017 }).status).toBe(404);
  });

  it('never returns internal notes to the author', () => {
    const { call, login } = setup();
    login('purchasing.employee');
    const d = call<RequestDetail>('GET', '/requests/1').body;
    expect(d.access_level).toBe('author');
    expect(d.timeline.some((t) => t.kind === 'internal')).toBe(false);
    login('purchasing.head');
    const staff = call<RequestDetail>('GET', '/requests/1').body;
    expect(staff.access_level).toBe('owner');
    expect(staff.timeline.some((t) => t.kind === 'internal')).toBe(true);
  });

  it('hides other employees requests (404) and the inbox from employees (403)', () => {
    const { call, login } = setup();
    login('purchasing.employee');
    expect(call('GET', '/requests/3').status).toBe(404);
    expect(call('GET', '/requests?scope=inbox').status).toBe(403);
    expect(call('GET', '/requests?scope=all').status).toBe(403);
  });

  it('CR6: a manager of another department is an observer (read, comment and internal note only)', () => {
    const { call, login, db } = setup();
    login('design.head');
    const all = call<RequestList>('GET', '/requests?scope=all&tab=all&page_size=100').body;
    expect(all.total).toBe(20);
    const mine = call<RequestList>('GET', '/requests?scope=all&tab=all&mine_scope=true&page_size=100').body;
    expect(mine.items.every((r) => r.access_level === 'owner')).toBe(true);
    expect(mine.items.map((r) => r.number)).toContain('MV-0014');
    const inbox = call<RequestList>('GET', '/requests?scope=inbox&tab=all&page_size=100').body;
    expect(inbox.items.map((r) => r.id).sort()).toEqual(mine.items.map((r) => r.id).sort());

    const d = call<RequestDetail>('GET', '/requests/1').body;
    expect(d.access_level).toBe('observer');
    expect(d.allowed_comment_kinds.sort()).toEqual(['comment', 'internal']);
    expect(d.allowed_statuses).toEqual([]);
    expect(d.can_assign).toBe(false);
    expect(call('POST', '/requests/1/status', { status: 'in_review' }).status).toBe(403);
    expect(call('POST', '/requests/1/assign', { department_id: 3 }).status).toBe(403);
    expect(call('POST', '/requests/1/theme', { title: 'Тема' }).status).toBe(403);
    expect(call('POST', '/requests/1/comments', { kind: 'question', body: '?' }).status).toBe(403);
    const c = call<RequestDetail>('POST', '/requests/1/comments', { kind: 'comment', body: 'Посмотрите шаблон из проектирования' });
    expect(c.status).toBe(201);
    // Observer comments are not a first response.
    expect(db.requests[0].first_responded_at).toBeNull();
    expect(db.notifications.some((n) => n.user_id === 2 && n.request_id === 1 && n.kind === 'comment' && n.body.startsWith('Посмотрите'))).toBe(true);
  });

  it('requires a result to close and validates transitions', () => {
    const { call, login } = setup();
    login('purchasing.head');
    expect(call('POST', '/requests/1/status', { status: 'declined' }).status).toBe(400);
    expect(call('POST', '/requests/1/status', { status: 'implemented', result_text: 'x' }).status).toBe(409);
    const ok = call<RequestDetail>('POST', '/requests/1/status', { status: 'declined', result_text: 'Шаблон уже содержит поля' });
    expect(ok.status).toBe(200);
    expect(ok.body.result_text).toBe('Шаблон уже содержит поля');
    expect(ok.body.sla.state).toBe('done');
  });

  it('returns tab counters for the inbox and filters by priority / type, sorts by priority', () => {
    const { call, login } = setup();
    login('admin');
    const list = call<RequestList>('GET', '/requests?scope=all&tab=needs_action').body;
    expect(list.counts!.all).toBe(20);
    expect(list.counts!.all).toBeGreaterThan(list.counts!.needs_action);
    const high = call<RequestList>('GET', '/requests?scope=all&tab=all&priority=high&page_size=100').body;
    expect(high.items.length).toBeGreaterThan(0);
    expect(high.items.every((r) => r.priority === 'high')).toBe(true);
    const fb = call<RequestList>('GET', '/requests?scope=all&tab=all&request_type=feedback&page_size=100').body;
    expect(fb.items.map((r) => r.number)).toEqual(['MV-0018']);
    const sorted = call<RequestList>('GET', '/requests?scope=all&tab=all&sort=priority&page_size=100').body.items.map((r) => r.priority);
    expect(sorted.indexOf('low')).toBeGreaterThan(sorted.lastIndexOf('high'));
  });

  it('walks the customer lifecycle and gates progress reports by status', () => {
    const { call, login } = setup();
    login('purchasing.head');
    const before = call<RequestDetail>('GET', '/requests/1').body;
    expect(before.status).toBe('new');
    expect(before.allowed_statuses).toEqual(expect.arrayContaining(['in_review', 'decision']));
    expect(before.allowed_comment_kinds).not.toContain('report');
    expect(call('POST', '/requests/1/comments', { kind: 'report', body: 'рано' }).status).toBe(409);
    expect(call<RequestDetail>('POST', '/requests/1/status', { status: 'in_review' }).body.status).toBe('in_review');
    const dec = call<RequestDetail>('POST', '/requests/1/status', { status: 'decision' }).body;
    expect(dec.allowed_statuses.sort()).toEqual(['closed', 'declined', 'in_progress', 'in_review']);
    const prog = call<RequestDetail>('POST', '/requests/1/status', { status: 'in_progress' }).body;
    expect(prog.allowed_comment_kinds).toContain('report');
    const rep = call<RequestDetail>('POST', '/requests/1/comments', { kind: 'report', body: 'Шаблон обновлён' });
    expect(rep.status).toBe(201);
    login('purchasing.employee');
    const d = call<RequestDetail>('GET', '/requests/1').body;
    expect(d.timeline.some((t) => t.kind === 'report')).toBe(true);
    expect(d.allowed_comment_kinds).not.toContain('report');
  });

  it('offers "closed" from every non-terminal status and requires a result', () => {
    const { call, login } = setup();
    login('admin');
    call('POST', '/requests/2/status', { status: 'decision' });
    const list = call<RequestList>('GET', '/requests?scope=all&tab=all&page_size=100').body.items;
    for (const s of ['new', 'in_review', 'needs_info', 'decision', 'in_progress'] as const) {
      const r = list.find((x) => x.status === s);
      expect(r, s).toBeTruthy();
      expect(call<RequestDetail>('GET', `/requests/${r!.id}`).body.allowed_statuses, s).toContain('closed');
    }
    expect(call('POST', '/requests/2/status', { status: 'closed' }).status).toBe(400);
    const done = call<RequestDetail>('POST', '/requests/2/status', { status: 'closed', result_text: 'Решено без доработок' });
    expect(done.body.status).toBe('closed');
    expect(done.body.allowed_statuses).toEqual([]);
  });

  it('CR5: analytics has on-time share, sample, medians and by_type, no p90', () => {
    const { call, login } = setup();
    login('admin');
    const a = call<AnalyticsSummary>('GET', '/analytics/summary').body;
    expect(a.kpis.total).toBe(20);
    expect(a.kpis.first_response_sample).toBe(15);
    expect(a.kpis.first_response_on_time_share).toBeGreaterThan(0);
    expect(a.kpis.first_response_on_time_share).toBeLessThan(1);
    expect(a.kpis.median_resolution_hours).toBeGreaterThan(24);
    expect('p90_first_response_hours' in a.kpis).toBe(false);
    expect(a.by_type).toEqual([
      { type: 'problem', count: 9, closed: 0 },
      { type: 'idea', count: 10, closed: 3 },
      { type: 'feedback', count: 1, closed: 1 },
    ]);
    // CR6: managers get company-wide numbers too.
    login('design.head');
    expect(call<AnalyticsSummary>('GET', '/analytics/summary').body.kpis.total).toBe(20);
  });

  it('CR4: telegram status, rotating link code and 409 once linked; meta feature flag', () => {
    const { call, login, db } = setup();
    login('purchasing.employee');
    expect(call<Meta>('GET', '/meta').body.features?.telegram).toBe(true);
    expect(call<TelegramStatus>('GET', '/telegram/status').body).toMatchObject({ enabled: true, linked: false });
    const c = call<TelegramLinkCode>('GET', '/telegram/link-code').body;
    expect(c.code).toMatch(/^[2-9A-HJKMNP-Z]{6}$/);
    expect(c.expires_in_seconds).toBeGreaterThan(0);
    expect(c.expires_in_seconds).toBeLessThanOrEqual(20);
    expect(linkCode(2, 1)).not.toBe(linkCode(2, 2));
    db.telegramLinks[2] = { linked_at: new Date(NOW).toISOString(), username: 'demo' };
    expect(call('GET', '/telegram/link-code').status).toBe(409);
    expect(call<TelegramStatus>('GET', '/telegram/status').body).toMatchObject({ linked: true, telegram_username: 'demo' });
    db.telegramEnabled = false;
    expect(call<Meta>('GET', '/meta').body.features?.telegram).toBe(false);
    expect(call('GET', '/telegram/link-code').status).toBe(404);
  });

  it('theme: title 2..140, reuse by title, 404 for an unknown theme; 429 when commenting too fast', () => {
    const { call, login } = setup();
    login('purchasing.head');
    expect(call('POST', '/requests/1/theme', { title: 'A' }).status).toBe(400);
    const t = call<RequestDetail>('POST', '/requests/1/theme', { title: 'Шаблоны заявок' }).body.theme!;
    expect(call<RequestDetail>('POST', '/requests/3/theme', { title: 'Шаблоны заявок' }).body.theme!.id).toBe(t.id);
    expect(call('POST', '/requests/1/theme', { theme_id: 9999 }).status).toBe(404);
    let last = 0;
    for (let i = 0; i < 32; i++) last = call('POST', '/requests/1/comments', { kind: 'internal', body: `n${i}` }).status;
    expect(last).toBe(429);
  });

  it('rejects a duplicate active routing rule with 409', () => {
    const { call, login } = setup();
    login('admin');
    const r = call('POST', '/admin/routing-rules', { category_id: 1, request_type: null, department_id: 1, assignee_user_id: null, first_response_hours: 8, resolution_hours: 72, active: true });
    expect(r.status).toBe(409);
  });
});
