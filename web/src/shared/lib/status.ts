import {
  CircleDot,
  Search,
  MessageCircleQuestion,
  ThumbsUp,
  CircleCheckBig,
  CircleX,
  Copy,
  type LucideIcon,
} from 'lucide-react';
import type { Status } from '@/shared/api/types';
import { ru } from '@/shared/i18n/ru';

export const STATUSES: Status[] = ['new', 'in_review', 'needs_info', 'accepted', 'implemented', 'declined', 'duplicate'];
export const TERMINAL: Status[] = ['implemented', 'declined', 'duplicate'];
/** Closing statuses require result text (duplicate also needs duplicate_of_id). */
export const CLOSING: Status[] = TERMINAL;

export const STATUS_ICON: Record<Status, LucideIcon> = {
  new: CircleDot,
  in_review: Search,
  needs_info: MessageCircleQuestion,
  accepted: ThumbsUp,
  implemented: CircleCheckBig,
  declined: CircleX,
  duplicate: Copy,
};

export function statusLabel(s: Status): string {
  return ru.statuses[s] ?? s;
}

export function isTerminal(s: Status): boolean {
  return TERMINAL.includes(s);
}

export function needsResult(s: Status): boolean {
  return CLOSING.includes(s);
}

/** Order of action buttons in the staff panel: progress first, closing last. */
const ACTION_ORDER: Status[] = ['in_review', 'needs_info', 'accepted', 'implemented', 'declined', 'duplicate', 'new'];
export function sortActions(list: Status[]): Status[] {
  return [...list].sort((a, b) => ACTION_ORDER.indexOf(a) - ACTION_ORDER.indexOf(b));
}

/** Parse "MV-0012", "mv 12", "12" into a request number string MV-0012, or null. */
export function parseRequestNumber(input: string): string | null {
  const m = input.trim().match(/^(?:mv[\s-]?)?0*(\d{1,6})$/i);
  if (!m) return null;
  return `MV-${m[1].padStart(4, '0')}`;
}
