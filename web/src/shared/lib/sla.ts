import type { SlaState, Status } from '@/shared/api/types';
import { ru } from '@/shared/i18n/ru';
import { formatDateTime, formatHours } from './format';

export type SlaTone = 'neutral' | 'warning' | 'danger' | 'info' | 'success';

export interface SlaView {
  tone: SlaTone;
  label: string;
  /** Longer text for screen readers / tooltips. */
  description: string;
}

/** The deadline that currently matters: first response until someone responded, then resolution. */
export function activeDue(sla: SlaState): string {
  return sla.first_responded_at ? sla.resolution_due : sla.first_response_due;
}

export function slaView(sla: SlaState, status: Status, now: Date = new Date()): SlaView {
  const due = activeDue(sla);
  const dueText = formatDateTime(due, now);
  // In the decision / implementation stages the deadline is the resolution SLA of the whole request,
  // not a deadline of the stage itself: say so, so it is not read as "decide by".
  const resolutionStage = status === 'decision' || status === 'in_progress';
  const which = sla.first_responded_at ? ru.sla.resolution : ru.sla.firstResponse;
  switch (sla.state) {
    case 'overdue': {
      const kind = sla.overdue_kind === 'first_response' ? ru.sla.overdueFirst : ru.sla.overdueResolution;
      const label = ru.sla.overdue(formatHours(sla.overdue_hours));
      return { tone: 'danger', label, description: `${label}: ${kind}` };
    }
    case 'soon':
      return {
        tone: 'warning',
        label: resolutionStage ? ru.sla.resolutionSoon(dueText) : ru.sla.soon(dueText),
        description: `${which} ${ru.sla.until(dueText)}`,
      };
    case 'paused':
      return { tone: 'info', label: ru.sla.paused, description: `${ru.sla.paused}: срок на паузе` };
    case 'done':
      return {
        tone: status === 'implemented' ? 'success' : 'neutral',
        label: ru.sla.done,
        description: ru.sla.done,
      };
    case 'ok':
    default:
      return {
        tone: 'neutral',
        label: resolutionStage ? ru.sla.resolutionUntil(dueText) : ru.sla.until(dueText),
        description: `${which} ${ru.sla.until(dueText)}`,
      };
  }
}

/** Days the request has been waiting for the author (J4 reminder). */
export function waitingDays(updatedAt: string, now: Date = new Date()): number {
  const ms = now.getTime() - new Date(updatedAt).getTime();
  return Math.max(0, Math.floor(ms / 86_400_000));
}
