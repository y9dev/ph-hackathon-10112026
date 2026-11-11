import { useMutation } from '@tanstack/react-query';
import { zodResolver } from '@hookform/resolvers/zod';
import { useForm } from 'react-hook-form';
import { Navigate, useNavigate, useSearchParams } from 'react-router';
import { z } from 'zod';
import { CircleAlert, MessageSquareText, Route, BadgeCheck } from 'lucide-react';
import { api, ApiError } from '@/shared/api/client';
import { ru } from '@/shared/i18n/ru';
import { Button, FormField, Input, useDocumentTitle } from '@/shared/ui';
import { Logo } from '@/app/Logo';
import { homePath, useAuth } from './AuthProvider';

export const loginSchema = z.object({
  email: z.string().trim().min(1, ru.auth.emailRequired).email(ru.auth.emailInvalid),
  password: z.string().trim().min(1, ru.auth.passwordRequired),
});
type LoginForm = z.infer<typeof loginSchema>;

function safeNext(next: string | null): string | null {
  return next && next.startsWith('/') && !next.startsWith('//') && !next.includes('\\') && !next.startsWith('/login') ? next : null;
}

export default function LoginPage() {
  useDocumentTitle(ru.auth.title);
  const { user, setUser, expired } = useAuth();
  const [sp] = useSearchParams();
  const navigate = useNavigate();
  const next = safeNext(sp.get('next'));

  const form = useForm<LoginForm>({ resolver: zodResolver(loginSchema), defaultValues: { email: '', password: '' } });
  const login = useMutation({
    mutationFn: (v: LoginForm) => api.login(v.email.trim(), v.password.trim()),
    onSuccess: (u) => {
      setUser(u);
      navigate(next ?? homePath(u.role), { replace: true });
    },
  });
  if (user) return <Navigate to={next ?? homePath(user.role)} replace />;

  const err = login.error;
  const formError =
    err instanceof ApiError
      ? err.status === 401
        ? ru.auth.invalid
        : err.status === 429
          ? ru.auth.throttled
          : err.message
      : err
        ? ru.errors.generic
        : null;

  return (
    <div className="min-h-dvh bg-bg">
      <main id="main" className="mx-auto grid min-h-dvh max-w-6xl grid-cols-1 items-center gap-12 px-4 py-12 md:px-8 lg:grid-cols-[1fr_440px] lg:gap-16">
        <section className="max-lg:order-2 max-lg:hidden" aria-labelledby="login-about">
          <Logo className="mb-12" />
          <h2 id="login-about" className="max-w-lg text-3xl font-semibold tracking-tight text-text">
            {ru.auth.aboutTitle}
          </h2>
          <p className="mt-4 max-w-lg text-base text-muted">{ru.auth.lead}</p>
          <ul className="mt-8 flex max-w-lg flex-col gap-4">
            {ru.auth.features.map(({ title, text }, i) => ({ title, text, icon: [MessageSquareText, Route, BadgeCheck][i] })).map(({ icon: Icon, title, text }) => (
              <li key={title} className="flex gap-4">
                <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-brand-soft text-brand" aria-hidden="true">
                  <Icon className="h-5 w-5" />
                </span>
                <div>
                  <p className="font-medium text-text">{title}</p>
                  <p className="text-sm text-muted">{text}</p>
                </div>
              </li>
            ))}
          </ul>
        </section>

        <div className="flex w-full flex-col gap-6">
          <Logo className="lg:hidden" />
          <div className="rounded-xl border border-border bg-surface p-8 shadow-md max-sm:p-6">
            <h1 className="text-2xl font-semibold tracking-tight text-text">{ru.auth.heading}</h1>
            <p className="mt-2 text-sm text-muted lg:hidden">{ru.auth.lead}</p>
            {expired && !formError && (
              <p role="status" className="mt-4 rounded-lg bg-info-soft px-4 py-3 text-sm text-info">
                {ru.auth.sessionExpired}
              </p>
            )}
            {formError && (
              <div role="alert" className="mt-4 flex items-start gap-2 rounded-lg border border-danger/30 bg-danger-soft px-4 py-3 text-sm text-danger">
                <CircleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
                <span>{formError}</span>
              </div>
            )}
            <form className="mt-6 flex flex-col gap-4" noValidate onSubmit={form.handleSubmit((v) => login.mutate(v))}>
              <FormField label={ru.auth.email} error={form.formState.errors.email?.message}>
                <Input type="email" autoComplete="username" inputMode="email" autoFocus {...form.register('email')} />
              </FormField>
              <FormField label={ru.auth.password} error={form.formState.errors.password?.message}>
                <Input type="password" autoComplete="current-password" {...form.register('password')} />
              </FormField>
              <Button type="submit" variant="primary" size="lg" className="mt-2 w-full" loading={login.isPending} loadingText={ru.auth.submitting}>
                {ru.auth.submit}
              </Button>
            </form>
          </div>
        </div>
      </main>
    </div>
  );
}
