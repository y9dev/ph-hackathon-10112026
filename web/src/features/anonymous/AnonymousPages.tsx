// CR2 public pages (no login): /anonymous — enter the secret token; /anonymous/request — the author
// view of that request. The token stays in memory/sessionStorage for this tab, never in the URL.
import { useState, type FormEvent, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, Navigate, useNavigate } from 'react-router';
import { KeyRound, LogOut, ShieldCheck } from 'lucide-react';
import { api, ApiError } from '@/shared/api/client';
import type { Attachment, CommentKind, RequestDetail } from '@/shared/api/types';
import { downloadFile } from '@/shared/lib/download';
import { ru } from '@/shared/i18n/ru';
import { Button, ButtonLink, ErrorState, FormField, Input, useDocumentTitle } from '@/shared/ui';
import { Logo } from '@/app/Logo';
import { useAuth, homePath } from '@/features/auth/AuthProvider';
import { Detail, DetailSkeleton } from '@/features/request-detail/RequestDetailPage';
import { canonicalToken, clearCurrentToken, getCurrentToken, setCurrentToken, TOKEN_ALPHABET } from './deviceStore';

/** Cache key per token without keeping the token itself in the key (FNV-1a fingerprint). */
function anonKey(token: string | null) {
  let h = 0x811c9dc5;
  for (const ch of canonicalToken(token ?? '')) h = Math.imul(h ^ ch.charCodeAt(0), 0x01000193) >>> 0;
  return ['anonymous-request', h.toString(36)] as const;
}

function PublicLayout({ children, wide }: { children: ReactNode; wide?: boolean }) {
  const { user } = useAuth();
  return (
    <div className="min-h-dvh bg-bg">
      <a
        href="#main"
        className="sr-only z-[70] rounded-lg bg-surface px-4 py-2 text-sm font-medium text-text shadow-md focus:not-sr-only focus:fixed focus:left-4 focus:top-4"
      >
        {ru.app.skipToContent}
      </a>
      <header className="sticky top-0 z-20 border-b border-border bg-surface/90 backdrop-blur-md">
        <div className="mx-auto flex h-16 max-w-[1200px] items-center justify-between gap-3 px-4 md:px-8">
          <Link to="/" aria-label={ru.app.name} className="inline-flex min-h-11 items-center rounded-lg">
            <Logo compact className="sm:hidden" />
            <Logo className="max-sm:hidden" />
          </Link>
          {user ? (
            <ButtonLink to={homePath(user.role)} size="sm">
              {ru.anonymous.toHome}
            </ButtonLink>
          ) : (
            <ButtonLink to="/login" size="sm">
              {ru.anonymous.toLogin}
            </ButtonLink>
          )}
        </div>
      </header>
      <main id="main" tabIndex={-1} className={`mx-auto w-full px-4 pb-16 pt-8 outline-none md:px-8 ${wide ? 'max-w-[1200px]' : 'max-w-xl'}`}>
        {children}
      </main>
    </div>
  );
}

/** Client check before calling the server: letters/digits of the token alphabet only (length is checked by the server). */
export function tokenInputError(raw: string): string | undefined {
  const c = canonicalToken(raw);
  if (!c) return ru.anonymous.tokenRequired;
  if (!TOKEN_ALPHABET.test(c)) return ru.anonymous.tokenInvalid;
  return undefined;
}

