import * as RToast from '@radix-ui/react-toast';
import { CircleCheck, CircleAlert, X } from 'lucide-react';
import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';
import { ru } from '@/shared/i18n/ru';

type ToastTone = 'success' | 'error';
interface ToastItem {
  id: number;
  tone: ToastTone;
  title: string;
  description?: string;
}
interface ToastApi {
  success: (title: string, description?: string) => void;
  error: (title: string, description?: string) => void;
}

const Ctx = createContext<ToastApi | null>(null);
let seq = 0;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<ToastItem[]>([]);
  const push = useCallback((tone: ToastTone, title: string, description?: string) => {
    seq += 1;
    const id = seq;
    setItems((cur) => [...cur.slice(-2), { id, tone, title, description }]);
  }, []);
  const api = useMemo<ToastApi>(
    () => ({ success: (t, d) => push('success', t, d), error: (t, d) => push('error', t, d) }),
    [push],
  );
  return (
    <Ctx.Provider value={api}>
      <RToast.Provider swipeDirection="right" duration={5000} label={ru.notifications.title}>
        {children}
        {items.map((t) => (
          <RToast.Root
            key={t.id}
            type={t.tone === 'error' ? 'foreground' : 'background'}
            onOpenChange={(open) => !open && setItems((cur) => cur.filter((x) => x.id !== t.id))}
            className={cn(
              'pointer-events-auto flex w-full items-start gap-3 rounded-xl border bg-surface p-4 shadow-lg animate-slide-up',
              'data-[swipe=move]:translate-x-[var(--radix-toast-swipe-move-x)] data-[state=closed]:animate-fade-in data-[state=closed]:opacity-0',
              t.tone === 'error' ? 'border-danger' : 'border-border',
            )}
          >
            {t.tone === 'success' ? (
              <CircleCheck className="mt-0.5 h-5 w-5 shrink-0 text-success" aria-hidden="true" />
            ) : (
              <CircleAlert className="mt-0.5 h-5 w-5 shrink-0 text-danger" aria-hidden="true" />
            )}
            <div className="min-w-0 flex-1">
              <RToast.Title className="text-sm font-medium text-text">{t.title}</RToast.Title>
              {t.description && (
                <RToast.Description className="mt-1 text-sm text-muted">{t.description}</RToast.Description>
              )}
            </div>
            <RToast.Close
              aria-label={ru.common.close}
              className="-m-1 inline-flex h-8 w-8 items-center justify-center rounded-md text-muted hover:bg-surface-2 hover:text-text"
            >
              <X className="h-4 w-4" aria-hidden="true" />
            </RToast.Close>
          </RToast.Root>
        ))}
        <RToast.Viewport className="fixed bottom-0 right-0 z-[60] flex w-full max-w-sm flex-col gap-2 p-4 outline-none max-md:bottom-16" />
      </RToast.Provider>
    </Ctx.Provider>
  );
}

export function useToast(): ToastApi {
  const c = useContext(Ctx);
  if (!c) throw new Error('useToast must be used inside <ToastProvider>');
  return c;
}
