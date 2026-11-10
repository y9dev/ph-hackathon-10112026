import { useState } from 'react';
import { Link } from 'react-router';
import { Bell, CheckCheck } from 'lucide-react';
import { ru } from '@/shared/i18n/ru';
import { cn } from '@/shared/lib/cn';
import { Button, ErrorState, Popover, PopoverContent, PopoverTrigger, Skeleton, Tooltip } from '@/shared/ui';
import { TelegramIcon } from '@/features/telegram/TelegramIcon';
import { useTelegramEnabled } from '@/features/telegram/useTelegramEnabled';
import { useMarkRead, useNotifications } from './api';
import type { Notification } from '@/shared/api/types';
import { NotificationItem } from './NotificationItem';

export function NotificationsBell() {
  const [open, setOpen] = useState(false);
  const q = useNotifications(8);
  const mark = useMarkRead();
  const unread = q.data?.unread_count ?? 0;
  const telegram = useTelegramEnabled();

  function onOpenItem(n: Notification) {
    setOpen(false);
    if (!n.read) mark.mutate({ ids: [n.id] });
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        aria-label={unread ? ru.notifications.openUnread(unread) : ru.notifications.open}
        className="relative inline-flex h-10 w-10 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-text data-[state=open]:bg-surface-2 data-[state=open]:text-text max-md:h-11 max-md:w-11"
      >
        <Bell className="h-5 w-5" aria-hidden="true" />
        {unread > 0 && (
          <span
            className="tabular absolute right-1 top-1 inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-brand px-1 text-[11px] font-semibold leading-none text-on-brand ring-2 ring-surface"
            aria-hidden="true"
          >
            {unread > 9 ? '9+' : unread}
          </span>
        )}
      </PopoverTrigger>
      <PopoverContent align="end" className="w-[min(380px,calc(100vw-24px))] p-0" aria-label={ru.notifications.title}>
        <div className="flex items-center justify-between gap-2 border-b border-border px-4 py-3">
          <h2 className="text-sm font-semibold text-text">{ru.notifications.title}</h2>
          {unread > 0 && (
            <Button
              size="sm"
              variant="ghost"
              onClick={() => mark.mutate({ all: true })}
              loading={mark.isPending}
              icon={<CheckCheck className="h-4 w-4" aria-hidden="true" />}
            >
              {ru.notifications.markAll}
            </Button>
          )}
        </div>
        <div className="max-h-[min(440px,60dvh)] overflow-y-auto p-1">
          {q.isPending ? (
            <div className="flex flex-col gap-2 p-3" aria-hidden="true">
              {[0, 1, 2].map((i) => (
                <div key={i} className="flex gap-3">
                  <Skeleton className="h-8 w-8 rounded-full" />
                  <div className="flex-1">
                    <Skeleton className="h-4 w-3/4" />
                    <Skeleton className="mt-2 h-3 w-1/2" />
                  </div>
                </div>
              ))}
            </div>
          ) : q.isError ? (
            <ErrorState compact error={q.error} onRetry={() => q.refetch()} />
          ) : q.data.items.length === 0 ? (
            <p className="px-4 py-8 text-center text-sm text-muted">{ru.notifications.empty}</p>
          ) : (
            <ul>
              {q.data.items.map((n) => (
                <li key={n.id}>
                  <NotificationItem dense n={n} onOpen={onOpenItem} />
                </li>
              ))}
            </ul>
          )}
        </div>
        <div className="flex items-center gap-1 border-t border-border p-1">
          <Link
            to="/notifications"
            onClick={() => setOpen(false)}
            className={cn('flex h-11 min-w-0 flex-1 items-center justify-center rounded-lg text-sm font-medium text-brand hover:bg-surface-2')}
          >
            {ru.notifications.all}
          </Link>
          {telegram && (
            <Tooltip content={ru.notifications.telegram}>
              <Link
                to="/telegram"
                onClick={() => setOpen(false)}
                aria-label={ru.notifications.telegram}
                data-testid="bell-telegram"
                className="inline-flex h-11 w-11 shrink-0 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-brand"
              >
                <TelegramIcon className="h-5 w-5" />
              </Link>
            </Tooltip>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}
