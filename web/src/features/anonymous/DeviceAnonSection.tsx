import { useState } from 'react';
import { useQueries } from '@tanstack/react-query';
import { useNavigate } from 'react-router';
import { EyeOff, KeyRound, Trash2, ChevronRight } from 'lucide-react';
import { api } from '@/shared/api/client';
import { formatDate } from '@/shared/lib/format';
import { ru } from '@/shared/i18n/ru';
import { Button, ButtonLink, Dialog, Skeleton, StatusChip, useToast } from '@/shared/ui';
import { loadAnonymousRequests, removeAnonymousRequest, setCurrentToken, type DeviceAnonRequest } from './deviceStore';

/**
 * «Анонимные обращения на этом устройстве» (CR2). Status comes from POST /anonymous/lookup with the
 * stored token (the session never sees these requests). Always offers the "check by token" entry.
 */
export function DeviceAnonSection() {
  const [items, setItems] = useState<DeviceAnonRequest[]>(loadAnonymousRequests);
  const [removing, setRemoving] = useState<DeviceAnonRequest | null>(null);
  const navigate = useNavigate();
  const toast = useToast();
  const statuses = useQueries({
    queries: items.map((it) => ({
      queryKey: ['anonymous-device', it.id, it.number] as const,
      queryFn: () => api.anonymousLookup(it.token),
      retry: false,
      staleTime: 60_000,
    })),
  });

  function open(it: DeviceAnonRequest) {
    setCurrentToken(it.token);
    navigate('/anonymous/request');
  }

  return (
    <section aria-labelledby="device-anon-title" className="mt-10" data-testid="device-anon">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div>
          <h2 id="device-anon-title" className="flex items-center gap-2 text-base font-semibold text-text">
            <EyeOff className="h-4 w-4 text-muted" aria-hidden="true" />
            {ru.mine.anonTitle}
          </h2>
          {items.length > 0 && <p className="mt-1 text-sm text-muted">{ru.mine.anonLead}</p>}
        </div>
        <ButtonLink to="/anonymous" size="sm" className="h-auto! min-h-9 max-w-full whitespace-normal! py-2 text-left max-md:min-h-11" icon={<KeyRound className="h-4 w-4 shrink-0" aria-hidden="true" />}>
          {ru.mine.anonCheckByToken}
        </ButtonLink>
      </div>
      {items.length > 0 && (
        <ul className="mt-4 flex flex-col gap-3">
          {items.map((it, i) => {
            const q = statuses[i];
            return (
              <li key={it.token} className="flex flex-wrap items-center gap-3 rounded-xl border border-border bg-surface p-4 shadow-sm md:px-6">
                <button
                  type="button"
                  onClick={() => open(it)}
                  className="group flex min-h-11 min-w-0 flex-1 basis-60 items-center gap-3 rounded-lg text-left"
                >
                  <span className="min-w-0 flex-1">
                    <span className="tabular block text-xs font-medium text-muted">
                      {it.number} · {ru.mine.anonCreated(formatDate(it.created_at))}
                    </span>
                    <span className="mt-1 block break-words font-semibold text-text group-hover:text-brand">{it.title}</span>
                    <span className="mt-2 block">
                      {q?.isPending ? (
                        <Skeleton className="h-6 w-28 rounded-full" />
                      ) : q?.isError ? (
                        <span className="text-xs text-muted">{ru.mine.anonStatusError}</span>
                      ) : q?.data ? (
                        <StatusChip status={q.data.status} />
                      ) : null}
                    </span>
                  </span>
                  <span className="sr-only">{ru.mine.anonOpen}</span>
                  <ChevronRight className="h-5 w-5 shrink-0 text-muted" aria-hidden="true" />
                </button>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => setRemoving(it)}
                  icon={<Trash2 className="h-4 w-4" aria-hidden="true" />}
                  aria-label={`${ru.mine.anonRemove}: ${it.number}`}
                >
                  <span className="max-sm:sr-only">{ru.mine.anonRemove}</span>
                </Button>
              </li>
            );
          })}
        </ul>
      )}
      {removing && (
        <Dialog
          open
          size="sm"
          onOpenChange={(o) => !o && setRemoving(null)}
          title={ru.mine.anonRemoveTitle}
          description={ru.mine.anonRemoveText(removing.number)}
          footer={
            <>
              <Button onClick={() => setRemoving(null)}>{ru.common.cancel}</Button>
              <Button
                variant="danger"
                onClick={() => {
                  removeAnonymousRequest(removing.token);
                  setItems(loadAnonymousRequests());
                  setRemoving(null);
                  toast.success(ru.mine.anonRemoved);
                }}
              >
                {ru.mine.anonRemove}
              </Button>
            </>
          }
        >
          <p className="text-sm text-text">{removing.title}</p>
        </Dialog>
      )}
    </section>
  );
}
