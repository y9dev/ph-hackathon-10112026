import type { RequestDetail } from '@/shared/api/types';

// The 201 response of an anonymous request (with its one-time token) lives only in memory for the
// success screen: not in the query cache, the URL, history state or sessionStorage.
const created = new Map<number, RequestDetail>();

export function rememberCreated(r: RequestDetail) {
  created.set(r.id, r);
}

export function getCreated(id: number): RequestDetail | undefined {
  return created.get(id);
}
