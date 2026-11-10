import type { SubmitValues } from './schema';

const key = (userId: number) => `mv-draft-${userId}`;

/** Draft of the submit form (text fields only; files cannot be persisted). */
export function loadDraft(userId: number): Partial<SubmitValues> | null {
  try {
    const raw = localStorage.getItem(key(userId));
    if (!raw) return null;
    const d = JSON.parse(raw) as Partial<SubmitValues>;
    const hasContent = Boolean(d.title?.trim() || d.body?.trim() || d.proposed_solution?.trim());
    return hasContent ? d : null;
  } catch {
    return null;
  }
}

export function saveDraft(userId: number, v: Partial<SubmitValues>) {
  try {
    const hasContent = Boolean(v.title?.trim() || v.body?.trim() || v.proposed_solution?.trim());
    if (!hasContent) {
      localStorage.removeItem(key(userId));
      return;
    }
    localStorage.setItem(
      key(userId),
      JSON.stringify({
        type: v.type,
        priority: v.priority,
        category_id: v.category_id,
        title: v.title ?? '',
        body: v.body ?? '',
        proposed_solution: v.proposed_solution ?? '',
        is_anonymous: Boolean(v.is_anonymous),
      }),
    );
  } catch {
    // storage full or disabled: drafts are a convenience only
  }
}

/** Remove every stored draft (called on logout so anonymous text does not stay on a shared device). */
export function clearAllDrafts() {
  try {
    Object.keys(localStorage)
      .filter((k) => k.startsWith('mv-draft-'))
      .forEach((k) => localStorage.removeItem(k));
  } catch {
    // ignore
  }
}

export function clearDraft(userId: number) {
  try {
    localStorage.removeItem(key(userId));
  } catch {
    // ignore
  }
}
