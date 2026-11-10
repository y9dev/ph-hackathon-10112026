import { useMemo, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient, type UseQueryResult } from '@tanstack/react-query';
import { useState } from 'react';
import { Link, useSearchParams } from 'react-router';
import { BarChart3, Layers, AlarmClock, Tags } from 'lucide-react';
import { api } from '@/shared/api/client';
import { qk, useMeta } from '@/shared/api/queries';
import type { AnalyticsSummary, ItemsOf, Status, SuggestedGroup, Theme } from '@/shared/api/types';
import { cn } from '@/shared/lib/cn';
import { formatHours, formatPercent } from '@/shared/lib/format';
import { STATUS_ICON, STATUSES } from '@/shared/lib/status';
import { ru } from '@/shared/i18n/ru';
import {
  Badge,
  Button,
  Card,
  CardHeader,
  EmptyState,
  ErrorState,
  InfoTip,
  LoadingBlock,
  PageHeader,
  Select,
  SegmentedControl,
  Skeleton,
  SlaChip,
  StatusChip,
  useDocumentTitle,
  useToast,
} from '@/shared/ui';
import { useUser } from '@/features/auth/AuthProvider';
import { BarList, StackedBar, TrendChart } from './charts';
import { confidenceOf, type Confidence } from './confidence';

type Period = '7' | '30' | '90' | 'all';
const PERIODS: Period[] = ['7', '30', '90', 'all'];
const ALL = '__all';

const STATUS_COLOR: Record<Status, string> = {
  new: 'var(--status-new)',
  in_review: 'var(--status-in-review)',
  needs_info: 'var(--status-needs-info)',
  decision: 'var(--status-decision)',
  in_progress: 'var(--status-in-progress)',
  implemented: 'var(--status-implemented)',
  declined: 'var(--status-declined)',
  closed: 'color-mix(in srgb, var(--status-closed) 70%, var(--surface))',
  duplicate: 'color-mix(in srgb, var(--status-duplicate) 55%, var(--surface))',
};