export function AnonymousLookupPage() {
  useDocumentTitle(ru.anonymous.lookupTitle);
  const navigate = useNavigate();
  const qc = useQueryClient();
  const [token, setToken] = useState('');
  const [error, setError] = useState<string>();
  const lookup = useMutation({
    mutationFn: (t: string) => api.anonymousLookup(t),
    onSuccess: (r, t) => {
      setCurrentToken(t);
      qc.setQueryData(anonKey(t), r);
      navigate('/anonymous/request');
    },
    onError: (e) => setError(e instanceof ApiError && e.status === 404 ? ru.anonymous.notFound : e.message),
  });

  function submit(e: FormEvent) {
    e.preventDefault();
    if (lookup.isPending) return;
    const err = tokenInputError(token);
    if (err) return setError(err);
    setError(undefined);
    lookup.mutate(token.trim());
  }

  return (
    <PublicLayout>
      <div className="flex flex-col items-start gap-2">
        <span className="flex h-12 w-12 items-center justify-center rounded-full bg-brand-soft text-brand" aria-hidden="true">
          <KeyRound className="h-6 w-6" />
        </span>
        <h1 className="mt-4 text-2xl font-semibold tracking-tight text-text md:text-3xl">{ru.anonymous.lookupTitle}</h1>
        <p className="text-base text-muted">{ru.anonymous.lookupLead}</p>
      </div>
      <form onSubmit={submit} noValidate className="mt-8 flex flex-col gap-4 rounded-xl border border-border bg-surface p-6 shadow-sm max-sm:p-4">
        <FormField label={ru.anonymous.tokenLabel} hint={ru.anonymous.tokenHint} error={error}>
          <Input
            value={token}
            onChange={(e) => {
              setToken(e.target.value);
              if (error) setError(undefined);
            }}
            placeholder={ru.anonymous.tokenPlaceholder}
            autoComplete="off"
            autoCapitalize="characters"
            spellCheck={false}
            autoFocus
            className="font-mono tracking-wider"
          />
        </FormField>
        <Button type="submit" variant="primary" size="lg" loading={lookup.isPending} loadingText={ru.anonymous.submitting} className="w-full">
          {ru.anonymous.submit}
        </Button>
        <p className="flex items-start gap-2 text-sm text-muted">
          <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          {ru.anonymous.privacy}
        </p>
      </form>
    </PublicLayout>
  );
}

export function AnonymousRequestPage() {
  const navigate = useNavigate();
  const qc = useQueryClient();
  const token = getCurrentToken();
  const q = useQuery({
    queryKey: anonKey(token),
    queryFn: () => api.anonymousLookup(token!),
    enabled: Boolean(token),
    retry: false,
    staleTime: 15_000,
  });
  useDocumentTitle(q.data ? `${q.data.number} · ${q.data.title}` : ru.anonymous.title);
  const comment = useMutation({
    mutationFn: (b: { kind: CommentKind; body: string }) => api.anonymousComment(token!, b.body),
    onSuccess: (r: RequestDetail) => qc.setQueryData(anonKey(token), r),
    onError: () => qc.invalidateQueries({ queryKey: anonKey(token) }),
  });
  const download = (a: Attachment) => downloadFile(api.anonymousAttachmentUrl(a.id), a.filename, { 'X-Anonymous-Token': token! });

  if (!token) return <Navigate to="/anonymous" replace />;

  function leave() {
    clearCurrentToken();
    qc.removeQueries({ queryKey: ['anonymous-request'] });
    navigate('/anonymous', { replace: true });
  }

  return (
    <PublicLayout wide>
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-border bg-surface px-4 py-3">
        <p className="flex items-center gap-2 text-sm text-text">
          <ShieldCheck className="h-4 w-4 shrink-0 text-brand" aria-hidden="true" />
          {ru.anonymous.banner}
        </p>
        <Button size="sm" variant="ghost" onClick={leave} icon={<LogOut className="h-4 w-4" aria-hidden="true" />}>
          {ru.anonymous.leave}
        </Button>
      </div>
      {q.isPending ? (
        <DetailSkeleton />
      ) : q.isError ? (
        <ErrorState
          error={q.error}
          onRetry={() => q.refetch()}
          notFoundTitle={ru.errors.requestNotFoundTitle}
          notFoundText={ru.anonymous.notFound}
        />
      ) : (
        <Detail r={q.data} mode={{ kind: 'token', comment, download }} />
      )}
    </PublicLayout>
  );
}
