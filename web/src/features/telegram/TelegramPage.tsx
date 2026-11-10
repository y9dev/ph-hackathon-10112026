// CR4: link the account to the Telegram bot. Shows a QR of the bot link, a rotating short code with a
// countdown (refetched exactly when it expires and when the tab regains focus) and polls the link status
// every 3 s until linked. Hidden entirely when /meta features.telegram is false.
import { useEffect, useState } from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { QRCodeSVG } from 'qrcode.react';
import { BellOff, CircleCheckBig, Copy, Check, ExternalLink } from 'lucide-react';
import { api, ApiError } from '@/shared/api/client';
import { shouldRetry, useMeta } from '@/shared/api/queries';
import type { TelegramLinkCode, TelegramStatus } from '@/shared/api/types';
import { formatDateTime } from '@/shared/lib/format';
import { ru } from '@/shared/i18n/ru';
import { Button, ButtonLink, buttonClasses, Card, EmptyState, ErrorState, LoadingBlock, PageHeader, Skeleton, useDocumentTitle, useToast } from '@/shared/ui';
import { copyText } from '@/features/anonymous/TokenCard';
import { TelegramIcon } from './TelegramIcon';

export const STATUS_POLL_MS = 3000;
const statusKey = ['telegram', 'status'] as const;
const codeKey = ['telegram', 'link-code'] as const;

export default function TelegramPage() {
  useDocumentTitle(ru.telegram.title);
  const meta = useMeta();
  const featureOn = meta.data?.features?.telegram === true;
  const status = useQuery({
    queryKey: statusKey,
    queryFn: api.telegramStatus,
    enabled: featureOn,
    retry: shouldRetry,
    // Poll while waiting for the user to send the code to the bot.
    refetchInterval: (q) => (q.state.data?.enabled && !q.state.data.linked ? STATUS_POLL_MS : false),
    refetchIntervalInBackground: false,
  });

  return (
    <div className="mx-auto max-w-3xl">
      <PageHeader title={ru.telegram.title} lead={ru.telegram.lead} />
      {meta.isPending || (featureOn && status.isPending) ? (
        <LoadingBlock>
          <Skeleton className="h-80" />
        </LoadingBlock>
      ) : meta.isError ? (
        <ErrorState error={meta.error} onRetry={() => meta.refetch()} />
      ) : !featureOn || status.data?.enabled === false ? (
        <Disabled />
      ) : status.isError ? (
        <ErrorState error={status.error} onRetry={() => status.refetch()} />
      ) : status.data!.linked ? (
        <Linked s={status.data!} />
      ) : (
        <LinkFlow s={status.data!} onLinkedElsewhere={() => status.refetch()} />
      )}
    </div>
  );
}

function Disabled() {
  return (
    <div className="rounded-xl border border-border bg-surface" data-testid="telegram-disabled">
      <EmptyState
        icon={<BellOff className="h-6 w-6" />}
        title={ru.telegram.disabledTitle}
        text={ru.telegram.disabledText}
        action={<ButtonLink to="/notifications">{ru.telegram.toNotifications}</ButtonLink>}
      />
    </div>
  );
}

function Linked({ s }: { s: TelegramStatus }) {
  return (
    <Card className="p-6 max-sm:p-4" data-testid="telegram-linked">
      <div className="flex items-start gap-3" role="status">
        <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-success-soft text-success" aria-hidden="true">
          <CircleCheckBig className="h-6 w-6" />
        </span>
        <div className="min-w-0">
          <h2 className="text-lg font-semibold text-text">{ru.telegram.linkedTitle}</h2>
          {s.telegram_username && <p className="mt-1 text-sm text-text [overflow-wrap:anywhere]">{ru.telegram.linkedAs(s.telegram_username)}</p>}
          {s.linked_at && <p className="tabular mt-1 text-sm text-muted">{ru.telegram.linkedAt(formatDateTime(s.linked_at))}</p>}
        </div>
      </div>
      <ul className="mt-4 flex flex-col gap-1 text-sm text-text">
        <li>{ru.telegram.linkedMirror}</li>
        <li>{ru.telegram.linkedPermanent}</li>
      </ul>
      <h3 className="mt-6 text-sm font-semibold text-text">{ru.telegram.commandsTitle}</h3>
      <dl className="mt-2 grid grid-cols-1 gap-x-4 gap-y-2 text-sm sm:grid-cols-[auto_1fr]">
        {ru.telegram.commands.map((c) => (
          <div key={c.cmd} className="contents">
            <dt className="font-mono text-text">{c.cmd}</dt>
            <dd className="text-muted max-sm:-mt-1 max-sm:mb-1">{c.text}</dd>
          </div>
        ))}
      </dl>
      {s.bot_url && (
        <a
          href={s.bot_url}
          target="_blank"
          rel="noopener noreferrer"
          className="mt-6 inline-flex min-h-11 items-center gap-2 text-sm font-medium text-brand underline-offset-4 hover:underline"
        >
          <TelegramIcon className="h-4 w-4" />
          {ru.telegram.openBot}
          <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
        </a>
      )}
    </Card>
  );
}