function isoDay(d: Date) {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

export function periodRange(p: Period, now = new Date()): { from?: string; to?: string } {
  if (p === 'all') return {};
  const from = new Date(now);
  from.setDate(from.getDate() - Number(p) + 1);
  return { from: isoDay(from), to: isoDay(now) };
}

function dayLabel(iso: string) {
  const [y, m, d] = iso.split('-');
  return `${d}.${m}.${y}`;
}

/** "12.10.2026 – 10.11.2026" or "всё время". */
export function rangeText(range: { from?: string; to?: string }): string {
  return range.from && range.to ? `${dayLabel(range.from)} – ${dayLabel(range.to)}` : ru.analytics.rangeAll;
}

export default function AnalyticsPage() {
  useDocumentTitle(ru.analytics.title);
  const user = useUser();
  const meta = useMeta();
  const [sp, setSp] = useSearchParams();
  const period = (PERIODS.includes(sp.get('period') as Period) ? sp.get('period') : '30') as Period;
  const dept = sp.get('department') ?? ALL;
  const range = useMemo(() => periodRange(period), [period]);
  const params = { ...range, department_id: dept === ALL ? undefined : Number(dept) };

  const summary = useQuery({ queryKey: [...qk.analytics, params], queryFn: () => api.analytics(params) });
  const groups = useQuery({ queryKey: qk.groups, queryFn: api.suggestedGroups });
  const themes = useQuery({ queryKey: qk.themes, queryFn: api.themes });

  const departments = (meta.data?.departments ?? []).filter((d) => user.role === 'admin' || user.led_department_ids.includes(d.id));

  function setParam(k: string, v: string | null) {
    const next = new URLSearchParams(sp);
    if (v === null) next.delete(k);
    else next.set(k, v);
    setSp(next, { replace: true });
  }

  return (
    <div>
      <PageHeader
        title={ru.analytics.title}
        lead={
          <>
            {ru.analytics.lead}
            <span className="tabular mt-1 block text-sm font-medium text-text" data-testid="analytics-range">
              {ru.analytics.rangeLabel}: {rangeText(range)}
            </span>
          </>
        }
      />
      <div className="mb-6 flex flex-wrap items-end gap-4">
        <div className="flex flex-col gap-1">
          <span className="text-xs font-medium text-muted" id="period-label">
            {ru.analytics.period}
          </span>
          <SegmentedControl
            aria-labelledby="period-label"
            className="sm:w-[440px]"
            value={period}
            onChange={(v) => setParam('period', v === '30' ? null : v)}
            options={PERIODS.map((p) => ({ value: p, label: ru.analytics.periods[p] }))}
          />
        </div>
        {departments.length > 1 && (
          <div className="flex w-56 flex-col gap-1 max-sm:w-full">
            <label htmlFor="an-dept" className="text-xs font-medium text-muted">
              {ru.analytics.department}
            </label>
            <Select
              id="an-dept"
              size="sm"
              value={dept}
              onValueChange={(v) => setParam('department', v === ALL ? null : v)}
              options={[{ value: ALL, label: ru.analytics.allDepartments }, ...departments.map((d) => ({ value: String(d.id), label: d.name }))]}
            />
          </div>
        )}
      </div>

      {summary.isPending ? (
        <AnalyticsSkeleton />
      ) : summary.isError ? (
        <ErrorState error={summary.error} onRetry={() => summary.refetch()} />
      ) : summary.data.kpis.total === 0 && summary.data.overdue.length === 0 ? (
        <div className="rounded-xl border border-border bg-surface">
          <EmptyState icon={<BarChart3 className="h-6 w-6" />} title={ru.analytics.emptyTitle} text={ru.analytics.emptyText} />
        </div>
      ) : (
        <Dashboard data={summary.data} />
      )}

      <div className="mt-6 grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,1fr)_380px]">
        <GroupsCard q={groups} />
        <ThemesCard q={themes} />
      </div>
    </div>
  );
}

function Dashboard({ data }: { data: AnalyticsSummary }) {
  const meta = useMeta();
  const k = data.kpis;
  const catId = (name: string) => meta.data?.categories.find((c) => c.name === name)?.id;
  const deptId = (name: string) => meta.data?.departments.find((d) => d.name === name)?.id;

  return (
    <div className="flex flex-col gap-6 animate-fade-in">
      <section aria-labelledby="kpi-title" className="grid grid-cols-2 gap-3 md:grid-cols-3 md:gap-4 xl:grid-cols-4">
        <h2 id="kpi-title" className="col-span-full text-sm font-medium text-muted">
          {ru.analytics.kpiScope}
        </h2>
        <Kpi label={ru.analytics.kpiTotal} help={ru.analytics.kpiTotalHelp} value={String(k.total)} />
        <Kpi label={ru.analytics.kpiReviewed} help={ru.analytics.kpiReviewedHelp} value={formatPercent(k.reviewed_share)} bar={k.reviewed_share} />
        <Kpi
          label={ru.analytics.kpiFirstResponse}
          help={ru.analytics.kpiFirstResponseHelp}
          value={k.median_first_response_hours === null ? '—' : formatHours(k.median_first_response_hours)}
          sub={k.p90_first_response_hours === null ? ru.analytics.noData : ru.analytics.p90(formatHours(k.p90_first_response_hours))}
        />
        <Kpi label={ru.analytics.kpiOutcome} help={ru.analytics.kpiOutcomeHelp} value={formatPercent(k.outcome_share)} bar={k.outcome_share} />
        <Kpi label={ru.analytics.kpiImplemented} help={ru.analytics.kpiImplementedHelp} value={String(k.implemented_count)} />
        <Kpi
          label={ru.analytics.kpiOverdue}
          help={ru.analytics.kpiOverdueHelp}
          value={String(k.overdue_count)}
          sub={ru.analytics.kpiOverdueSub}
          tone={k.overdue_count > 0 ? 'danger' : undefined}
          href="/inbox?tab=all&overdue=1"
        />
        <Kpi label={ru.analytics.kpiAutoRouted} help={ru.analytics.kpiAutoRoutedHelp} value={formatPercent(k.auto_routed_share)} bar={k.auto_routed_share} />
      </section>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader title={ru.analytics.byStatus} description={ru.analytics.scopePeriod} />
          <div className="px-6 pb-6 pt-4 max-sm:px-4">
            <StackedBar
              data={STATUSES.map((s) => {
                const Icon = STATUS_ICON[s];
                return {
                  key: s,
                  label: ru.statuses[s],
                  value: data.by_status.find((b) => b.status === s)?.count ?? 0,
                  color: STATUS_COLOR[s],
                  href: `/inbox?tab=all&status=${s}`,
                  icon: <Icon className="h-3.5 w-3.5 shrink-0 text-muted" aria-hidden="true" />,
                };
              })}
            />
          </div>
        </Card>
        <Card>
          <CardHeader title={ru.analytics.trend} description={ru.analytics.scopePeriod} />
          <div className="px-6 pb-6 pt-4 max-sm:px-4">
            <TrendChart data={data.timeseries} />
          </div>
        </Card>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <Card>
          <CardHeader title={ru.analytics.byCategory} description={ru.analytics.scopePeriod} />
          <div className="px-6 pb-6 pt-4 max-sm:px-4">
            <BarList
              data={data.by_category.map((c) => {
                const id = catId(c.category);
                return {
                  key: c.category,
                  label: c.category,
                  value: c.count,
                  meta: `${ru.analytics.open.toLowerCase()} ${c.open} · ${ru.analytics.implemented.toLowerCase()} ${c.implemented}`,
                  href: id ? `/inbox?tab=all&category=${id}` : undefined,
                };
              })}
            />
          </div>
        </Card>
        <Card>
          <CardHeader title={ru.analytics.byDepartment} description={ru.analytics.scopePeriod} />
          <div className="px-6 pb-6 pt-4 max-sm:px-4">
            <BarList
              data={data.by_department.map((d) => {
                const id = deptId(d.department);
                return {
                  key: d.department,
                  label: d.department,
                  value: d.count,
                  color: 'var(--chart-2)',
                  meta: (
                    <>
                      {ru.analytics.open.toLowerCase()} {d.open}
                      {d.overdue > 0 && (
                        <span className="font-medium text-danger">
                          {' '}
                          · {ru.analytics.overdue.toLowerCase()} {d.overdue}
                        </span>
                      )}
                    </>
                  ),
                  href: id ? `/inbox?tab=all&department=${id}` : undefined,
                };
              })}
            />
          </div>
        </Card>
      </div>

      <Card>
        <CardHeader
          title={ru.analytics.overdueList}
          description={data.overdue.length > 8 ? `${ru.analytics.scopeNow}. ${ru.analytics.overdueMore(8, data.overdue.length)}` : ru.analytics.scopeNow}
          action={
            data.overdue.length > 0 && (
              <Link to="/inbox?tab=all&overdue=1" className="inline-flex min-h-10 items-center text-sm font-medium text-brand underline-offset-4 hover:underline max-md:min-h-11">
                {ru.analytics.openInbox}
              </Link>
            )
          }
        />
        <div className="px-6 pb-6 pt-4 max-sm:px-4">
          {data.overdue.length === 0 ? (
            <p className="text-sm text-muted">{ru.analytics.overdueEmpty}</p>
          ) : (
            <ul className="divide-y divide-border">
              {data.overdue.slice(0, 8).map((r) => (
                <li key={r.id}>
                  <Link to={`/requests/${r.id}`} className="-mx-2 flex flex-wrap items-center gap-x-4 gap-y-2 rounded-lg px-2 py-3 transition-colors duration-150 hover:bg-surface-2">
                    <span className="tabular w-20 shrink-0 text-xs font-medium text-muted">{r.number}</span>
                    <span className="min-w-0 flex-1 basis-48 truncate text-sm font-medium text-text">{r.title}</span>
                    <span className="text-xs text-muted max-md:hidden">{r.department?.name ?? ru.inbox.unrouted}</span>
                    <SlaChip sla={r.sla} status={r.status} />
                  </Link>
                </li>
              ))}
            </ul>
          )}
        </div>
      </Card>
    </div>
  );
}

function Kpi({
  label,
  help,
  value,
  sub,
  bar,
  tone,
  href,
}: {
  label: string;
  help: string;
  value: string;
  sub?: string;
  bar?: number;
  tone?: 'danger';
  href?: string;
}) {
  const body: ReactNode = (
    <>
      <p className={cn('tabular text-3xl font-semibold tracking-tight', tone === 'danger' ? 'text-danger' : 'text-text')}>
        {tone === 'danger' && <AlarmClock className="mr-1 inline h-6 w-6 align-[-3px]" aria-hidden="true" />}
        {value}
      </p>
      {sub && <p className="mt-1 text-xs text-muted">{sub}</p>}
      {bar !== undefined && (
        <span className="mt-3 block h-1.5 overflow-hidden rounded-full bg-chart-track" aria-hidden="true">
          <span className="block h-full rounded-full bg-brand" style={{ width: `${Math.round(bar * 100)}%` }} />
        </span>
      )}
    </>
  );
  return (
    <div className={cn('relative rounded-xl border bg-surface p-4 shadow-sm md:p-6', tone === 'danger' ? 'border-danger/40' : 'border-border')}>
      <h3 className="mb-2 pr-8 text-sm font-medium text-muted">{label}</h3>
      <span className="absolute right-2 top-2 z-[2] md:right-3 md:top-3">
        <InfoTip label={ru.analytics.aboutMetric(label)}>{help}</InfoTip>
      </span>
      {href ? (
        <Link to={href} className="block rounded-md after:absolute after:inset-0 after:rounded-xl after:content-['']" aria-label={`${label}: ${value}. ${ru.analytics.openInbox}`}>
          {body}
        </Link>
      ) : (
        body
      )}
    </div>
  );
}

const CONFIDENCE_TONE: Record<Confidence, 'success' | 'warning' | 'neutral'> = { high: 'success', medium: 'warning', low: 'neutral' };

function GroupsCard({ q }: { q: UseQueryResult<ItemsOf<SuggestedGroup>> }) {
  const qc = useQueryClient();
  const toast = useToast();
  const confirm = useMutation({
    // Create the theme on the first request, then link the remaining requests to it.
    // Only the requests the user kept ticked are linked.
    mutationFn: async ({ g, ids }: { g: SuggestedGroup; ids: number[] }) => {
      const picked = g.requests.filter((r) => ids.includes(r.id));
      const [first, ...rest] = picked;
      const created = await api.linkTheme(first.id, { title: g.title });
      const themeId = created.theme?.id;
      if (themeId) for (const r of rest) await api.linkTheme(r.id, { theme_id: themeId });
      return { ...g, requests: picked };
    },
    onSuccess: (g) => {
      toast.success(ru.analytics.groupConfirmed(g.title));
      qc.invalidateQueries({ queryKey: qk.groups });
      qc.invalidateQueries({ queryKey: qk.themes });
      qc.invalidateQueries({ queryKey: qk.requests });
      g.requests.forEach((r) => qc.invalidateQueries({ queryKey: qk.request(r.id) }));
    },
    onError: (e) => toast.error(e.message),
  });

  return (
    <Card>
      <CardHeader title={ru.analytics.groups} description={ru.analytics.groupsLead} />
      <div className="px-6 pb-6 pt-4 max-sm:px-4">
        {q.isPending ? (
          <LoadingBlock>
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="mb-3 h-24" />
            ))}
          </LoadingBlock>
        ) : q.isError ? (
          <ErrorState compact error={q.error} onRetry={() => q.refetch()} />
        ) : q.data.items.length === 0 ? (
          <EmptyState compact icon={<Layers className="h-6 w-6" />} title={ru.analytics.groupsEmpty} />
        ) : (
          <ul className="flex flex-col gap-3">
            {q.data.items.map((g) => (
              <GroupItem
                key={g.key}
                g={g}
                pending={confirm.isPending && confirm.variables?.g.key === g.key}
                otherPending={confirm.isPending && confirm.variables?.g.key !== g.key}
                onConfirm={(ids) => confirm.mutate({ g, ids })}
              />
            ))}
          </ul>
        )}
      </div>
    </Card>
  );
}

