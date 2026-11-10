import { ChevronLeft, ChevronRight } from 'lucide-react';
import { useEffect, type HTMLAttributes, type ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';
import { initials } from '@/shared/lib/format';
import { ru } from '@/shared/i18n/ru';
import { Button } from './Button';

export function Card({ className, ...rest }: HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('rounded-xl border border-border bg-surface shadow-sm', className)} {...rest} />;
}

export function CardHeader({
  title,
  action,
  description,
  className,
  as: As = 'h2',
  id,
}: {
  title: ReactNode;
  action?: ReactNode;
  description?: ReactNode;
  className?: string;
  as?: 'h2' | 'h3';
  id?: string;
}) {
  return (
    <div className={cn('flex items-start justify-between gap-3 px-6 pt-6 max-sm:px-4 max-sm:pt-4', className)}>
      <div className="min-w-0">
        <As id={id} className="text-base font-semibold text-text">
          {title}
        </As>
        {description && <p className="mt-1 text-sm text-muted">{description}</p>}
      </div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  );
}

export function PageHeader({
  title,
  lead,
  actions,
  eyebrow,
}: {
  title: ReactNode;
  lead?: ReactNode;
  actions?: ReactNode;
  eyebrow?: ReactNode;
}) {
  return (
    <header className="mb-6 flex flex-wrap items-end justify-between gap-4 max-sm:mb-4">
      <div className="min-w-0">
        {eyebrow && <div className="mb-2">{eyebrow}</div>}
        <h1 className="text-2xl font-semibold tracking-tight text-text md:text-3xl">{title}</h1>
        {lead && <p className="mt-2 max-w-2xl text-base text-muted">{lead}</p>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </header>
  );
}

/** Sets document.title as "<page> · Modus Voice". */
export function useDocumentTitle(title: string | undefined) {
  useEffect(() => {
    document.title = title ? `${title} · ${ru.app.name}` : ru.app.name;
  }, [title]);
}

export function Avatar({ name, anonymous, size = 'md' }: { name: string; anonymous?: boolean; size?: 'sm' | 'md' }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        'inline-flex shrink-0 items-center justify-center rounded-full font-semibold',
        size === 'sm' ? 'h-6 w-6 text-[11px]' : 'h-8 w-8 text-xs',
        anonymous ? 'border border-dashed border-border-strong bg-surface text-muted' : 'bg-surface-2 text-text',
      )}
    >
      {anonymous ? '?' : initials(name)}
    </span>
  );
}

export function Pagination({
  page,
  pageSize,
  total,
  onPage,
}: {
  page: number;
  pageSize: number;
  total: number;
  onPage: (p: number) => void;
}) {
  const pages = Math.max(1, Math.ceil(total / pageSize));
  if (total <= pageSize) return null;
  const from = (page - 1) * pageSize + 1;
  const to = Math.min(total, page * pageSize);
  return (
    <nav className="flex items-center justify-between gap-4" aria-label={ru.common.page(page, pages)}>
      <p className="tabular text-sm text-muted">{ru.common.shown(from, to, total)}</p>
      <div className="flex items-center gap-2">
        <Button
          size="sm"
          onClick={() => onPage(page - 1)}
          disabled={page <= 1}
          icon={<ChevronLeft className="h-4 w-4" aria-hidden="true" />}
        >
          {ru.common.prev}
        </Button>
        <span className="tabular text-sm text-muted max-sm:hidden">{ru.common.page(page, pages)}</span>
        <Button size="sm" onClick={() => onPage(page + 1)} disabled={page >= pages}>
          {ru.common.next}
          <ChevronRight className="h-4 w-4" aria-hidden="true" />
        </Button>
      </div>
    </nav>
  );
}

/** Key/value row used in request headers and panels. */
export function Meta({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="min-w-0">
      <dt className="text-xs font-medium text-muted">{label}</dt>
      <dd className="mt-1 truncate text-sm text-text">{children}</dd>
    </div>
  );
}
