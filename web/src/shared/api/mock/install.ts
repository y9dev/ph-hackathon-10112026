// Installs a fetch interceptor for /api/v1/* when VITE_MOCK=1.
// State lives in sessionStorage so a reload keeps your changes within the tab;
// a new tab (or Playwright context) starts from the seed.
import { createHandlers, HttpError, type MockRequest } from './handlers';
import type { MockDb } from './model';
import { buildSeed } from './seed';

const KEY = 'mv-mock-db';
const VERSION = 3;

function load(): MockDb {
  try {
    const raw = sessionStorage.getItem(KEY);
    if (raw) {
      const db = JSON.parse(raw) as MockDb;
      if (db.version === VERSION) return db;
    }
  } catch {
    // storage unavailable or corrupted: start from seed
  }
  return buildSeed();
}

function save(db: MockDb) {
  try {
    sessionStorage.setItem(KEY, JSON.stringify(db));
  } catch {
    // ignore quota / private-mode errors; the in-memory state still works
  }
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

export function installMock() {
  const db = load();
  const handle = createHandlers(db);
  const realFetch = window.fetch.bind(window);
  const latency = Number(new URLSearchParams(location.search).get('mockLatency') ?? 280);

  // Exposed for demos / debugging: window.__mvMockReset()
  (window as unknown as { __mvMockReset: () => void }).__mvMockReset = () => {
    sessionStorage.removeItem(KEY);
    location.reload();
  };

  window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url, location.origin);
    if (url.origin !== location.origin || !url.pathname.startsWith('/api/v1/')) return realFetch(input, init);

    const method = (init?.method ?? 'GET').toUpperCase();
    const body = init?.body;
    const req: MockRequest = {
      method,
      path: url.pathname.slice('/api/v1'.length),
      query: url.searchParams,
      json: () => (typeof body === 'string' && body ? JSON.parse(body) : {}),
      form: body instanceof FormData ? body : null,
    };
    await sleep(latency * (0.6 + Math.random() * 0.8));
    try {
      const res = handle(req);
      if (method !== 'GET') save(db);
      if (res.status === 204) return new Response(null, { status: 204 });
      if (res.text !== undefined)
        return new Response(res.text, { status: res.status, headers: { 'Content-Type': res.contentType ?? 'text/plain', ...res.headers } });
      return new Response(JSON.stringify(res.body), { status: res.status, headers: { 'Content-Type': 'application/json' } });
    } catch (e) {
      if (method !== 'GET') save(db);
      const err = e instanceof HttpError ? e : new HttpError(500, 'internal', 'Внутренняя ошибка');
      if (!(e instanceof HttpError)) console.error('[mock]', e);
      return new Response(JSON.stringify({ error: { code: err.code, message: err.message, fields: err.fields ?? null } }), {
        status: err.status,
        headers: { 'Content-Type': 'application/json' },
      });
    }
  };
}