function GroupItem({
  g,
  pending,
  otherPending,
  onConfirm,
}: {
  g: SuggestedGroup;
  pending: boolean;
  otherPending: boolean;
  onConfirm: (ids: number[]) => void;
}) {
  const [off, setOff] = useState<number[]>([]);
  const picked = g.requests.filter((r) => !off.includes(r.id));
  const level = confidenceOf(g.score);
  const enough = picked.length >= 2;
  return (
    <li className="rounded-xl border border-border p-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0 flex-1">
          <p className="break-words font-semibold text-text">{g.title}</p>
          <p className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted">
            <Badge tone={CONFIDENCE_TONE[level]}>{ru.analytics.confidence(ru.analytics.confidenceLevels[level])}</Badge>
            <span>
              {ru.common.requests(g.count)} · {ru.analytics.groupScore(Math.round(g.score * 100))} · {g.categories.join(', ')}
            </span>
          </p>
        </div>
        <Button
          size="sm"
          className="max-sm:w-full"
          onClick={() => onConfirm(picked.map((r) => r.id))}
          loading={pending}
          disabled={otherPending || !enough}
          icon={<Tags className="h-4 w-4" aria-hidden="true" />}
        >
          {ru.analytics.groupConfirm}
        </Button>
      </div>
      <p className="mt-2 text-sm text-muted">{ru.analytics.groupReview}</p>
      <ul className="mt-3 flex flex-col gap-1">
        {g.requests.map((r) => {
          const on = !off.includes(r.id);
          return (
            <li key={r.id} className="flex items-center gap-1">
              <label className="flex h-10 w-10 shrink-0 cursor-pointer items-center justify-center max-md:h-11 max-md:w-11">
                <input
                  type="checkbox"
                  checked={on}
                  disabled={pending}
                  onChange={() => setOff((cur) => (on ? [...cur, r.id] : cur.filter((x) => x !== r.id)))}
                  aria-label={ru.analytics.groupInclude(r.number)}
                  className="h-4 w-4 accent-[var(--brand)]"
                />
              </label>
              <Link
                to={`/requests/${r.id}`}
                className={cn(
                  'flex min-w-0 flex-1 items-center gap-3 rounded-md px-2 py-1.5 text-sm transition-colors duration-150 hover:bg-surface-2 max-md:min-h-11',
                  !on && 'opacity-60',
                )}
              >
                <span className="tabular w-16 shrink-0 text-xs text-muted">{r.number}</span>
                <span className="min-w-0 flex-1 truncate text-text">{r.title}</span>
                <StatusChip status={r.status} className="max-sm:hidden" />
              </Link>
            </li>
          );
        })}
      </ul>
      <p className="mt-2 text-xs text-muted" aria-live="polite">
        {enough ? ru.analytics.groupSelected(picked.length, g.requests.length) : ru.analytics.groupMin}
      </p>
    </li>
  );
}

