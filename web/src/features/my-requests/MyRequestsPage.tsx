import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { useSearchParams } from 'react-router';
import { FilePlus2, Inbox, Plus } from 'lucide-react';
import { api } from '@/shared/api/client';
import { qk } from '@/shared/api/queries';
import type { RequestSummary } from '@/shared/api/types';
import { isTerminal } from '@/shared/lib/status';
import { cn } from '@/shared/lib/cn';
import { ru } from '@/shared/i18n/ru';
import { Button, ButtonLink, EmptyState, ErrorState, LoadingBlock, PageHeader, useDocumentTitle } from '@/shared/ui';
import { RequestCard, RequestCardSkeleton } from '@/shared/ui/RequestCard';

export type MineFilter = 'all' | 'open' | 'answer' | 'closed';
const FILTERS: MineFilter[] = ['all', 'open', 'answer', 'closed'];

export function matchMine(r: RequestSummary, f: MineFilter): boolean {
  if (f === 'open') return !isTerminal(r.status);
  if (f === 'answer') return r.status === 'needs_info';
  if (f === 'closed') return isTerminal(r.status);
  return true;
}

const LABEL: Record<MineFilter, string> = {
  all: ru.mine.all,
  open: ru.mine.open,
  answer: ru.mine.needsAnswer,
  closed: ru.mine.closed,
};

export default function MyRequestsPage() {
  useDocumentTitle(ru.mine.title);
  const [sp, setSp] = useSearchParams();
  const raw = sp.get('filter') as MineFilter | null;
  const filter: MineFilter = raw && FILTERS.includes(raw) ? raw : 'all';
  const q = sp.get('q') ?? undefined;

  const list = useQuery({
    queryKey: [...qk.requests, 'mine', { q }],
    queryFn: () => api.listRequests({ scope: 'mine', sort: 'updated', page_size: 100, q }),
  });

  const items = useMemo(() => {
    const all = list.data?.items ?? [];
    // Requests waiting for the author's answer go first.
    return [...all.filter((r) => matchMine(r, filter))].sort(
      (a, b) => Number(b.status === 'needs_info') - Number(a.status === 'needs_info'),
    );
  }, [list.data, filter]);
  const counts = useMemo(() => {
    const all = list.data?.items ?? [];
    return Object.fromEntries(FILTERS.map((f) => [f, all.filter((r) => matchMine(r, f)).length])) as Record<MineFilter, number>;
  }, [list.data]);

  function setFilter(f: MineFilter) {
    const next = new URLSearchParams(sp);
    if (f === 'all') next.delete('filter');
    else next.set('filter', f);
    setSp(next, { replace: true });
  }

  const total = list.data?.items.length ?? 0;

  return (
    <div className="mx-auto max-w-4xl">
      <PageHeader
        title={ru.mine.title}
        lead={q ? `${ru.inbox.search}: «${q}»` : ru.mine.lead}
        actions={
          <ButtonLink to="/new" variant="primary" icon={<Plus className="h-4 w-4" aria-hidden="true" />} className="max-md:hidden">
            {ru.nav.new}
          </ButtonLink>
        }
      />
      {list.isSuccess && total > 0 && (
        <div role="group" aria-label={ru.mine.filters} className="mb-4 flex flex-wrap gap-2">
          {FILTERS.map((f) => (
            <button
              key={f}
              type="button"
              aria-pressed={filter === f}
              onClick={() => setFilter(f)}
              className={cn(
                'inline-flex h-9 items-center gap-2 rounded-full border px-4 text-sm font-medium transition-colors duration-150 max-md:h-11',
                filter === f
                  ? 'border-brand bg-brand-soft text-brand'
                  : 'border-border bg-surface text-muted hover:border-border-strong hover:text-text',
                f === 'answer' && counts.answer > 0 && filter !== f && 'border-warning/50 text-warning',
              )}
            >
              {LABEL[f]}
              <span className="tabular text-xs opacity-80">{counts[f]}</span>
            </button>
          ))}
        </div>
      )}

      {list.isPending ? (
        <LoadingBlock>
          <div className="flex flex-col gap-3">
            {[0, 1, 2, 3].map((i) => (
              <RequestCardSkeleton key={i} />
            ))}
          </div>
        </LoadingBlock>
      ) : list.isError ? (
        <ErrorState error={list.error} onRetry={() => list.refetch()} />
      ) : total === 0 ? (
        <div className="rounded-xl border border-dashed border-border-strong bg-surface">
          <EmptyState
            icon={<FilePlus2 className="h-6 w-6" />}
            title={q ? ru.inbox.emptyFilteredTitle : ru.mine.emptyTitle}
            text={q ? ru.inbox.emptyFilteredText : ru.mine.emptyText}
            action={
              <ButtonLink to="/new" variant="primary" icon={<Plus className="h-4 w-4" aria-hidden="true" />}>
                {ru.mine.create}
              </ButtonLink>
            }
          />
        </div>
      ) : items.length === 0 ? (
        <div className="rounded-xl border border-border bg-surface">
          <EmptyState
            compact
            icon={<Inbox className="h-6 w-6" />}
            title={ru.mine.emptyFilteredTitle}
            text={ru.mine.emptyFilteredText}
            action={<Button onClick={() => setFilter('all')}>{ru.mine.all}</Button>}
          />
        </div>
      ) : (
        <ul className="flex flex-col gap-3" aria-label={ru.mine.title}>
          {items.map((r) => (
            <li key={r.id} className="animate-fade-in">
              <RequestCard r={r} variant="author" />
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
