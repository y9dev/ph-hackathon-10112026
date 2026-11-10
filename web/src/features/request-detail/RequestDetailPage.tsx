import { useRef, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link, useNavigate, useParams } from 'react-router';
import {
  ArrowLeft,
  Paperclip,
  Download,
  MessageCircleQuestion,
  CircleCheckBig,
  CircleX,
  Copy,
  Archive,
  UserCog,
  Tags,
  Search,
  Scale,
  Hammer,
  Undo2,
  EyeOff,
} from 'lucide-react';
import { api, ApiError } from '@/shared/api/client';
import { qk } from '@/shared/api/queries';
import type { RequestDetail, Status } from '@/shared/api/types';
import { cn } from '@/shared/lib/cn';
import { downloadFile } from '@/shared/lib/download';
import { formatBytes, formatDateTime } from '@/shared/lib/format';
import { actionLabel, isBackward, isTerminal, needsResult, sortActions, statusLabel } from '@/shared/lib/status';
import { ru } from '@/shared/i18n/ru';
import {
  AnonymousBadge,
  Badge,
  Button,
  Card,
  CardHeader,
  ErrorState,
  LoadingBlock,
  Meta,
  Skeleton,
  SlaChip,
  StatusChip,
  useDocumentTitle,
  useToast,
  type ButtonVariant,
} from '@/shared/ui';
import { TypeLabel } from '@/shared/ui/RequestCard';
import { useUser, isStaff } from '@/features/auth/AuthProvider';
import { Timeline } from './Timeline';
import { LifecycleCard } from './LifecycleCard';
import { Composer } from './Composer';
import { AssignDialog, CloseDialog, QuestionDialog, ThemeDialog } from './ActionDialogs';
import { useRequestMutations } from './useRequestMutations';

export default function RequestDetailPage() {
  const id = Number(useParams().id);
  const valid = Number.isInteger(id) && id > 0;
  const q = useQuery({ queryKey: qk.request(id), queryFn: () => api.getRequest(id), enabled: valid });
  useDocumentTitle(q.data ? `${q.data.number} · ${q.data.title}` : ru.detail.titleFallback);

  if (!valid)
    return <ErrorState error={new ApiError(404, 'not_found', '')} notFoundTitle={ru.errors.requestNotFoundTitle} notFoundText={ru.errors.requestNotFound} />;
  if (q.isPending) return <DetailSkeleton />;
  if (q.isError)
    return (
      <ErrorState
        error={q.error}
        onRetry={() => q.refetch()}
        notFoundTitle={ru.errors.requestNotFoundTitle}
        notFoundText={ru.errors.requestNotFound}
      />
    );
  return <Detail r={q.data} />;
}

function BackLink() {
  const navigate = useNavigate();
  const user = useUser();
  return (
    <button
      type="button"
      onClick={() => (window.history.length > 1 ? navigate(-1) : navigate(isStaff(user.role) ? '/inbox' : '/my'))}
      className="mb-4 inline-flex h-9 items-center gap-2 rounded-lg pr-2 text-sm font-medium text-muted transition-colors duration-150 hover:text-text max-md:h-11"
    >
      <ArrowLeft className="h-4 w-4" aria-hidden="true" />
      {ru.detail.back}
    </button>
  );
}

