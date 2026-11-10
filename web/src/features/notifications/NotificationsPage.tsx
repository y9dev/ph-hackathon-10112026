import { useState } from 'react';
import { Bell, CheckCheck } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { api } from '@/shared/api/client';
import { qk, shouldRetry } from '@/shared/api/queries';
import { ru } from '@/shared/i18n/ru';
import { Button, Card, EmptyState, ErrorState, LoadingBlock, PageHeader, SegmentedControl, Skeleton, useDocumentTitle } from '@/shared/ui';
import { NotificationItem } from './NotificationItem';
import { POLL_MS, useMarkRead } from './api';

export default function NotificationsPage() {
  useDocumentTitle(ru.notifications.title);
  const [filter, setFilter] = useState<'all' | 'unread'>('all');
  const q = useQuery({
    queryKey: [...qk.notifications, { limit: 100, unread: filter === 'unread' }],
    queryFn: () => api.notifications({ limit: 100, unread: filter === 'unread' || undefined }),
    refetchInterval: POLL_MS,
    retry: shouldRetry,
  });
  const mark = useMarkRead();
  const unread = q.data?.unread_count ?? 0;

  return (
    <div className="mx-auto max-w-3xl">
      <PageHeader
        title={ru.notifications.title}
        lead={ru.notifications.lead}
        actions={
          unread > 0 && (
            <Button
              onClick={() => mark.mutate({ all: true })}
              loading={mark.isPending}
              icon={<CheckCheck className="h-4 w-4" aria-hidden="true" />}
            >
              {ru.notifications.markAll}
            </Button>
          )
        }
      />
      <SegmentedControl
        className="mb-4 max-w-xs"
        size="sm"
        aria-label={ru.notifications.title}
        value={filter}
        onChange={setFilter}
        options={[
          { value: 'all', label: ru.common.all },
          { value: 'unread', label: `${ru.notifications.unreadOnly}${unread ? ` · ${unread}` : ''}` },
        ]}
      />
      <Card className="p-2">
        {q.isPending ? (
          <LoadingBlock>
            {[0, 1, 2, 3, 4].map((i) => (
              <div key={i} className="flex gap-3 px-3 py-4">
                <Skeleton className="h-8 w-8 rounded-full" />
                <div className="flex-1">
                  <Skeleton className="h-4 w-2/3" />
                  <Skeleton className="mt-2 h-3 w-1/2" />
                </div>
              </div>
            ))}
          </LoadingBlock>
        ) : q.isError ? (
          <ErrorState error={q.error} onRetry={() => q.refetch()} />
        ) : q.data.items.length === 0 ? (
          <EmptyState icon={<Bell className="h-6 w-6" />} title={ru.notifications.emptyTitle} text={ru.notifications.emptyText} />
        ) : (
          <ul className="divide-y divide-border">
            {q.data.items.map((n) => (
              <li key={n.id}>
                <NotificationItem n={n} onOpen={(x) => !x.read && mark.mutate({ ids: [x.id] })} />
              </li>
            ))}
          </ul>
        )}
      </Card>
    </div>
  );
}
