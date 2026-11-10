import { useRef, type KeyboardEvent, type ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';

export interface SegmentOption<T extends string> {
  value: T;
  label: ReactNode;
  icon?: ReactNode;
  hint?: ReactNode;
  /** Accessible name when the visible label is shortened (e.g. on narrow screens). */
  ariaLabel?: string;
}

export interface SegmentedControlProps<T extends string> {
  value: T | undefined;
  onChange: (v: T) => void;
  options: SegmentOption<T>[];
  'aria-label'?: string;
  'aria-labelledby'?: string;
  'aria-describedby'?: string;
  invalid?: boolean;
  className?: string;
  /** Let long labels wrap to two lines instead of truncating. */
  wrap?: boolean;
}

/** Radio group rendered as a segmented control (roving tabindex, arrow keys). */
export function SegmentedControl<T extends string>({
  value,
  onChange,
  options,
  invalid,
  className,
  wrap,
  ...aria
}: SegmentedControlProps<T>) {
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  const activeIndex = Math.max(
    0,
    options.findIndex((o) => o.value === value),
  );

  function onKey(e: KeyboardEvent, i: number) {
    let next = -1;
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') next = (i + 1) % options.length;
    if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') next = (i - 1 + options.length) % options.length;
    if (e.key === 'Home') next = 0;
    if (e.key === 'End') next = options.length - 1;
    if (next >= 0) {
      e.preventDefault();
      onChange(options[next].value);
      refs.current[next]?.focus();
    }
  }

  return (
    <div
      role="radiogroup"
      {...aria}
      aria-invalid={invalid || undefined}
      className={cn(
        'inline-flex w-full gap-1 rounded-lg border bg-surface-2 p-1',
        invalid ? 'border-danger' : 'border-border',
        className,
      )}
    >
      {options.map((o, i) => {
        const selected = o.value === value;
        return (
          <button
            key={o.value}
            ref={(el) => {
              refs.current[i] = el;
            }}
            type="button"
            role="radio"
            aria-checked={selected}
            aria-label={o.ariaLabel}
            tabIndex={i === activeIndex ? 0 : -1}
            onClick={() => onChange(o.value)}
            onKeyDown={(e) => onKey(e, i)}
            className={cn(
              'flex min-w-0 flex-1 items-center justify-center gap-2 rounded-lg font-medium transition-[background-color,color,box-shadow] duration-150',
              wrap ? 'min-h-10 px-2 py-1.5 text-sm leading-tight max-md:min-h-11' : 'h-10 px-3 text-sm max-md:h-11',
              selected ? 'bg-surface text-text shadow-sm ring-1 ring-border' : 'text-muted hover:text-text',
            )}
          >
            {o.icon}
            <span className={wrap ? 'text-center [overflow-wrap:anywhere]' : 'truncate'}>{o.label}</span>
          </button>
        );
      })}
    </div>
  );
}
