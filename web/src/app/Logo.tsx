import { cn } from '@/shared/lib/cn';
import { ru } from '@/shared/i18n/ru';

/** Wordmark: a speech-bubble glyph in brand teal + product name. */
export function Logo({ className, compact }: { className?: string; compact?: boolean }) {
  return (
    <div className={cn('flex items-center gap-3', className)}>
      <svg viewBox="0 0 32 32" className="h-8 w-8 shrink-0" aria-hidden="true">
        <rect width="32" height="32" rx="8" fill="var(--brand)" />
        <path
          d="M9 11.5A2.5 2.5 0 0 1 11.5 9h9A2.5 2.5 0 0 1 23 11.5v6a2.5 2.5 0 0 1-2.5 2.5H15l-4 3v-3h0A2 2 0 0 1 9 18Z"
          fill="none"
          stroke="var(--on-brand)"
          strokeWidth="1.8"
          strokeLinejoin="round"
        />
        <path d="M13 14.5h6" stroke="var(--on-brand)" strokeWidth="1.8" strokeLinecap="round" />
      </svg>
      {!compact && (
        <span className="flex flex-col leading-tight">
          <span className="text-base font-semibold tracking-tight text-text">{ru.app.name}</span>
          <span className="text-xs text-muted">{ru.app.tagline}</span>
        </span>
      )}
    </div>
  );
}