function ThemesCard({ q }: { q: UseQueryResult<ItemsOf<Theme>> }) {
  return (
    <Card>
      <CardHeader title={ru.analytics.themes} />
      <div className="px-6 pb-6 pt-4 max-sm:px-4">
        {q.isPending ? (
          <LoadingBlock>
            <Skeleton className="h-12" />
          </LoadingBlock>
        ) : q.isError ? (
          <ErrorState compact error={q.error} onRetry={() => q.refetch()} />
        ) : q.data.items.length === 0 ? (
          <p className="text-sm text-muted">{ru.analytics.themesEmpty}</p>
        ) : (
          <ul className="flex flex-col divide-y divide-border">
            {q.data.items.map((t) => (
              <li key={t.id} className="flex items-start justify-between gap-3 py-3 first:pt-0 last:pb-0">
                <div className="min-w-0">
                  <p className="min-w-0 text-sm font-medium text-text [overflow-wrap:anywhere]">{t.title}</p>
                  <p className="mt-0.5 text-xs text-muted">{t.categories.join(', ')}</p>
                </div>
                <div className="shrink-0 text-right">
                  <p className="tabular text-sm font-semibold text-text">{ru.common.requests(t.count)}</p>
                  <p className="text-xs text-muted">{ru.analytics.themeOpen(t.open_count)}</p>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
    </Card>
  );
}

function AnalyticsSkeleton() {
  return (
    <LoadingBlock>
      <div className="grid grid-cols-2 gap-4 md:grid-cols-3 xl:grid-cols-4">
        {[0, 1, 2, 3, 4, 5, 6].map((i) => (
          <Skeleton key={i} className="h-32" />
        ))}
      </div>
      <div className="mt-6 grid grid-cols-1 gap-6 lg:grid-cols-2">
        <Skeleton className="h-72" />
        <Skeleton className="h-72" />
      </div>
    </LoadingBlock>
  );
}
