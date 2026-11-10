import { Link } from 'react-router';
import { MessageCircleQuestion, MessageSquare, Lightbulb, TriangleAlert, ChevronRight, ThumbsUp, Eye } from 'lucide-react';
import type { RequestSummary } from '@/shared/api/types';
import { cn } from '@/shared/lib/cn';
import { formatRelative } from '@/shared/lib/format';
import { isTerminal } from '@/shared/lib/status';
import { waitingDays } from '@/shared/lib/sla';
import { ru } from '@/shared/i18n/ru';
import { AnonymousBadge, Badge, PriorityChip, SlaChip, StatusChip } from './Chips';
import { StageStepperCompact } from './StageStepper';
import { Skeleton } from './States';

export function TypeLabel({ type, className }: { type: RequestSummary['type']; className?: string }) {
  const Icon = type === 'idea' ? Lightbulb : type === 'feedback' ? ThumbsUp : TriangleAlert;
  return (
    <span className={cn('inline-flex items-center gap-1 text-xs text-muted', className)}>
      <Icon className="h-3.5 w-3.5" aria-hidden="true" />
      {ru.types[type]}
    </span>
  );
}

/**
 * Card used for "My requests" and the mobile inbox. The whole card is one link;
 * the title is the accessible name, the rest is supporting text.
 */
export function RequestCard({ r, variant }: { r: RequestSummary; variant: 'author' | 'staff' }) {
  const needsAnswer = variant === 'author' && r.status === 'needs_info';
  const overdue = r.sla.state === 'overdue';
  return (
    <Link
      to={`/requests/${r.id}`}
      className={cn(
        'group relative block rounded-xl border bg-surface p-4 shadow-sm transition-[border-color,box-shadow,transform] duration-150 ease-out hover:-translate-y-px hover:border-border-strong hover:shadow-md md:p-6',
        needsAnswer ? 'border-warning/60' : 'border-border',
      )}
    >
      {variant === 'staff' && overdue && (
        <span className="absolute inset-y-4 left-0 w-1 rounded-r-full bg-danger" aria-hidden="true" />
      )}
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <span className="tabular text-xs font-medium text-muted">{r.number}</span>
        <TypeLabel type={r.type} />
        <span className="text-xs text-muted">{r.category.name}</span>
        {r.author.anonymous && <AnonymousBadge />}
      </div>
      <div className="mt-2 flex items-start justify-between gap-3">
        <h2 className="min-w-0 break-words text-base font-semibold text-text group-hover:text-brand">{r.title}</h2>
        <ChevronRight className="mt-0.5 h-5 w-5 shrink-0 text-muted transition-transform duration-150 group-hover:translate-x-0.5" aria-hidden="true" />
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-2">
        <StatusChip status={r.status} />
        {!isTerminal(r.status) && <SlaChip sla={r.sla} status={r.status} />}
        <PriorityChip priority={r.priority} />
        {variant === 'staff' && r.access_level === 'observer' && (
          <Badge icon={<Eye className="h-3.5 w-3.5" aria-hidden="true" />}>{ru.inbox.otherDept}</Badge>
        )}
        {r.comments_count > 0 && (
          <span className="inline-flex items-center gap-1 text-xs text-muted">
            <MessageSquare className="h-3.5 w-3.5" aria-hidden="true" />
            {ru.common.comments(r.comments_count)}
          </span>
        )}
      </div>
      <StageStepperCompact status={r.status} className="mt-3" />
      {needsAnswer && (
        <p className="mt-3 flex items-center gap-2 rounded-lg bg-warning-soft px-3 py-2 text-sm font-medium text-warning">
          <MessageCircleQuestion className="h-4 w-4 shrink-0" aria-hidden="true" />
          {ru.mine.needsAnswerBanner}
        </p>
      )}
      <div className="mt-3 flex flex-wrap items-center justify-between gap-x-4 gap-y-1 text-xs text-muted">
        <span className="min-w-0 truncate">
          {variant === 'author'
            ? r.department
              ? `${ru.mine.owner}: ${r.department.name}${r.assignee ? ` · ${r.assignee.name}` : ''}`
              : ru.mine.unrouted
            : `${ru.inbox.colAuthor}: ${r.author.name}${r.department ? ` · ${r.department.name}` : ` · ${ru.inbox.unrouted}`}`}
        </span>
        <span>
          {variant === 'staff' && r.awaiting_author
            ? ru.sla.waitingDays(waitingDays(r.updated_at))
            : `${ru.common.updated} ${formatRelative(r.updated_at)}`}
        </span>
      </div>
    </Link>
  );
}

export function RequestCardSkeleton() {
  return (
    <div className="rounded-xl border border-border bg-surface p-4 md:p-6" aria-hidden="true">
      <Skeleton className="h-3 w-40" />
      <Skeleton className="mt-3 h-5 w-3/4" />
      <div className="mt-4 flex gap-2">
        <Skeleton className="h-6 w-28 rounded-full" />
        <Skeleton className="h-6 w-32 rounded-full" />
      </div>
      <Skeleton className="mt-4 h-3 w-1/2" />
    </div>
  );
}
