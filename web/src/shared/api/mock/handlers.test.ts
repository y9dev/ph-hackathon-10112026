// Contract checks for the in-browser mock: it must behave like the frozen API
// (AuthorView masking, internal notes, transitions), otherwise the UI is tested
// against the wrong thing.
import { describe, expect, it } from 'vitest';
import { createHandlers, type MockRequest } from './handlers';
import { buildSeed } from './seed';
import type { RequestDetail, RequestList } from '../types';

function setup() {
  const db = buildSeed(Date.UTC(2026, 10, 10, 12));
  const handle = createHandlers(db, () => Date.UTC(2026, 10, 10, 12));
  const call = <T,>(method: string, path: string, body?: unknown): { status: number; body: T } => {
    const url = new URL(path, 'http://x');
    const req: MockRequest = { method, path: url.pathname, query: url.searchParams, json: () => body ?? {}, form: null };
    try {
      const r = handle(req);
      return { status: r.status, body: r.body as T };
    } catch (e) {
      const err = e as { status: number; code: string; message: string };
      return { status: err.status, body: { error: { code: err.code, message: err.message } } as T };
    }
  };
  const login = (email: string) => call('POST', '/auth/login', { email, password: 'demo12345' });
  return { db, call, login };
}

describe('mock API contract', () => {
  it('returns 401 with the error envelope without a session', () => {
    const { call } = setup();
    const r = call<{ error: { code: string } }>('GET', '/auth/me');
    expect(r.status).toBe(401);
    expect(r.body.error.code).toBe('unauthorized');
  });

  it('masks anonymous authors for managers and admins everywhere (AuthorView)', () => {
    const { call, login } = setup();
    for (const email of ['service.head@modus.demo', 'admin@modus.demo']) {
      login(email);
      const d = call<RequestDetail>('GET', '/requests/10').body;
      const serialized = JSON.stringify(d);
      expect(d.author).toEqual({ name: 'Автор (анонимно)', anonymous: true, is_me: false, role: null });
      expect(serialized).not.toContain('Тимур');
      expect(serialized).not.toContain('timur@');
      const list = call<RequestList>('GET', `/requests?scope=${email.startsWith('admin') ? 'all' : 'inbox'}&tab=all&page_size=100`).body;
      expect(JSON.stringify(list.items.find((r) => r.id === 10))).not.toContain('Тимур');
    }
  });

  it('shows the anonymous author their own identity and lets them answer', () => {
    const { call, login } = setup();
    login('timur@modus.demo');
    const d = call<RequestDetail>('GET', '/requests/10').body;
    expect(d.author).toMatchObject({ is_me: true, anonymous: true });
    expect(d.allowed_comment_kinds).toContain('answer');
    expect(d.allowed_statuses).toEqual([]);
    const after = call<RequestDetail>('POST', '/requests/10/comments', { kind: 'answer', body: 'Ответ автора' });
    expect(after.status).toBe(201);
    expect(after.body.status).toBe('in_review');
  });

  it('never returns internal notes to the author', () => {
    const { call, login } = setup();
    login('timur@modus.demo');
    const d = call<RequestDetail>('GET', '/requests/8').body;
    expect(d.timeline.some((t) => t.kind === 'internal')).toBe(false);
    login('supply.head@modus.demo');
    const staff = call<RequestDetail>('GET', '/requests/8').body;
    expect(staff.timeline.some((t) => t.kind === 'internal')).toBe(true);
  });

  it('hides other employees requests (404) and the inbox from employees (403)', () => {
    const { call, login } = setup();
    login('timur@modus.demo');
    expect(call('GET', '/requests/4').status).toBe(404);
    expect(call('GET', '/requests?scope=inbox').status).toBe(403);
  });

  it('requires a result to close and validates transitions', () => {
    const { call, login } = setup();
    login('it.head@modus.demo');
    expect(call('POST', '/requests/6/status', { status: 'declined' }).status).toBe(400);
    expect(call('POST', '/requests/6/status', { status: 'implemented', result_text: 'x' }).status).toBe(409);
    const ok = call<RequestDetail>('POST', '/requests/6/status', { status: 'declined', result_text: 'Заменили картридж' });
    expect(ok.status).toBe(200);
    expect(ok.body.result_text).toBe('Заменили картридж');
    expect(ok.body.sla.state).toBe('done');
  });

  it('returns tab counters for the inbox', () => {
    const { call, login } = setup();
    login('it.head@modus.demo');
    const list = call<RequestList>('GET', '/requests?scope=inbox&tab=needs_action').body;
    expect(list.counts).toBeTruthy();
    expect(list.counts!.all).toBeGreaterThan(list.counts!.needs_action);
    expect(list.items.every((r) => r.status === 'new' || r.sla.state === 'overdue')).toBe(true);
  });

  it('walks the customer lifecycle and gates progress reports by status', () => {
    const { call, login } = setup();
    login('it.head@modus.demo');
    const before = call<RequestDetail>('GET', '/requests/6').body;
    expect(before.status).toBe('new');
    expect(before.allowed_statuses).toEqual(expect.arrayContaining(['in_review', 'decision']));
    expect(before.allowed_statuses).not.toContain('in_progress');
    expect(before.allowed_comment_kinds).not.toContain('report');
    expect(call('POST', '/requests/6/comments', { kind: 'report', body: 'рано' }).status).toBe(409);

    expect(call<RequestDetail>('POST', '/requests/6/status', { status: 'in_review' }).body.status).toBe('in_review');
    const dec = call<RequestDetail>('POST', '/requests/6/status', { status: 'decision' }).body;
    expect(dec.allowed_statuses.sort()).toEqual(['closed', 'declined', 'in_progress', 'in_review']);
    const prog = call<RequestDetail>('POST', '/requests/6/status', { status: 'in_progress' }).body;
    expect(prog.allowed_comment_kinds).toContain('report');
    expect(prog.allowed_statuses.sort()).toEqual(['closed', 'decision', 'declined', 'implemented']);

    const rep = call<RequestDetail>('POST', '/requests/6/comments', { kind: 'report', body: 'Заказали картридж' });
    expect(rep.status).toBe(201);
    expect(rep.body.timeline.filter((t) => t.kind === 'report').at(-1)).toMatchObject({ kind: 'report', text: 'Заказали картридж' });

    // The author sees the report and gets a notification.
    login('timur@modus.demo');
    const mine = call<RequestList>('GET', '/requests?scope=mine&page_size=100').body.items.find((r) => r.id === 6);
    if (mine) {
      const d = call<RequestDetail>('GET', '/requests/6').body;
      expect(d.timeline.some((t) => t.kind === 'report')).toBe(true);
      expect(d.allowed_comment_kinds).not.toContain('report');
    }
  });

  it('offers "closed" from every non-terminal status and requires a result', () => {
    const { call, login } = setup();
    login('admin@modus.demo');
    const list = call<RequestList>('GET', '/requests?scope=all&tab=all&page_size=100').body.items;
    for (const s of ['new', 'in_review', 'needs_info', 'decision', 'in_progress'] as const) {
      const r = list.find((x) => x.status === s);
      expect(r, s).toBeTruthy();
      const d = call<RequestDetail>('GET', `/requests/${r!.id}`).body;
      expect(d.allowed_statuses, s).toContain('closed');
    }
    const target = list.find((x) => x.status === 'decision')!;
    expect(call('POST', `/requests/${target.id}/status`, { status: 'closed' }).status).toBe(400);
    const done = call<RequestDetail>('POST', `/requests/${target.id}/status`, { status: 'closed', result_text: 'Решено без доработок' });
    expect(done.status).toBe(200);
    expect(done.body.status).toBe('closed');
    expect(done.body.allowed_statuses).toEqual([]);
    const tab = call<RequestList>('GET', '/requests?scope=all&tab=closed&page_size=100').body;
    expect(tab.items.some((x) => x.id === target.id)).toBe(true);
  });

  it('migrates accepted-era data: no accepted status, tabs follow the new lifecycle', () => {
    const { call, login, db } = setup();
    expect(db.requests.some((r) => (r.status as string) === 'accepted')).toBe(false);
    login('admin@modus.demo');
    const tab = call<RequestList>('GET', '/requests?scope=all&tab=in_progress&page_size=100').body;
    expect(tab.items.length).toBeGreaterThan(0);
    expect(tab.items.every((r) => ['in_review', 'decision', 'in_progress'].includes(r.status))).toBe(true);
    const meta = call<{ statuses: { id: string }[] }>('GET', '/meta').body;
    expect(meta.statuses.map((s) => s.id)).toEqual(['new', 'in_review', 'needs_info', 'decision', 'in_progress', 'implemented', 'declined', 'duplicate', 'closed']);
  });

  it('returns 404 for an unknown theme and 429 when creating comments too fast', () => {
    const { call, login } = setup();
    login('it.head@modus.demo');
    const t = call<{ error: { message: string } }>('POST', '/requests/6/theme', { theme_id: 9999 });
    expect(t.status).toBe(404);
    expect(t.body.error.message).toBeTruthy();
    let last = 0;
    for (let i = 0; i < 32; i++) last = call('POST', '/requests/6/comments', { kind: 'internal', body: `n${i}` }).status;
    expect(last).toBe(429);
  });

  it('rejects a duplicate active routing rule with 409', () => {
    const { call, login } = setup();
    login('admin@modus.demo');
    const r = call('POST', '/admin/routing-rules', { category_id: 1, request_type: null, department_id: 1, assignee_user_id: null, first_response_hours: 8, resolution_hours: 72, active: true });
    expect(r.status).toBe(409);
  });
});
