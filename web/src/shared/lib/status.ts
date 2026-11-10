import {
  CircleDot,
  Search,
  MessageCircleQuestion,
  Scale,
  Hammer,
  CircleCheckBig,
  CircleX,
  Copy,
  Archive,
  type LucideIcon,
} from 'lucide-react';
import type { Status } from '@/shared/api/types';
import { ru } from '@/shared/i18n/ru';

export const STATUSES: Status[] = ['new', 'in_review', 'needs_info', 'decision', 'in_progress', 'implemented', 'declined', 'duplicate', 'closed'];
export const TERMINAL: Status[] = ['implemented', 'declined', 'duplicate', 'closed'];
/** Closing statuses require result text (duplicate also needs duplicate_of_id). */
export const CLOSING: Status[] = TERMINAL;

export const STATUS_ICON: Record<Status, LucideIcon> = {
  new: CircleDot,
  in_review: Search,
  needs_info: MessageCircleQuestion,
  decision: Scale,
  in_progress: Hammer,
  implemented: CircleCheckBig,
  declined: CircleX,
  duplicate: Copy,
  closed: Archive,
};

export function statusLabel(s: Status): string {
  return ru.statuses[s] ?? s;
}

export type Stage = 'development' | 'decision' | 'implementation';
export const STAGES: Stage[] = ['development', 'decision', 'implementation'];

/** Lifecycle stage of a non-terminal status; null for terminal statuses. */
export function stageOf(s: Status): Stage | null {
  if (s === 'new' || s === 'in_review' || s === 'needs_info') return 'development';
  if (s === 'decision') return 'decision';
  if (s === 'in_progress') return 'implementation';
  return null;
}

/** Action label for moving the request from one status to another (a few transitions are "backwards"). */
export function actionLabel(from: Status, to: Status): string {
  if (from === 'decision' && to === 'in_review') return ru.lifecycle.backToRework;
  if (from === 'in_progress' && to === 'decision') return ru.lifecycle.backToDecision;
  return ru.statusActions[to];
}

export function isTerminal(s: Status): boolean {
  return TERMINAL.includes(s);
}

export function needsResult(s: Status): boolean {
  return CLOSING.includes(s);
}

/** Order of action buttons in the staff panel: progress first, closing last. */
const ACTION_ORDER: Status[] = ['in_review', 'decision', 'in_progress', 'implemented', 'needs_info', 'closed', 'declined', 'duplicate', 'new'];

/** Moving the request back to an earlier stage ("Вернуть на доработку", "Вернуть на принятие решения"). */
export function isBackward(from: Status, to: Status): boolean {
  return (from === 'decision' && to === 'in_review') || (from === 'in_progress' && to === 'decision');
}

/** Forward progress first, "return" actions after it, closing actions last. */
export function sortActions(list: Status[], from?: Status): Status[] {
  const pos = (s: Status) => (from && isBackward(from, s) ? ACTION_ORDER.indexOf('needs_info') - 0.5 : ACTION_ORDER.indexOf(s));
  return [...list].sort((a, b) => pos(a) - pos(b));
}

/** Parse "MV-0012", "mv 12", "12" into a request number string MV-0012, or null. */
export function parseRequestNumber(input: string): string | null {
  const m = input.trim().match(/^(?:mv[\s-]?)?0*(\d{1,6})$/i);
  if (!m) return null;
  return `MV-${m[1].padStart(4, '0')}`;
}
