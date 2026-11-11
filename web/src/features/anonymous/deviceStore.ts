// CR2: anonymous requests have no author link on the server. The only way back is the secret
// token, so the browser keeps {id, number, title, token, created_at} locally (this device only)
// to list them under «Анонимные обращения на этом устройстве». Storage may be unavailable
// (private mode, blocked site data): every access is wrapped and the UI works without it.

export interface DeviceAnonRequest {
  id: number;
  number: string;
  title: string;
  token: string;
  created_at: string;
}

const KEY = 'mv-anon-requests';
/** Token of the request opened in the anonymous view: per tab only, never in the URL. */
const CURRENT_KEY = 'mv-anon-current';
let memoryCurrent: string | null = null;

function isEntry(x: unknown): x is DeviceAnonRequest {
  const e = x as DeviceAnonRequest;
  return Boolean(e) && typeof e.id === 'number' && typeof e.number === 'string' && typeof e.title === 'string' && typeof e.token === 'string' && typeof e.created_at === 'string';
}

export function loadAnonymousRequests(): DeviceAnonRequest[] {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return [];
    const list = JSON.parse(raw) as unknown;
    return Array.isArray(list) ? list.filter(isEntry) : [];
  } catch {
    return [];
  }
}

function write(list: DeviceAnonRequest[]): boolean {
  try {
    localStorage.setItem(KEY, JSON.stringify(list));
    return true;
  } catch {
    return false;
  }
}

/** Adds (or refreshes) an entry; newest first. Returns false when storage is unavailable. */
export function saveAnonymousRequest(e: DeviceAnonRequest): boolean {
  const list = loadAnonymousRequests().filter((x) => x.token !== e.token);
  return write([e, ...list]);
}

export function removeAnonymousRequest(token: string): void {
  write(loadAnonymousRequests().filter((x) => x.token !== token));
}

/** Forgets every anonymous request saved on this device (used on explicit sign-out only). */
export function clearAnonymousRequests(): void {
  try {
    localStorage.removeItem(KEY);
  } catch {
    // storage unavailable: nothing to clear
  }
}

/** Canonical form for comparison/validation: upper case, separators removed, O→0, I/L→1 (as the server). */
export function canonicalToken(input: string): string {
  return input
    .toUpperCase()
    .replace(/[\s-]+/g, '')
    .replace(/O/g, '0')
    .replace(/[IL]/g, '1');
}

/** Groups a canonical token by 4 for display ("ABCD-EFGH-…"); the group count is not hard-coded. */
export function formatToken(input: string): string {
  return (canonicalToken(input).match(/.{1,4}/g) ?? []).join('-');
}

export const TOKEN_ALPHABET = /^[0-9ABCDEFGHJKMNPQRSTVWXYZ]+$/;

export function setCurrentToken(token: string) {
  memoryCurrent = token;
  try {
    sessionStorage.setItem(CURRENT_KEY, token);
  } catch {
    // memory only
  }
}

export function getCurrentToken(): string | null {
  if (memoryCurrent) return memoryCurrent;
  try {
    return sessionStorage.getItem(CURRENT_KEY);
  } catch {
    return null;
  }
}

export function clearCurrentToken() {
  memoryCurrent = null;
  try {
    sessionStorage.removeItem(CURRENT_KEY);
  } catch {
    // ignore
  }
}
