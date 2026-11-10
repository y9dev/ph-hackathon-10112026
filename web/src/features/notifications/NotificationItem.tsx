import { Link } from 'react-router';
import { MessageCircleQuestion, MessageSquare, RefreshCw, AlarmClock, Inbox, UserCheck, Bell, Reply } from 'lucide-react';
import type { Notification } from '@/shared/api/types';
import { cn } from '@/shared/lib/cn';
import { formatRelative } from '@/shared/lib/format';
import { ru } from '@/shared/i18n/ru';

const ICON: Record<string, typeof Bell> = {
  question: MessageCircleQuestion,
  answer: Reply,
  comment: MessageSquare,
  status_changed: RefreshCw,
  overdue: AlarmClock,
  created: Inbox,
  unrouted: Inbox,
  assigned: UserCheck,
  reminder: AlarmClock,
};

export function NotificationItem({ n, onOpen, dense }: { n: Notification; onOpen?: (n: Notification) => void; dense?: boolean }) {
  const Icon = ICON[n.kind] ?? Bell;
  const urgent = n.kind === 'question' || n.kind === 'overdue';
  return (
    <Link
      to={`/requests/${n.request_id}`}
      onClick={() => onOpen?.(n)}
      className={cn(
        'group flex gap-3 rounded-lg px-3 transition-colors duration-150 hover:bg-surface-2',
        dense ? 'py-3' : 'py-4',
        !n.read && 'bg-brand-soft/40',
      )}
    >
      <span
        className={cn(
          'mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full',
          urgent ? 'bg-warning-soft text-warning' : 'bg-surface-2 text-muted',
        )}
        aria-hidden="true"
      >
        <Icon className="h-4 w-4" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex items-start justify-between gap-2">
          <span className={cn('text-sm text-text', !n.read && 'font-semibold')}>{n.title}</span>
          {!n.read && (
            <span className="mt-1.5 h-2 w-2 shrink-0 rounded-full bg-brand" aria-hidden="true" />
          )}
        </span>
        {n.body && <span className="mt-0.5 line-clamp-2 block text-sm text-muted">{n.body}</span>}
        <span className="mt-1 block text-xs text-muted">
          {formatRelative(n.created_at)}
          {!n.read && <span className="sr-only">. {ru.notifications.unread}</span>}
        </span>
      </span>
    </Link>
  );
}
