import { useEffect, useId, useMemo, useState, type ReactNode } from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { Link, useNavigate, useSearchParams } from 'react-router';
import { Download, Filter, Inbox as InboxIcon, RotateCcw, Search, X, AlarmClock, EyeOff } from 'lucide-react';
import { api } from '@/shared/api/client';
import { qk, useMeta } from '@/shared/api/queries';
import type { InboxTab, ListSort, RequestSummary, Status } from '@/shared/api/types';
import { cn } from '@/shared/lib/cn';
import { downloadFile } from '@/shared/lib/download';
import {
  filtersFromSearch,
  filtersToParams,
  filtersToSearch,
  hasActiveFilters,
  patchFilters,
  type InboxFilters,
} from '@/shared/lib/inboxFilters';
import { STATUSES, isTerminal } from '@/shared/lib/status';
import { waitingDays } from '@/shared/lib/sla';
import { useDebounced } from '@/shared/lib/useDebounced';
import { ru } from '@/shared/i18n/ru';
import {
  Button,
  EmptyState,
  ErrorState,
  LoadingBlock,
  PageHeader,
  Pagination,
  Select,
  Skeleton,
  SlaChip,
  StatusChip,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  useDocumentTitle,
  useToast,
} from '@/shared/ui';
import { RequestCard, RequestCardSkeleton, TypeLabel } from '@/shared/ui/RequestCard';
import { useUser } from '@/features/auth/AuthProvider';

const TABS: InboxTab[] = ['needs_action', 'in_progress', 'waiting_author', 'closed', 'all'];
const ALL = '__all';

