// Mock of the backend heuristic (ARCHITECTURE D4): Jaccard over normalized tokens,
// lowercase, punctuation stripped, stop-words removed, 5-char prefix "stemming".
const STOP = new Set(
  'и в во на не что как это для по из за от до при с со у к о об же ли бы но а или то так уже ещё еще нет да мы вы я он она они их его её ее мне нам вам там тут где когда очень все всё весь каждый сейчас потом только этот эта эти того чтобы приходится'.split(
    ' ',
  ),
);

export function tokens(text: string): Set<string> {
  const out = new Set<string>();
  for (const raw of text.toLowerCase().replace(/ё/g, 'е').split(/[^a-zа-я0-9]+/i)) {
    if (raw.length < 3 || STOP.has(raw)) continue;
    out.add(raw.slice(0, 5));
  }
  return out;
}

export function jaccard(a: Set<string>, b: Set<string>): number {
  if (a.size === 0 || b.size === 0) return 0;
  let inter = 0;
  for (const t of a) if (b.has(t)) inter += 1;
  return inter / (a.size + b.size - inter);
}

/** Similarity of two requests; same category gets a small boost (capped at 1). */
export function similarity(
  a: { title: string; body: string; category_id: number },
  b: { title: string; body: string; category_id: number },
): number {
  const s = jaccard(tokens(`${a.title} ${a.body}`), tokens(`${b.title} ${b.body}`));
  return Math.min(1, a.category_id === b.category_id ? s * 1.25 : s);
}

export const SIMILAR_THRESHOLD = 0.14;
