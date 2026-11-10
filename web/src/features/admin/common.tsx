import type { ReactNode } from 'react';
import { CircleAlert } from 'lucide-react';
import { ru } from '@/shared/i18n/ru';
import { Badge, LoadingBlock, Skeleton } from '@/shared/ui';

export function TableShell({ caption, head, children }: { caption: string; head: ReactNode; children: ReactNode }) {
  return (
    <div className="overflow-x-auto rounded-xl border border-border bg-surface shadow-sm">
      <table className="w-full min-w-[640px] border-collapse text-left text-sm">
        <caption className="sr-only">{caption}</caption>
        <thead className="bg-surface-2 text-xs text-muted">
          <tr className="border-b border-border">{head}</tr>
        </thead>
        <tbody className="divide-y divide-border">{children}</tbody>
      </table>
    </div>
  );
}

export function Th({ children, className }: { children?: ReactNode; className?: string }) {
  return (
    <th scope="col" className={`px-4 py-3 font-medium ${className ?? ''}`}>
      {children}
    </th>
  );
}

export function Td({ children, className }: { children?: ReactNode; className?: string }) {
  return <td className={`px-4 py-3 align-middle ${className ?? ''}`}>{children}</td>;
}

export function ActiveBadge({ active, user }: { active: boolean; user?: boolean }) {
  return active ? (
    <Badge tone="success">{user ? ru.common.activeUser : ru.common.active}</Badge>
  ) : (
    <Badge>{user ? ru.common.inactiveUser : ru.common.inactive}</Badge>
  );
}

export function TableSkeleton() {
  return (
    <LoadingBlock>
      <div className="rounded-xl border border-border bg-surface">
        {[0, 1, 2, 3, 4].map((i) => (
          <div key={i} className="flex gap-4 border-b border-border px-4 py-4 last:border-0">
            <Skeleton className="h-4 w-1/4" />
            <Skeleton className="h-4 w-1/4" />
            <Skeleton className="h-4 w-1/6" />
            <Skeleton className="ml-auto h-8 w-20" />
          </div>
        ))}
      </div>
    </LoadingBlock>
  );
}

export function FormAlert({ message }: { message: string | null }) {
  if (!message) return null;
  return (
    <div role="alert" className="flex items-start gap-2 rounded-lg border border-danger/30 bg-danger-soft px-4 py-3 text-sm text-danger">
      <CircleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
      <span>{message}</span>
    </div>
  );
}
