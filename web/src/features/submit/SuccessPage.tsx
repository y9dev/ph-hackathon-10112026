import { useEffect, useState, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Link, useParams } from 'react-router';
import { CircleCheckBig, ArrowRight, Plus, KeyRound } from 'lucide-react';
import { api } from '@/shared/api/client';
import { qk } from '@/shared/api/queries';
import type { RequestDetail } from '@/shared/api/types';
import { formatDateTime } from '@/shared/lib/format';
import { ru } from '@/shared/i18n/ru';
import { ButtonLink, buttonClasses, Card, EmptyState, LoadingBlock, Skeleton, useDocumentTitle } from '@/shared/ui';
import { getCreated } from './createdStore';
import { TokenCard } from '@/features/anonymous/TokenCard';
import { setCurrentToken } from '@/features/anonymous/deviceStore';

export default function SuccessPage() {
  useDocumentTitle(ru.success.title);
  const id = Number(useParams().id);
  const valid = Number.isInteger(id) && id > 0;
  // CR2: an anonymous request is not readable by the session (no author link); its 201 response is in memory.
  const [anon] = useState(() => (valid ? getCreated(id) : undefined));
  const q = useQuery({ queryKey: qk.request(id), queryFn: () => api.getRequest(id), enabled: valid && !anon });

  if (anon?.anonymous_token) return <AnonymousSuccess r={anon} token={anon.anonymous_token} />;
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
  if (q.isError) {
    // A reload of the anonymous success screen: the token was shown once and is kept on this device.
    return (
      <EmptyState
        icon={<KeyRound className="h-6 w-6" />}
        title={ru.success.lostTitle}
        text={ru.success.lostText}
        action={
          <div className="flex flex-wrap justify-center gap-3">
            <ButtonLink to="/anonymous" variant="primary">
              {ru.success.openAnon}
            </ButtonLink>
            <ButtonLink to="/my">{ru.nav.mine}</ButtonLink>
          </div>
        }
      />
    );
  }
  return <NamedSuccess r={q.data} />;
}

function Hero({ heading }: { heading: string }) {
  return (
    <div className="flex flex-col items-center text-center">
      <span className="flex h-14 w-14 items-center justify-center rounded-full bg-success-soft text-success" aria-hidden="true">
        <CircleCheckBig className="h-7 w-7" />
      </span>
      <h1 className="mt-6 text-2xl font-semibold tracking-tight text-text md:text-3xl">{heading}</h1>
    </div>
  );
}

function Summary({ r }: { r: RequestDetail }) {
  return (
    <Card className="mt-8 overflow-hidden">
      <div className="border-b border-border bg-surface-2/60 px-6 py-6 text-center max-sm:px-4">
        <p className="text-sm text-muted">{ru.success.number}</p>
        <p className="tabular mt-1 text-3xl font-semibold tracking-tight text-text" data-testid="request-number">
          {r.number}
        </p>
        <p className="mt-2 break-words text-sm text-muted">{r.title}</p>
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
    </Card>
  );
}

function NamedSuccess({ r }: { r: RequestDetail }) {
  return (
    <div className="mx-auto max-w-2xl animate-slide-up">
      <Hero heading={ru.success.heading} />
      <Summary r={r} />
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

/** CR2 success: the token is the only way back. Actions stay locked until "Я сохранил токен" is ticked. */
function AnonymousSuccess({ r, token }: { r: RequestDetail; token: string }) {
  const [saved, setSaved] = useState(false);
  // Warn before closing the tab while the token is not confirmed as saved.
  useEffect(() => {
    if (saved) return;
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };
    window.addEventListener('beforeunload', onBeforeUnload);
    return () => window.removeEventListener('beforeunload', onBeforeUnload);
  }, [saved]);

  return (
    <div className="mx-auto max-w-2xl animate-slide-up">
      <Hero heading={ru.success.anonHeading} />
      <TokenCard number={r.number} token={token} saved={saved} onSavedChange={setSaved} />
      <Summary r={r} />
      <div className="mt-8 flex flex-wrap items-center gap-3 max-sm:flex-col">
        <LockedLink to="/anonymous/request" disabled={!saved} primary token={token}>
          {ru.success.openAnon}
          <ArrowRight className="h-4 w-4" aria-hidden="true" />
        </LockedLink>
        <LockedLink to="/my" disabled={!saved}>
          {ru.success.done}
        </LockedLink>
        {!saved && (
          <p className="text-sm text-muted" id="token-gate-hint">
            {ru.success.tokenSavedHint}
          </p>
        )}
      </div>
    </div>
  );
}

function LockedLink({
  to,
  disabled,
  primary,
  token,
  children,
}: {
  to: string;
  disabled: boolean;
  primary?: boolean;
  token?: string;
  children: ReactNode;
}) {
  const cls = buttonClasses(primary ? 'primary' : 'secondary', 'lg', 'max-sm:w-full');
  if (disabled)
    return (
      <button type="button" disabled aria-describedby="token-gate-hint" className={cls}>
        {children}
      </button>
    );
  return (
    <Link to={to} className={cls} onClick={() => token && setCurrentToken(token)}>
      {children}
    </Link>
  );
}