/** Seconds left until the code expires, ticking once per second. */
function useSecondsLeft(expiresAt: number | null): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (expiresAt === null) return;
    const t = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(t);
  }, [expiresAt]);
  return expiresAt === null ? 0 : Math.max(0, Math.ceil((expiresAt - now) / 1000));
}

function LinkFlow({ s, onLinkedElsewhere }: { s: TelegramStatus; onLinkedElsewhere: () => void }) {
  const toast = useToast();
  const [copied, setCopied] = useState(false);
  const code = useQuery({
    queryKey: codeKey,
    queryFn: api.telegramLinkCode,
    retry: shouldRetry,
    staleTime: Infinity,
    gcTime: 0,
    placeholderData: keepPreviousData,
    // A code shown after the tab was hidden may be stale: always refetch on focus.
    refetchOnWindowFocus: 'always',
  });
  const data: TelegramLinkCode | undefined = code.data;
  const expiresAt = data ? code.dataUpdatedAt + data.expires_in_seconds * 1000 : null;
  const left = useSecondsLeft(expiresAt);
  const period = data?.period_seconds || 20;

  // Refetch exactly when the current code expires.
  const { refetch } = code;
  useEffect(() => {
    if (expiresAt === null) return;
    const t = window.setTimeout(() => void refetch(), Math.max(0, expiresAt - Date.now()));
    return () => window.clearTimeout(t);
  }, [expiresAt, refetch]);

  // 409: the account got linked meanwhile (e.g. from another tab) → show the linked state.
  const conflict = code.error instanceof ApiError && code.error.status === 409;
  useEffect(() => {
    if (conflict) onLinkedElsewhere();
  }, [conflict, onLinkedElsewhere]);

  const botUrl = data?.bot_url || s.bot_url || '';

  async function copy() {
    if (!data) return;
    if (await copyText(data.code)) {
      setCopied(true);
      toast.success(ru.telegram.copied);
      window.setTimeout(() => setCopied(false), 2000);
    } else toast.error(ru.telegram.copyFailed);
  }

  return (
    <div className="flex flex-col gap-6">
      <Card className="p-6 max-sm:p-4">
        <h2 className="text-base font-semibold text-text">{ru.telegram.steps}</h2>
        <div className="mt-4 grid grid-cols-1 gap-6 md:grid-cols-[auto_1fr] md:gap-8">
          {botUrl && (
            <div className="flex flex-col items-center gap-2 max-md:order-2">
              <div className="rounded-xl border border-border bg-[var(--qr-bg)] p-3 text-[var(--qr-fg)]" data-testid="telegram-qr">
                <QRCodeSVG
                  value={botUrl}
                  size={168}
                  bgColor="transparent"
                  fgColor="currentColor"
                  marginSize={1}
                  title={ru.telegram.qrLabel(botUrl)}
                  role="img"
                  aria-label={ru.telegram.qrLabel(botUrl)}
                />
              </div>
              <span className="max-w-[200px] text-center text-xs text-muted [overflow-wrap:anywhere]">{botUrl.replace(/^https?:\/\//, '')}</span>
            </div>
          )}
          <ol className="flex flex-col gap-6">
            <li className="flex gap-3">
              <StepNumber n={1} />
              <div className="min-w-0">
                <p className="font-medium text-text">{ru.telegram.step1}</p>
                <p className="mt-0.5 text-sm text-muted">{ru.telegram.step1Hint}</p>
                {botUrl && (
                  <a
                    href={botUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className={buttonClasses('secondary', 'md', 'mt-2')}
                  >
                    <TelegramIcon className="h-4 w-4 shrink-0" />
                    {ru.telegram.openBot}
                    <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
                  </a>
                )}
              </div>
            </li>
            <Step n={2} title={ru.telegram.step2} hint={ru.telegram.step2Hint} />
            <li className="flex gap-3">
              <StepNumber n={3} />
              <div className="min-w-0 flex-1">
                <p className="font-medium text-text">{ru.telegram.step3}</p>
                <p className="mt-0.5 text-sm text-muted">{ru.telegram.step3Hint}</p>
                <CodeBox
                  code={data?.code}
                  left={left}
                  period={period}
                  loading={code.isPending}
                  refreshing={code.isFetching && left === 0}
                  copied={copied}
                  onCopy={copy}
                />
                {code.isError && !conflict && (
                  <div className="mt-3">
                    <ErrorState compact error={code.error} onRetry={() => code.refetch()} />
                  </div>
                )}
              </div>
            </li>
          </ol>
        </div>
      </Card>
      <p className="flex items-center gap-2 text-sm text-muted" role="status" aria-live="polite">
        <span className="h-2 w-2 shrink-0 animate-pulse rounded-full bg-brand motion-reduce:animate-none" aria-hidden="true" />
        {ru.telegram.waiting}
      </p>
    </div>
  );
}

function StepNumber({ n }: { n: number }) {
  return (
    <span className="tabular flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-brand-soft text-sm font-semibold text-brand" aria-hidden="true">
      {n}
    </span>
  );
}

function Step({ n, title, hint }: { n: number; title: string; hint: string }) {
  return (
    <li className="flex gap-3">
      <StepNumber n={n} />
      <div>
        <p className="font-medium text-text">{title}</p>
        <p className="mt-0.5 text-sm text-muted">{hint}</p>
      </div>
    </li>
  );
}

const R = 18;
const CIRC = 2 * Math.PI * R;

/** Large monospace code + countdown ring. Fixed size, so a new code never shifts the layout. */
function CodeBox({
  code,
  left,
  period,
  loading,
  refreshing,
  copied,
  onCopy,
}: {
  code: string | undefined;
  left: number;
  period: number;
  loading: boolean;
  refreshing: boolean;
  copied: boolean;
  onCopy: () => void;
}) {
  const frac = Math.min(1, Math.max(0, left / period));
  return (
    <div className="mt-3 flex flex-wrap items-center gap-3" data-testid="telegram-code-box">
      <div className="flex h-16 min-w-[12ch] items-center justify-center rounded-xl border border-border bg-surface-2 px-4">
        {loading || !code ? (
          <Skeleton className="h-8 w-36" />
        ) : (
          <>
            <span className="tabular font-mono text-3xl font-semibold tracking-[0.2em] text-text" data-testid="telegram-code" aria-hidden="true">
              {code}
            </span>
            <span className="sr-only">
              {ru.telegram.code}: {code.split('').join(' ')}
            </span>
          </>
        )}
      </div>
      <div className="relative h-12 w-12 shrink-0" title={ru.telegram.secondsLeft(left)}>
        <svg viewBox="0 0 44 44" className="h-12 w-12 -rotate-90" aria-hidden="true">
          <circle cx="22" cy="22" r={R} fill="none" stroke="var(--chart-track)" strokeWidth="4" />
          <circle
            cx="22"
            cy="22"
            r={R}
            fill="none"
            stroke="var(--brand)"
            strokeWidth="4"
            strokeLinecap="round"
            strokeDasharray={CIRC}
            strokeDashoffset={CIRC * (1 - frac)}
            className="transition-[stroke-dashoffset] duration-1000 ease-linear motion-reduce:transition-none"
          />
        </svg>
        <span className="tabular absolute inset-0 flex items-center justify-center text-xs font-semibold text-text" aria-hidden="true">
          {ru.telegram.secondsShort(left)}
        </span>
      </div>
      <span className="sr-only" aria-live="off" data-testid="telegram-seconds">
        {refreshing ? ru.telegram.refreshing : ru.telegram.secondsLeft(left)}
      </span>
      <Button onClick={onCopy} disabled={!code} icon={copied ? <Check className="h-4 w-4" aria-hidden="true" /> : <Copy className="h-4 w-4" aria-hidden="true" />}>
        {copied ? ru.telegram.copied : ru.telegram.copy}
      </Button>
    </div>
  );
}