function Detail({ r }: { r: RequestDetail }) {
  const composerRef = useRef<HTMLTextAreaElement>(null);
  const isAuthor = r.author.is_me;
  const staffView = r.allowed_statuses.length > 0 || r.can_assign || r.allowed_comment_kinds.includes('internal');
  const needsAnswer = isAuthor && r.status === 'needs_info' && r.allowed_comment_kinds.includes('answer');
  const closed = isTerminal(r.status);

  return (
    <div>
      <BackLink />
      <header className="mb-6">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
          <span className="tabular text-sm font-medium text-muted">{r.number}</span>
          <TypeLabel type={r.type} className="text-sm" />
          <StatusChip status={r.status} />
          {!closed && <SlaChip sla={r.sla} status={r.status} />}
          {r.author.anonymous && <AnonymousBadge />}
          {r.theme && (
            <Badge tone="brand" icon={<Tags className="h-3.5 w-3.5" aria-hidden="true" />}>
              {r.theme.title}
            </Badge>
          )}
        </div>
        <h1 className="mt-3 text-2xl font-semibold tracking-tight text-text md:text-3xl">{r.title}</h1>
      </header>

      {needsAnswer && (
        <div role="status" className="mb-6 flex flex-wrap items-start justify-between gap-4 rounded-xl border border-warning/50 bg-warning-soft px-4 py-4 md:px-6">
          <div className="flex gap-3">
            <MessageCircleQuestion className="mt-0.5 h-5 w-5 shrink-0 text-warning" aria-hidden="true" />
            <div>
              <p className="font-semibold text-text">{ru.detail.needsAnswerTitle}</p>
              <p className="mt-1 text-sm text-text">{ru.detail.needsAnswerText}</p>
            </div>
          </div>
          <Button
            variant="primary"
            onClick={() => {
              composerRef.current?.scrollIntoView({ behavior: 'smooth', block: 'center' });
              composerRef.current?.focus({ preventScroll: true });
            }}
          >
            {ru.detail.needsAnswerAction}
          </Button>
        </div>
      )}

      <div className="mb-6">
        <LifecycleCard r={r} showHint={isAuthor && !staffView} />
      </div>

      <div className="grid grid-cols-1 items-start gap-6 lg:grid-cols-[minmax(0,1fr)_340px] lg:gap-8">
        <div className="flex min-w-0 flex-col gap-6">
          {r.result_text && <ResultCard r={r} highlight={isAuthor} />}

          <Card>
            <CardHeader title={ru.detail.description} />
            <div className="px-6 pb-6 pt-3 max-sm:px-4 max-sm:pb-4">
              <p className="whitespace-pre-line break-words text-base leading-relaxed text-text">{r.body}</p>
              {r.proposed_solution && (
                <div className="mt-6 border-t border-border pt-4">
                  <h3 className="text-sm font-semibold text-text">{ru.detail.solution}</h3>
                  <p className="mt-2 whitespace-pre-line break-words text-base leading-relaxed text-text">{r.proposed_solution}</p>
                </div>
              )}
              {r.attachments.length > 0 && <Attachments r={r} />}
            </div>
          </Card>

          <section aria-labelledby="timeline-title">
            <h2 id="timeline-title" className="mb-4 text-base font-semibold text-text">
              {ru.timeline.title}
            </h2>
            <Timeline items={r.timeline} />
          </section>

          <Composer ref={composerRef} request={r} />
        </div>

        <aside className="flex flex-col gap-6 lg:sticky lg:top-24" aria-label={ru.detail.metaPanel}>
          {staffView && <StaffActions r={r} />}
          <MetaCard r={r} />
          {staffView && <SimilarCard r={r} />}
          {!staffView && r.similar.count > 0 && (
            <p className="px-1 text-sm text-muted">{ru.submit.similar(r.similar.count)}</p>
          )}
        </aside>
      </div>
    </div>
  );
}

function ResultCard({ r, highlight }: { r: RequestDetail; highlight: boolean }) {
  const Icon = r.status === 'implemented' ? CircleCheckBig : r.status === 'closed' ? Archive : r.status === 'duplicate' ? Copy : r.status === 'declined' ? CircleX : CircleCheckBig;
  const tone =
    r.status === 'implemented'
      ? 'border-success/40 bg-success-soft'
      : r.status === 'declined' || r.status === 'duplicate' || r.status === 'closed'
        ? 'border-border-strong bg-surface-2'
        : 'border-brand/40 bg-brand-soft';
  return (
    <section
      aria-labelledby="result-title"
      className={cn('rounded-xl border p-6 max-sm:p-4', tone, highlight && 'shadow-md ring-1 ring-inset ring-border animate-slide-up')}
    >
      <div className="flex items-center gap-2">
        <Icon className="h-5 w-5 text-text" aria-hidden="true" />
        <h2 id="result-title" className="text-base font-semibold text-text">
          {ru.detail.resultTitle}: {statusLabel(r.status)}
        </h2>
      </div>
      <p className="mt-3 whitespace-pre-line break-words text-base leading-relaxed text-text">{r.result_text}</p>
      {r.duplicate_of && (
        <p className="mt-3 text-sm text-text">
          {ru.detail.duplicateOf}{' '}
          <Link to={`/requests/${r.duplicate_of.id}`} className="font-medium text-brand underline-offset-4 hover:underline">
            {r.duplicate_of.number}
          </Link>
        </p>
      )}
      {r.closed_at && <p className="tabular mt-3 text-xs text-muted">{formatDateTime(r.closed_at)}</p>}
    </section>
  );
}

