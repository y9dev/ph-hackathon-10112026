import { CircleAlert, Lock, SearchX, RotateCw } from 'lucide-react';
import type { ReactNode } from 'react';
import { ApiError } from '@/shared/api/client';
import { cn } from '@/shared/lib/cn';
import { ru } from '@/shared/i18n/ru';
import { Button, ButtonLink } from './Button';

export function Skeleton({ className }: { className?: string }) {
  return <div className={cn('skeleton', className)} aria-hidden="true" />;
}

/** Wraps skeleton blocks and announces loading once for assistive tech. */
export function LoadingBlock({ children, label = ru.common.loading }: { children: ReactNode; label?: string }) {
  return (
    <div role="status" aria-live="polite" aria-busy="true">
      <span className="sr-only">{label}</span>
      {children}
    </div>
  );
}

export function EmptyState({
  icon,
  title,
  text,
  action,
  className,
  compact,
}: {
  icon?: ReactNode;
  title: string;
  text?: ReactNode;
  action?: ReactNode;
  className?: string;
  compact?: boolean;
}) {
  return (
    <div
      className={cn(
        'flex flex-col items-center text-center',
        compact ? 'gap-2 px-4 py-8' : 'gap-3 px-6 py-12',
        className,
      )}
    >
      {icon && (
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-surface-2 text-muted" aria-hidden="true">
          {icon}
        </div>
      )}
      <h2 className={cn('font-semibold text-text', compact ? 'text-base' : 'text-xl')}>{title}</h2>
      {text && <p className="max-w-md text-sm text-muted">{text}</p>}
      {action && <div className="mt-2 flex flex-wrap justify-center gap-2">{action}</div>}
    </div>
  );
}

/** Error with retry; 403/404 get their own wording and no retry. */
export function ErrorState({
  error,
  onRetry,
  className,
  notFoundTitle,
  notFoundText,
  compact,
}: {
  error: unknown;
  onRetry?: () => void;
  className?: string;
  notFoundTitle?: string;
  notFoundText?: string;
  compact?: boolean;
}) {
  const status = error instanceof ApiError ? error.status : 0;
  if (status === 403) {
    return (
      <EmptyState
        compact={compact}
        className={className}
        icon={<Lock className="h-6 w-6" />}
        title={ru.errors.forbiddenTitle}
        text={ru.errors.forbidden}
        action={<ButtonLink to="/">{ru.errors.toHome}</ButtonLink>}
      />
    );
  }
  if (status === 404) {
    return (
      <EmptyState
        compact={compact}
        className={className}
        icon={<SearchX className="h-6 w-6" />}
        title={notFoundTitle ?? ru.errors.notFoundTitle}
        text={notFoundText ?? ru.errors.notFound}
        action={<ButtonLink to="/">{ru.errors.toHome}</ButtonLink>}
      />
    );
  }
  const message = error instanceof Error ? error.message : ru.errors.generic;
  return (
    <div role="alert" className={className}>
      <EmptyState
        compact={compact}
        icon={<CircleAlert className="h-6 w-6 text-danger" />}
        title={ru.errors.loadTitle}
        text={message}
        action={
          onRetry && (
            <Button onClick={onRetry} icon={<RotateCw className="h-4 w-4" aria-hidden="true" />}>
              {ru.common.retry}
            </Button>
          )
        }
      />
    </div>
  );
}