export default function InboxPage() {
  useDocumentTitle(ru.inbox.title);
  const user = useUser();
  const scope = user.role === 'admin' ? 'all' : 'inbox';
  const [sp, setSp] = useSearchParams();
  const filters = useMemo(() => filtersFromSearch(sp), [sp]);
  const meta = useMeta();
  const toast = useToast();
  const [exporting, setExporting] = useState(false);
  const [showFilters, setShowFilters] = useState(false);

  // Search box: local state, debounced into the URL.
  const [term, setTerm] = useState(filters.q ?? '');
  // Sync the box when the URL changes from outside (back/forward, top-bar search).
  const [urlQ, setUrlQ] = useState(filters.q ?? '');
  if ((filters.q ?? '') !== urlQ) {
    setUrlQ(filters.q ?? '');
    setTerm(filters.q ?? '');
  }
  const debounced = useDebounced(term.trim(), 350);
  const update = (patch: Partial<InboxFilters>) => setSp(filtersToSearch(patchFilters(filters, patch)), { replace: true });
  useEffect(() => {
    if ((filters.q ?? '') !== debounced) update({ q: debounced || undefined });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debounced]);

  const params = filtersToParams(filters, scope);
  const list = useQuery({
    queryKey: [...qk.requests, 'inbox', params],
    queryFn: () => api.listRequests(params),
    placeholderData: keepPreviousData,
  });
  const counts = list.data?.counts;
  const active = hasActiveFilters(filters);
  const activeCount = [filters.status, filters.category_id, filters.department_id, filters.assignee, filters.overdue].filter(Boolean).length;

  async function exportCsv() {
    setExporting(true);
    try {
      const { page: _p, page_size: _s, ...rest } = params;
      await downloadFile(api.exportCsvUrl(rest), `modus-voice-${new Date().toISOString().slice(0, 10)}.csv`);
    } catch (e) {
      toast.error(e instanceof Error ? e.message : ru.errors.generic);
    } finally {
      setExporting(false);
    }
  }

  const deptOptions = [{ value: ALL, label: ru.analytics.allDepartments }, ...(meta.data?.departments ?? []).map((d) => ({ value: String(d.id), label: d.name }))];
  const catOptions = [{ value: ALL, label: ru.common.all }, ...(meta.data?.categories ?? []).map((c) => ({ value: String(c.id), label: c.name }))];
  const statusOptions = [{ value: ALL, label: ru.common.all }, ...STATUSES.map((s) => ({ value: s, label: ru.statuses[s] }))];
  const sortOptions = (Object.keys(ru.inbox.sorts) as ListSort[]).map((s) => ({ value: s, label: ru.inbox.sorts[s] }));

  return (
    <div>
      <PageHeader
        title={ru.inbox.title}
        lead={user.role === 'admin' ? ru.inbox.leadAdmin : ru.inbox.lead}
        actions={
          <Button onClick={exportCsv} loading={exporting} icon={<Download className="h-4 w-4" aria-hidden="true" />}>
            {ru.inbox.export}
          </Button>
        }
      />

      <Tabs value={filters.tab} onValueChange={(v) => update({ tab: v as InboxTab })}>
        <TabsList aria-label={ru.inbox.tabs}>
          {TABS.map((t) => (
            <TabsTrigger key={t} value={t} count={counts?.[t]} countTone={t === 'needs_action' ? 'danger' : 'neutral'}>
              {ru.tabs[t]}
            </TabsTrigger>
          ))}
        </TabsList>

        {/* Filter bar */}
        <div className="mt-4 flex flex-col gap-3">
          <div className="flex flex-wrap items-center gap-2">
            <div className="relative min-w-0 flex-1 basis-64">
              <label htmlFor="inbox-search" className="sr-only">
                {ru.inbox.search}
              </label>
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" aria-hidden="true" />
              <input
                id="inbox-search"
                type="search"
                value={term}
                onChange={(e) => setTerm(e.target.value)}
                placeholder={ru.inbox.searchPlaceholder}
                className="h-10 w-full rounded-lg border border-border bg-surface pl-9 pr-9 text-sm text-text placeholder:text-muted transition-[border-color,box-shadow] duration-150 hover:border-border-strong focus-visible:border-focus focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-focus/30 max-md:h-11"
              />
              {term && (
                <button
                  type="button"
                  onClick={() => setTerm('')}
                  aria-label={ru.common.close}
                  className="absolute right-1 top-1/2 inline-flex h-8 w-8 -translate-y-1/2 items-center justify-center rounded-md text-muted hover:bg-surface-2 hover:text-text"
                >
                  <X className="h-4 w-4" aria-hidden="true" />
                </button>
              )}
            </div>
            <Button
              className="md:hidden"
              onClick={() => setShowFilters((s) => !s)}
              aria-expanded={showFilters}
              aria-controls="inbox-filters"
              icon={<Filter className="h-4 w-4" aria-hidden="true" />}
            >
              {ru.inbox.filters}
              {activeCount > 0 && <span className="tabular rounded-full bg-brand-soft px-1.5 text-xs text-brand">{activeCount}</span>}
            </Button>
            <div className="w-52 max-md:hidden">
              <Select size="sm" aria-label={ru.inbox.sort} value={filters.sort} onValueChange={(v) => update({ sort: v as ListSort })} options={sortOptions} />
            </div>
          </div>

          <div id="inbox-filters" className={cn('flex-wrap items-end gap-3 md:flex', showFilters ? 'flex' : 'hidden')} role="group" aria-label={ru.inbox.filters}>
            <FilterSelect label={ru.inbox.category} value={filters.category_id ? String(filters.category_id) : ALL} options={catOptions} onChange={(v) => update({ category_id: v === ALL ? undefined : Number(v) })} />
            <FilterSelect label={ru.inbox.department} value={filters.department_id ? String(filters.department_id) : ALL} options={deptOptions} onChange={(v) => update({ department_id: v === ALL ? undefined : Number(v) })} />
            <FilterSelect label={ru.inbox.status} value={filters.status ?? ALL} options={statusOptions} onChange={(v) => update({ status: v === ALL ? undefined : (v as Status) })} />
            <div className="w-full md:hidden">
              <FilterSelect label={ru.inbox.sort} value={filters.sort} options={sortOptions} onChange={(v) => update({ sort: v as ListSort })} />
            </div>
            <div className="flex flex-wrap gap-2">
              <ToggleChip pressed={Boolean(filters.overdue)} onClick={() => update({ overdue: filters.overdue ? undefined : true })}>
                <AlarmClock className="h-4 w-4" aria-hidden="true" />
                {ru.inbox.overdueOnly}
              </ToggleChip>
              <ToggleChip pressed={filters.assignee === 'me'} onClick={() => update({ assignee: filters.assignee === 'me' ? undefined : 'me' })}>
                {ru.inbox.mineOnly}
              </ToggleChip>
              <ToggleChip pressed={filters.assignee === 'none'} onClick={() => update({ assignee: filters.assignee === 'none' ? undefined : 'none' })}>
                {ru.inbox.unassigned}
              </ToggleChip>
            </div>
            {active && (
              <Button
                variant="ghost"
                size="sm"
                onClick={() => {
                  setTerm('');
                  setSp(filtersToSearch({ tab: filters.tab, sort: filters.sort, page: 1 }), { replace: true });
                }}
                icon={<RotateCcw className="h-4 w-4" aria-hidden="true" />}
              >
                {ru.inbox.reset}
              </Button>
            )}
          </div>
        </div>

        <TabsContent value={filters.tab} className="mt-4">
          {list.isPending ? (
            <ListSkeleton />
          ) : list.isError ? (
            <ErrorState error={list.error} onRetry={() => list.refetch()} />
          ) : list.data.items.length === 0 ? (
            <div className="rounded-xl border border-border bg-surface">
              <EmptyState
                icon={<InboxIcon className="h-6 w-6" />}
                title={active ? ru.inbox.emptyFilteredTitle : ru.inbox.emptyTitle}
                text={active ? ru.inbox.emptyFilteredText : ru.inbox.emptyText}
                action={
                  active && (
                    <Button
                      onClick={() => {
                        setTerm('');
                        setSp(filtersToSearch({ tab: filters.tab, sort: filters.sort, page: 1 }), { replace: true });
                      }}
                    >
                      {ru.inbox.reset}
                    </Button>
                  )
                }
              />
            </div>
          ) : (
            <div className={cn('transition-opacity duration-150', list.isPlaceholderData && 'opacity-60')} aria-busy={list.isFetching}>
              <RequestTable items={list.data.items} />
              <ul className="flex flex-col gap-3 md:hidden">
                {list.data.items.map((r) => (
                  <li key={r.id}>
                    <RequestCard r={r} variant="staff" />
                  </li>
                ))}
              </ul>
              <div className="mt-4">
                <Pagination page={list.data.page} pageSize={list.data.page_size} total={list.data.total} onPage={(p) => update({ page: p })} />
              </div>
            </div>
          )}
        </TabsContent>
      </Tabs>
    </div>
  );
}

