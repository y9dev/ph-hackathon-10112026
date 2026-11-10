import type { UseQueryResult } from '@tanstack/react-query';
import { Building2, Clock, ShieldCheck, Users, Inbox, EyeOff, CircleAlert } from 'lucide-react';
import type { ReactNode } from 'react';
import type { Category, RequestType, RoutingPreview } from '@/shared/api/types';
import { formatDateTime, formatHours } from '@/shared/lib/format';
import { ru } from '@/shared/i18n/ru';
import { Skeleton } from '@/shared/ui';

function Row({ icon, children, tone }: { icon: ReactNode; children: ReactNode; tone?: 'brand' | 'muted' | 'warning' }) {
  return (
    <li className="flex gap-3">
      <span
        className={
          tone === 'brand'
            ? 'mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-brand-soft text-brand'
            : tone === 'warning'
              ? 'mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-warning-soft text-warning'
              : 'mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-lg bg-surface-2 text-muted'
        }
        aria-hidden="true"
      >
        {icon}
      </span>
      <div className="min-w-0 text-sm text-text">{children}</div>
    </li>
  );
}

/** Live "Что произойдёт дальше" panel driven by /routing/preview. */
export function NextStepsPanel({
  category,
  type,
  anonymous,
  preview,
}: {
  category: Category | undefined;
  type: RequestType | undefined;
  anonymous: boolean;
  preview: UseQueryResult<RoutingPreview>;
}) {
  const p = preview.data;
  return (
    <section className="rounded-xl border border-border bg-surface p-6 shadow-sm max-sm:p-4" aria-live="polite" aria-labelledby="next-title">
      <h2 id="next-title" className="text-base font-semibold text-text">
        {ru.submit.nextTitle}
      </h2>
      {!category || !type ? (
        <p className="mt-3 text-sm text-muted">{ru.submit.nextPick}</p>
      ) : preview.isPending ? (
        <div className="mt-4 flex flex-col gap-3" aria-hidden="true">
          <Skeleton className="h-5 w-4/5" />
          <Skeleton className="h-5 w-3/5" />
          <Skeleton className="h-5 w-2/3" />
        </div>
      ) : preview.isError ? (
        <p className="mt-3 flex items-start gap-2 text-sm text-muted">
          <CircleAlert className="mt-0.5 h-4 w-4 shrink-0 text-warning" aria-hidden="true" />
          {ru.submit.previewError}
        </p>
      ) : p ? (
        <ul className="mt-4 flex flex-col gap-4">
          {p.routed && p.department ? (
            <Row icon={<Building2 className="h-4 w-4" />} tone="brand">
              <p className="font-medium">{ru.submit.nextRouted(p.department.name)}</p>
              {p.assignee_name && <p className="mt-0.5 text-muted">{ru.submit.nextAssignee(p.assignee_name)}</p>}
            </Row>
          ) : (
            <Row icon={<Inbox className="h-4 w-4" />} tone="warning">
              {ru.submit.nextUnrouted}
            </Row>
          )}
          <Row icon={<Clock className="h-4 w-4" />}>
            <p className="font-medium">{ru.submit.nextDeadline(formatDateTime(p.first_response_due))}</p>
            <p className="mt-0.5 text-muted">{ru.submit.nextResolution(formatHours(p.resolution_hours))}</p>
          </Row>
          <Row icon={anonymous ? <EyeOff className="h-4 w-4" /> : <ShieldCheck className="h-4 w-4" />}>
            <p>{anonymous ? ru.submit.nextPrivacyAnon : ru.submit.nextPrivacy}</p>
          </Row>
          {p.similar_count > 0 && (
            <Row icon={<Users className="h-4 w-4" />}>
              <p className="text-muted">{ru.submit.similar(p.similar_count)}</p>
            </Row>
          )}
        </ul>
      ) : null}
    </section>
  );
}
