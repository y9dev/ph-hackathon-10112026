import {
  FilePlus2,
  Route,
  UserCheck,
  RefreshCw,
  MessageSquare,
  MessageCircleQuestion,
  Reply,
  Lock,
  ClipboardList,
  Tags,
  AlarmClock,
  ArrowRight,
} from 'lucide-react';
import type { TimelineItem, TimelineKind } from '@/shared/api/types';
import { cn } from '@/shared/lib/cn';
import { formatDateTime } from '@/shared/lib/format';
import { ru } from '@/shared/i18n/ru';
import { Avatar, StatusChip } from '@/shared/ui';

const ICON: Record<TimelineKind, typeof FilePlus2> = {
  created: FilePlus2,
  routed: Route,
  assigned: UserCheck,
  status_changed: RefreshCw,
  comment: MessageSquare,
  question: MessageCircleQuestion,
  answer: Reply,
  internal: Lock,
  report: ClipboardList,
  theme_linked: Tags,
  overdue: AlarmClock,
};

const MESSAGE_KINDS: TimelineKind[] = ['comment', 'question', 'answer', 'internal', 'report'];

function ActorLine({ item }: { item: TimelineItem }) {
  const a = item.actor;
  return (
    <span className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
      <span className="text-sm font-medium text-text">
        {a ? (a.is_me ? `${a.name} (${ru.common.you.toLowerCase()})` : a.name) : ru.timeline.system}
      </span>
      {a?.role && <span className="text-xs text-muted">{a.role}</span>}
      <span className="text-xs text-muted">·</span>
      <time dateTime={item.at} className="tabular text-xs text-muted">
        {formatDateTime(item.at)}
      </time>
    </span>
  );
}

export function Timeline({ items }: { items: TimelineItem[] }) {
  if (items.length === 0) return <p className="text-sm text-muted">{ru.timeline.empty}</p>;
  return (
    <ol className="relative flex flex-col">
      {items.map((item, i) => {
        const Icon = ICON[item.kind] ?? RefreshCw;
        const isMessage = MESSAGE_KINDS.includes(item.kind);
        const internal = item.kind === 'internal';
        const last = i === items.length - 1;
        return (
          <li key={item.id} className="relative flex gap-4 pb-6 last:pb-0">
            {!last && <span className="absolute bottom-0 left-4 top-9 w-px bg-border" aria-hidden="true" />}
            <span
              className={cn(
                'relative z-[1] flex h-8 w-8 shrink-0 items-center justify-center rounded-full border',
                internal
                  ? 'border-warning/40 bg-warning-soft text-warning'
                  : item.kind === 'question'
                    ? 'border-[color-mix(in_srgb,var(--status-needs-info)_35%,transparent)] bg-[var(--status-needs-info-soft)] text-[var(--status-needs-info)]'
                    : item.kind === 'report'
                      ? 'border-[color-mix(in_srgb,var(--status-in-progress)_35%,transparent)] bg-[var(--status-in-progress-soft)] text-[var(--status-in-progress)]'
                      : isMessage
                      ? 'border-border bg-surface text-text'
                      : 'border-border bg-surface-2 text-muted',
              )}
              aria-hidden="true"
            >
              {isMessage && item.actor && !internal && item.kind === 'comment' ? (
                <Avatar name={item.actor.name} anonymous={item.actor.anonymous} size="sm" />
              ) : (
                <Icon className="h-4 w-4" />
              )}
            </span>
            <div className="min-w-0 flex-1 pt-1">
              {isMessage && <p className="sr-only">{internal ? `${ru.timeline.internal}, ${ru.timeline.internalBadge}` : ru.timeline[item.kind]}.</p>}
              <ActorLine item={item} />
              {isMessage ? (
                <div
                  className={cn(
                    'mt-2 rounded-xl border px-4 py-3',
                    internal
                      ? 'border-dashed border-warning/50 bg-warning-soft/60'
                      : item.kind === 'question'
                        ? 'border-[color-mix(in_srgb,var(--status-needs-info)_30%,transparent)] bg-surface'
                        : item.kind === 'report'
                          ? 'border-[color-mix(in_srgb,var(--status-in-progress)_40%,transparent)] bg-[var(--status-in-progress-soft)]'
                          : 'border-border bg-surface',
                  )}
                >
                  {item.kind !== 'comment' && (
                    <p
                      className={cn(
                        'mb-1 flex items-center gap-1 text-xs font-medium',
                        internal
                          ? 'text-warning'
                          : item.kind === 'question'
                            ? 'text-[var(--status-needs-info)]'
                            : item.kind === 'report'
                              ? 'text-[var(--status-in-progress)]'
                              : 'text-muted',
                      )}
                      aria-hidden="true"
                    >
                      <Icon className="h-3.5 w-3.5" />
                      {internal ? `${ru.timeline.internal} · ${ru.timeline.internalBadge}` : ru.timeline[item.kind]}
                    </p>
                  )}
                  <p className="whitespace-pre-line break-words text-sm leading-relaxed text-text">{item.text}</p>
                </div>
              ) : (
                <div className="mt-1 text-sm text-muted [overflow-wrap:anywhere]">
                  {item.kind === 'status_changed' && item.to_status ? (
                    <span className="flex flex-wrap items-center gap-2">
                      <span>{ru.timeline.status_changed}:</span>
                      {item.from_status && <StatusChip status={item.from_status} />}
                      {item.from_status && <ArrowRight className="h-3.5 w-3.5" aria-label={ru.timeline.to} />}
                      <StatusChip status={item.to_status} />
                    </span>
                  ) : item.kind === 'routed' && item.text ? (
                    // System text already says where it went ("Передано в отдел «IT» по правилу…").
                    <span className="text-text">{item.text}</span>
                  ) : (
                    <span>
                      {ru.timeline[item.kind]}
                      {item.text ? <span className="text-text">: {item.text}</span> : null}
                    </span>
                  )}
                  {item.kind === 'status_changed' && item.text && (
                    <p className="mt-2 whitespace-pre-line break-words rounded-lg bg-surface-2 px-3 py-2 text-sm text-text">{item.text}</p>
                  )}
                </div>
              )}
            </div>
          </li>
        );
      })}
    </ol>
  );
}