function FilterSelect({ label, value, options, onChange }: { label: string; value: string; options: { value: string; label: string }[]; onChange: (v: string) => void }) {
  const id = `flt${useId().replace(/:/g, '')}`;
  return (
    <div className="flex min-w-0 flex-col gap-1 max-md:w-full md:w-48">
      <label htmlFor={id} className="text-xs font-medium text-muted">
        {label}
      </label>
      <Select id={id} size="sm" value={value} onValueChange={onChange} options={options} />
    </div>
  );
}

function ToggleChip({ pressed, onClick, children }: { pressed: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      aria-pressed={pressed}
      onClick={onClick}
      className={cn(
        'inline-flex h-9 items-center gap-2 rounded-full border px-3 text-sm font-medium transition-colors duration-150 max-md:h-11',
        pressed ? 'border-brand bg-brand-soft text-brand' : 'border-border bg-surface text-muted hover:border-border-strong hover:text-text',
      )}
    >
      {children}
    </button>
  );
}

function RequestTable({ items }: { items: RequestSummary[] }) {
  const navigate = useNavigate();
  return (
    <div className="overflow-hidden rounded-xl border border-border bg-surface shadow-sm max-md:hidden">
      <table className="w-full table-fixed border-collapse text-left">
        <caption className="sr-only">{ru.inbox.caption}</caption>
        <thead className="sticky top-16 z-[5] bg-surface-2/95 backdrop-blur">
          <tr className="border-b border-border text-xs font-medium text-muted">
            <th scope="col" className="w-28 px-4 py-3 font-medium">
              {ru.inbox.colNumber}
            </th>
            <th scope="col" className="px-4 py-3 font-medium">
              {ru.inbox.colTitle}
            </th>
            <th scope="col" className="w-44 px-4 py-3 font-medium">
              {ru.inbox.colStatus}
            </th>
            <th scope="col" className="w-48 px-4 py-3 font-medium max-xl:hidden">
              {ru.inbox.colDept}
            </th>
            <th scope="col" className="w-48 px-4 py-3 font-medium">
              {ru.inbox.colSla}
            </th>
          </tr>
        </thead>
        <tbody>
          {items.map((r) => {
            const overdue = r.sla.state === 'overdue';
            return (
              <tr
                key={r.id}
                onClick={(e) => {
                  if ((e.target as HTMLElement).closest('a')) return;
                  navigate(`/requests/${r.id}`);
                }}
                className={cn(
                  'group cursor-pointer border-b border-border transition-colors duration-150 last:border-0 hover:bg-surface-2/70',
                  overdue && 'bg-danger-soft/30',
                )}
              >
                <td className={cn('relative px-4 py-4 align-top', overdue && 'before:absolute before:inset-y-2 before:left-0 before:w-1 before:rounded-r-full before:bg-danger')}>
                  <span className="tabular block text-sm font-medium text-text">{r.number}</span>
                  <TypeLabel type={r.type} className="mt-1" />
                </td>
                <td className="px-4 py-4 align-top">
                  <Link to={`/requests/${r.id}`} className="line-clamp-2 text-sm font-semibold text-text underline-offset-4 group-hover:text-brand hover:underline">
                    {r.title}
                  </Link>
                  <span className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs text-muted">
                    <span>{r.category.name}</span>
                    <span aria-hidden="true">·</span>
                    <span className="inline-flex items-center gap-1">
                      {r.author.anonymous && <EyeOff className="h-3 w-3" aria-hidden="true" />}
                      {r.author.name}
                    </span>
                  </span>
                </td>
                <td className="px-4 py-4 align-top">
                  <StatusChip status={r.status} />
                  {r.awaiting_author && <span className="mt-1 block text-xs text-muted">{ru.sla.waitingDays(waitingDays(r.updated_at))}</span>}
                </td>
                <td className="px-4 py-4 align-top text-sm max-xl:hidden">
                  {r.department ? (
                    <span className="block truncate text-text">{r.department.name}</span>
                  ) : (
                    <span className="block font-medium text-warning">{ru.inbox.unrouted}</span>
                  )}
                  <span className="block truncate text-xs text-muted">{r.assignee?.name ?? ru.common.none}</span>
                </td>
                <td className="px-4 py-4 align-top">{isTerminal(r.status) ? <StatusDone r={r} /> : <SlaChip sla={r.sla} status={r.status} />}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function StatusDone({ r }: { r: RequestSummary }) {
  return <span className="tabular text-xs text-muted">{r.closed_at ? new Date(r.closed_at).toLocaleDateString('ru-RU') : '—'}</span>;
}

function ListSkeleton() {
  return (
    <LoadingBlock>
      <div className="overflow-hidden rounded-xl border border-border bg-surface max-md:hidden">
        <div className="h-10 border-b border-border bg-surface-2" />
        {[0, 1, 2, 3, 4, 5].map((i) => (
          <div key={i} className="flex gap-4 border-b border-border px-4 py-4 last:border-0">
            <Skeleton className="h-4 w-16" />
            <div className="flex-1">
              <Skeleton className="h-4 w-3/4" />
              <Skeleton className="mt-2 h-3 w-1/3" />
            </div>
            <Skeleton className="h-6 w-28 rounded-full" />
            <Skeleton className="h-6 w-32 rounded-full" />
          </div>
        ))}
      </div>
      <div className="flex flex-col gap-3 md:hidden">
        {[0, 1, 2].map((i) => (
          <RequestCardSkeleton key={i} />
        ))}
      </div>
    </LoadingBlock>
  );
}