function Attachments({ r }: { r: RequestDetail }) {
  const toast = useToast();
  const [busy, setBusy] = useState<number | null>(null);
  return (
    <div className="mt-6 border-t border-border pt-4">
      <h3 className="text-sm font-semibold text-text">{ru.detail.attachments}</h3>
      <ul className="mt-3 flex flex-col gap-2">
        {r.attachments.map((a) => (
          <li key={a.id}>
            <button
              type="button"
              disabled={busy === a.id}
              onClick={async () => {
                setBusy(a.id);
                try {
                  await downloadFile(api.attachmentUrl(a.id), a.filename);
                } catch (e) {
                  toast.error(e instanceof Error ? e.message : ru.errors.generic);
                } finally {
                  setBusy(null);
                }
              }}
              aria-label={ru.detail.download(a.filename)}
              className="group flex w-full items-center gap-3 rounded-lg border border-border px-3 py-2 text-left transition-colors duration-150 hover:border-border-strong hover:bg-surface-2 max-md:min-h-11"
            >
              <Paperclip className="h-4 w-4 shrink-0 text-muted" aria-hidden="true" />
              <span className="min-w-0 flex-1 truncate text-sm text-text">{a.filename}</span>
              <span className="tabular text-xs text-muted">{formatBytes(a.size)}</span>
              <Download className="h-4 w-4 shrink-0 text-muted group-hover:text-text" aria-hidden="true" />
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}

function MetaCard({ r }: { r: RequestDetail }) {
  const routing =
    r.routing_source === 'rule' ? ru.detail.routingRule : r.routing_source === 'manual' ? ru.detail.routingManual : ru.detail.routingUnrouted;
  return (
    <Card>
      <CardHeader title={ru.detail.details} as="h2" />
      <dl className="grid grid-cols-2 gap-4 px-6 pb-6 pt-4 max-sm:px-4 lg:grid-cols-1">
        <Meta label={ru.detail.category}>{r.category.name}</Meta>
        <Meta label={ru.detail.department}>
          {r.department ? r.department.name : <span className="text-warning">{ru.inbox.unrouted}</span>}
          <span className="mt-0.5 block text-xs text-muted">{routing}</span>
        </Meta>
        <Meta label={ru.detail.assignee}>{r.assignee?.name ?? <span className="text-muted">{ru.common.none}</span>}</Meta>
        <Meta label={ru.detail.author}>
          <span className="inline-flex items-center gap-1">
            {r.author.anonymous && <EyeOff className="h-3.5 w-3.5 text-muted" aria-hidden="true" />}
            {r.author.is_me ? `${r.author.name} (${ru.common.you.toLowerCase()})` : r.author.name}
          </span>
        </Meta>
        <Meta label={ru.detail.created}>
          <span className="tabular">{formatDateTime(r.created_at)}</span>
        </Meta>
        <Meta label={r.sla.first_responded_at ? ru.detail.resolutionDue : ru.detail.firstDue}>
          <span className="tabular">{formatDateTime(r.sla.first_responded_at ? r.sla.resolution_due : r.sla.first_response_due)}</span>
        </Meta>
      </dl>
    </Card>
  );
}

const ACTION_ICON: Partial<Record<Status, typeof Search>> = {
  in_review: Search,
  needs_info: MessageCircleQuestion,
  decision: Scale,
  in_progress: Hammer,
  implemented: CircleCheckBig,
  declined: CircleX,
  duplicate: Copy,
  closed: Archive,
};

function StaffActions({ r }: { r: RequestDetail }) {
  const toast = useToast();
  const { status } = useRequestMutations(r.id);
  const [closing, setClosing] = useState<Status | null>(null);
  const [question, setQuestion] = useState(false);
  const [assign, setAssign] = useState(false);
  const [theme, setTheme] = useState(false);
  const [pending, setPending] = useState<Status | null>(null);
  // "Запросить уточнение" is a question comment (it sets needs_info), so it follows allowed_comment_kinds.
  const actions = sortActions([
    ...r.allowed_statuses.filter((s) => s !== 'needs_info'),
    ...(r.allowed_comment_kinds.includes('question') ? (['needs_info'] as Status[]) : []),
  ], r.status);
  const primary = actions.find(
    (s) => !isBackward(r.status, s) && (s === 'in_review' || s === 'decision' || s === 'in_progress' || s === 'implemented'),
  );

  function run(s: Status) {
    if (s === 'needs_info') return setQuestion(true);
    if (needsResult(s)) return setClosing(s);
    setPending(s);
    status.mutate(
      { status: s },
      {
        onSuccess: () => toast.success(ru.detail.statusChanged(statusLabel(s))),
        onError: (e) => toast.error(e.message),
        onSettled: () => setPending(null),
      },
    );
  }

  const variantFor = (s: Status): ButtonVariant => (s === primary ? 'primary' : s === 'declined' ? 'danger-soft' : 'secondary');

  return (
    <Card>
      <CardHeader title={ru.detail.staffPanel} as="h2" />
      <div className="flex flex-col gap-2 px-6 pb-6 pt-4 max-sm:px-4">
        {actions.length === 0 && <p className="text-sm text-muted">{ru.detail.noActions}</p>}
        {actions.map((s) => {
          const Icon = isBackward(r.status, s) ? Undo2 : ACTION_ICON[s];
          return (
            <Button
              key={s}
              variant={variantFor(s)}
              className="w-full justify-start"
              onClick={() => run(s)}
              loading={pending === s}
              disabled={status.isPending && pending !== s}
              icon={Icon && <Icon className="h-4 w-4" aria-hidden="true" />}
            >
              {actionLabel(r.status, s)}
            </Button>
          );
        })}
        {(r.can_assign || !isTerminal(r.status)) && <div className="my-2 h-px bg-border" aria-hidden="true" />}
        {r.can_assign && (
          <Button variant="ghost" className="w-full justify-start" onClick={() => setAssign(true)} icon={<UserCog className="h-4 w-4" aria-hidden="true" />}>
            {ru.detail.assign}
          </Button>
        )}
        <Button variant="ghost" className="w-full justify-start" onClick={() => setTheme(true)} icon={<Tags className="h-4 w-4" aria-hidden="true" />}>
          {r.theme ? `${ru.detail.theme}: ${r.theme.title}` : ru.detail.themeAdd}
        </Button>
      </div>
      {closing && <CloseDialog key={closing} request={r} status={closing} open onOpenChange={(o) => !o && setClosing(null)} />}
      {question && <QuestionDialog request={r} open onOpenChange={setQuestion} />}
      {assign && <AssignDialog request={r} open onOpenChange={setAssign} />}
      {theme && <ThemeDialog request={r} open onOpenChange={setTheme} />}
    </Card>
  );
}

function SimilarCard({ r }: { r: RequestDetail }) {
  const items = r.similar.items ?? [];
  return (
    <Card>
      <CardHeader title={ru.detail.similar} as="h2" description={r.similar.count > 0 ? ru.detail.similarCount(r.similar.count) : undefined} />
      <div className="px-6 pb-6 pt-4 max-sm:px-4">
        {items.length === 0 ? (
          <p className="text-sm text-muted">{ru.detail.similarNone}</p>
        ) : (
          <ul className="flex flex-col gap-2">
            {items.map((s) => (
              <li key={s.id}>
                <Link
                  to={`/requests/${s.id}`}
                  className="group block rounded-lg border border-border px-3 py-2 transition-colors duration-150 hover:border-border-strong hover:bg-surface-2"
                >
                  <span className="flex items-center justify-between gap-2">
                    <span className="tabular text-xs font-medium text-muted">{s.number}</span>
                    <span className="tabular text-xs text-muted">{ru.detail.similarScore(Math.round(s.score * 100))}</span>
                  </span>
                  <span className="mt-1 line-clamp-2 block text-sm text-text group-hover:text-brand">{s.title}</span>
                  <span className="mt-2 block">
                    <StatusChip status={s.status} />
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </div>
    </Card>
  );
}

function DetailSkeleton() {
  return (
    <LoadingBlock>
      <Skeleton className="mb-6 h-5 w-32" />
      <div className="flex gap-2">
        <Skeleton className="h-6 w-20" />
        <Skeleton className="h-6 w-28 rounded-full" />
        <Skeleton className="h-6 w-32 rounded-full" />
      </div>
      <Skeleton className="mt-4 h-9 w-2/3" />
      <div className="mt-8 grid grid-cols-1 gap-8 lg:grid-cols-[minmax(0,1fr)_340px]">
        <div className="flex flex-col gap-6">
          <Skeleton className="h-48" />
          <Skeleton className="h-6 w-32" />
          {[0, 1, 2].map((i) => (
            <div key={i} className="flex gap-4">
              <Skeleton className="h-8 w-8 rounded-full" />
              <div className="flex-1">
                <Skeleton className="h-4 w-48" />
                <Skeleton className="mt-2 h-14" />
              </div>
            </div>
          ))}
        </div>
        <div className="flex flex-col gap-6">
          <Skeleton className="h-56" />
          <Skeleton className="h-48" />
        </div>
      </div>
    </LoadingBlock>
  );
}
