import * as RDialog from '@radix-ui/react-dialog';
import { X } from 'lucide-react';
import { useLayoutEffect, useRef, type ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';
import { ru } from '@/shared/i18n/ru';

export interface DialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: ReactNode;
  description?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
  size?: 'sm' | 'md' | 'lg';
  /** Prevent closing on outside click while a mutation is running. */
  busy?: boolean;
}

/**
 * Modal dialog: focus is trapped while open. Dialogs are usually mounted by a parent state flag (no Radix Trigger),
 * so the element focused at mount time is remembered and refocused on close (Esc, Cancel or success).
 * If that element is gone (e.g. the action button disappeared after a status change) focus moves to the
 * nearest `[data-dialog-fallback]` element, then to <main>.
 */
export function Dialog({ open, onOpenChange, title, description, children, footer, size = 'md', busy }: DialogProps) {
  const opener = useRef<HTMLElement | null>(null);
  // Layout effects of the parent run before the passive effect in which Radix moves focus into the dialog.
  useLayoutEffect(() => {
    if (!open) return;
    const a = document.activeElement;
    opener.current = a instanceof HTMLElement && a !== document.body ? a : null;
  }, [open]);
  function restoreFocus(e: Event) {
    const target = opener.current;
    e.preventDefault();
    opener.current = null;
    if (target && target.isConnected) target.focus();
    else (document.querySelector<HTMLElement>('[data-dialog-fallback]') ?? document.getElementById('main'))?.focus();
  }
  return (
    <RDialog.Root open={open} onOpenChange={(o) => (!busy || o ? onOpenChange(o) : undefined)}>
      <RDialog.Portal>
        <RDialog.Overlay className="fixed inset-0 z-40 bg-overlay animate-fade-in" />
        <RDialog.Content
          className={cn(
            'fixed z-50 flex max-h-[min(90dvh,760px)] w-[calc(100vw-32px)] flex-col rounded-xl border border-border bg-surface shadow-lg animate-pop-in',
            'left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2',
            size === 'sm' && 'max-w-md',
            size === 'md' && 'max-w-lg',
            size === 'lg' && 'max-w-2xl',
          )}
          aria-modal="true"
          onCloseAutoFocus={restoreFocus}
          onPointerDownOutside={(e) => busy && e.preventDefault()}
          onEscapeKeyDown={(e) => busy && e.preventDefault()}
          {...(description ? {} : { 'aria-describedby': undefined })}
        >
          <div className="flex items-start justify-between gap-4 border-b border-border px-6 pb-4 pt-6 max-sm:px-4">
            <div className="min-w-0">
              <RDialog.Title className="text-xl font-semibold text-text">{title}</RDialog.Title>
              {description && <RDialog.Description className="mt-1 text-sm text-muted">{description}</RDialog.Description>}
            </div>
            <RDialog.Close
              className="-mr-2 -mt-1 inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-text max-md:h-11 max-md:w-11"
              aria-label={ru.common.close}
              disabled={busy}
            >
              <X className="h-5 w-5" aria-hidden="true" />
            </RDialog.Close>
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto px-6 py-4 max-sm:px-4">{children}</div>
          {footer && (
            <div className="flex flex-wrap justify-end gap-2 border-t border-border px-6 py-4 max-sm:px-4 max-sm:[&>*]:flex-1">
              {footer}
            </div>
          )}
        </RDialog.Content>
      </RDialog.Portal>
    </RDialog.Root>
  );
}
