import { useQuery } from '@tanstack/react-query';
import { useParams } from 'react-router';
import { CircleCheckBig, ArrowRight, Plus, EyeOff } from 'lucide-react';
import { api } from '@/shared/api/client';
import { qk } from '@/shared/api/queries';
import { formatDateTime } from '@/shared/lib/format';
import { ru } from '@/shared/i18n/ru';
import { ButtonLink, Card, ErrorState, LoadingBlock, Skeleton, useDocumentTitle } from '@/shared/ui';

export default function SuccessPage() {
  useDocumentTitle(ru.success.title);
  const id = Number(useParams().id);
  const q = useQuery({ queryKey: qk.request(id), queryFn: () => api.getRequest(id), enabled: Number.isInteger(id) && id > 0 });

  if (q.isPending)
    return (
      <LoadingBlock>
        <div className="mx-auto max-w-2xl">
          <Skeleton className="mx-auto h-14 w-14 rounded-full" />
          <Skeleton className="mx-auto mt-6 h-8 w-80 max-w-full" />
          <Skeleton className="mt-8 h-56" />
        </div>
      </LoadingBlock>
    );
  if (q.isError)
    return <ErrorState error={q.error} onRetry={() => q.refetch()} notFoundTitle={ru.errors.requestNotFoundTitle} notFoundText={ru.errors.requestNotFound} />;
  const r = q.data;

  return (
    <div className="mx-auto max-w-2xl animate-slide-up">
      <div className="flex flex-col items-center text-center">
        <span className="flex h-14 w-14 items-center justify-center rounded-full bg-success-soft text-success" aria-hidden="true">
          <CircleCheckBig className="h-7 w-7" />
        </span>
        <h1 className="mt-6 text-2xl font-semibold tracking-tight text-text md:text-3xl">{ru.success.heading}</h1>
      </div>

      <Card className="mt-8 overflow-hidden">
        <div className="border-b border-border bg-surface-2/60 px-6 py-6 text-center max-sm:px-4">
          <p className="text-sm text-muted">{ru.success.number}</p>
          <p className="tabular mt-1 text-3xl font-semibold tracking-tight text-text" data-testid="request-number">
            {r.number}
          </p>
          <p className="mt-2 text-sm text-muted">{r.title}</p>
        </div>
        <dl className="grid grid-cols-1 gap-4 px-6 py-6 sm:grid-cols-2 max-sm:px-4">
          <div>
            <dt className="text-xs font-medium text-muted">{ru.success.owner}</dt>
            <dd className="mt-1 text-sm font-medium text-text">
              {r.department ? `${r.department.name}${r.assignee ? ` · ${r.assignee.name}` : ''}` : ru.success.ownerUnrouted}
            </dd>
          </div>
          <div>
            <dt className="text-xs font-medium text-muted">{ru.success.deadline}</dt>
            <dd className="tabular mt-1 text-sm font-medium text-text">{formatDateTime(r.sla.first_response_due)}</dd>
          </div>
        </dl>
        {r.author.anonymous && (
          <p className="mx-6 mb-6 flex items-center gap-2 rounded-lg bg-surface-2 px-3 py-2 text-sm text-muted max-sm:mx-4">
            <EyeOff className="h-4 w-4 shrink-0" aria-hidden="true" />
            {ru.success.anonymous}
          </p>
        )}
      </Card>

      <section className="mt-8" aria-labelledby="next-steps">
        <h2 id="next-steps" className="text-base font-semibold text-text">
          {ru.success.nextTitle}
        </h2>
        <ol className="mt-4 flex flex-col gap-4">
          {[ru.success.step1, ru.success.step2, ru.success.step3].map((s, i) => (
            <li key={s} className="flex gap-3">
              <span className="tabular flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-brand-soft text-sm font-semibold text-brand" aria-hidden="true">
                {i + 1}
              </span>
              <p className="pt-0.5 text-sm text-text">{s}</p>
            </li>
          ))}
        </ol>
      </section>

      <div className="mt-8 flex flex-wrap gap-3 max-sm:flex-col">
        <ButtonLink to={`/requests/${r.id}`} variant="primary" size="lg" className="max-sm:w-full">
          {ru.success.open}
          <ArrowRight className="h-4 w-4" aria-hidden="true" />
        </ButtonLink>
        <ButtonLink to="/new" size="lg" icon={<Plus className="h-4 w-4" aria-hidden="true" />} className="max-sm:w-full">
          {ru.success.another}
        </ButtonLink>
      </div>
    </div>
  );
}
