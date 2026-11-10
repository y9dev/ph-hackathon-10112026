import { Clock, AlarmClock, PauseCircle, CircleCheck, Lock, EyeOff } from 'lucide-react';
import type { ReactNode } from 'react';
import type { SlaState, Status } from '@/shared/api/types';
import { cn } from '@/shared/lib/cn';
import { slaView, type SlaTone } from '@/shared/lib/sla';
import { STATUS_ICON, statusLabel } from '@/shared/lib/status';
import { ru } from '@/shared/i18n/ru';

const chipBase =
  'inline-flex max-w-full items-center gap-1 whitespace-nowrap rounded-full border px-2 py-0.5 text-xs font-medium leading-5';

const STATUS_STYLE: Record<Status, string> = {
  new: 'text-[var(--status-new)] bg-[var(--status-new-soft)] border-[color-mix(in_srgb,var(--status-new)_25%,transparent)]',
  in_review:
    'text-[var(--status-in-review)] bg-[var(--status-in-review-soft)] border-[color-mix(in_srgb,var(--status-in-review)_25%,transparent)]',
  needs_info:
    'text-[var(--status-needs-info)] bg-[var(--status-needs-info-soft)] border-[color-mix(in_srgb,var(--status-needs-info)_25%,transparent)]',
  decision:
    'text-[var(--status-decision)] bg-[var(--status-decision-soft)] border-[color-mix(in_srgb,var(--status-decision)_25%,transparent)]',
  in_progress:
    'text-[var(--status-in-progress)] bg-[var(--status-in-progress-soft)] border-[color-mix(in_srgb,var(--status-in-progress)_25%,transparent)]',
  implemented:
    'text-[var(--status-implemented)] bg-[var(--status-implemented-soft)] border-[color-mix(in_srgb,var(--status-implemented)_25%,transparent)]',
  declined:
    'text-[var(--status-declined)] bg-[var(--status-declined-soft)] border-[color-mix(in_srgb,var(--status-declined)_25%,transparent)]',
  closed:
    'text-[var(--status-closed)] bg-[var(--status-closed-soft)] border-[color-mix(in_srgb,var(--status-closed)_25%,transparent)]',
  duplicate:
    'text-[var(--status-duplicate)] bg-[var(--status-duplicate-soft)] border-[color-mix(in_srgb,var(--status-duplicate)_25%,transparent)]',
};

/** Status = icon + text + colour (never colour only). */
export function StatusChip({ status, className }: { status: Status; className?: string }) {
  const Icon = STATUS_ICON[status];
  return (
    <span className={cn(chipBase, STATUS_STYLE[status], className)}>
      <Icon className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
      <span className="truncate">{statusLabel(status)}</span>
    </span>
  );
}

const TONE_STYLE: Record<SlaTone, string> = {
  neutral: 'text-muted bg-surface-2 border-border',
  warning: 'text-warning bg-warning-soft border-[color-mix(in_srgb,var(--warning)_30%,transparent)]',
  danger: 'text-danger bg-danger-soft border-[color-mix(in_srgb,var(--danger)_30%,transparent)]',
  info: 'text-info bg-info-soft border-[color-mix(in_srgb,var(--info)_25%,transparent)]',
  success: 'text-success bg-success-soft border-[color-mix(in_srgb,var(--success)_25%,transparent)]',
};
const TONE_ICON = { neutral: Clock, warning: AlarmClock, danger: AlarmClock, info: PauseCircle, success: CircleCheck };

export function SlaChip({ sla, status, className }: { sla: SlaState; status: Status; className?: string }) {
  const v = slaView(sla, status);
  const Icon = sla.state === 'done' ? CircleCheck : TONE_ICON[v.tone];
  return (
    <span className={cn(chipBase, TONE_STYLE[v.tone], 'tabular', className)} title={v.description}>
      <Icon className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
      <span className="truncate">{v.label}</span>
      {v.description !== v.label && <span className="sr-only">. {v.description}</span>}
    </span>
  );
}

export type BadgeTone = 'neutral' | 'brand' | 'info' | 'warning' | 'danger' | 'success';
const BADGE_STYLE: Record<BadgeTone, string> = {
  neutral: 'text-muted bg-surface-2 border-border',
  brand: 'text-brand bg-brand-soft border-[color-mix(in_srgb,var(--brand)_25%,transparent)]',
  info: TONE_STYLE.info,
  warning: TONE_STYLE.warning,
  danger: TONE_STYLE.danger,
  success: TONE_STYLE.success,
};

export function Badge({
  tone = 'neutral',
  icon,
  children,
  className,
}: {
  tone?: BadgeTone;
  icon?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <span className={cn(chipBase, BADGE_STYLE[tone], className)}>
      {icon}
      <span className="min-w-0 truncate">{children}</span>
    </span>
  );
}

export function AnonymousBadge() {
  return (
    <Badge icon={<EyeOff className="h-3.5 w-3.5" aria-hidden="true" />}>{ru.mine.anonymous}</Badge>
  );
}

export function InternalBadge() {
  return (
    <Badge tone="warning" icon={<Lock className="h-3.5 w-3.5" aria-hidden="true" />}>
      {ru.timeline.internalBadge}
    </Badge>
  );
}
