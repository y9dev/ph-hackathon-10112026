import {
  createContext,
  forwardRef,
  useContext,
  useId,
  type InputHTMLAttributes,
  type ReactNode,
  type TextareaHTMLAttributes,
} from 'react';
import { CircleAlert } from 'lucide-react';
import { cn } from '@/shared/lib/cn';
import { ru } from '@/shared/i18n/ru';

interface FieldCtx {
  id: string;
  describedBy?: string;
  invalid: boolean;
}
const Ctx = createContext<FieldCtx | null>(null);
export const FieldContext = Ctx;

/** Gives children the ids needed to tie label, hint and error together. */
export function useField(): FieldCtx {
  const c = useContext(Ctx);
  if (!c) throw new Error('useField must be used inside <FormField>');
  return c;
}

export interface FormFieldProps {
  label: ReactNode;
  hint?: ReactNode;
  error?: string;
  optional?: boolean;
  /** Extra content on the label row (e.g. character counter). */
  aside?: ReactNode;
  className?: string;
  children: ReactNode;
  /** Render label as <legend> inside a fieldset (for radio groups / card pickers). */
  asFieldset?: boolean;
  id?: string;
}

export function FormField({ label, hint, error, optional, aside, className, children, asFieldset, id }: FormFieldProps) {
  const auto = useId();
  const fid = id ?? `f${auto.replace(/:/g, '')}`;
  const hintId = hint ? `${fid}-hint` : undefined;
  const errId = error ? `${fid}-err` : undefined;
  const describedBy = [errId, hintId].filter(Boolean).join(' ') || undefined;
  const labelInner = (
    <>
      {label}
      {optional && <span className="ml-1 font-normal text-muted">({ru.common.optional})</span>}
    </>
  );
  const body = (
    <>
      <div className="flex items-baseline justify-between gap-3">
        {asFieldset ? (
          <legend className="text-sm font-medium text-text">{labelInner}</legend>
        ) : (
          <label htmlFor={fid} className="text-sm font-medium text-text">
            {labelInner}
          </label>
        )}
        {aside}
      </div>
      {hint && (
        <p id={hintId} className="text-sm text-muted">
          {hint}
        </p>
      )}
      <Ctx.Provider value={{ id: fid, describedBy, invalid: Boolean(error) }}>{children}</Ctx.Provider>
      {error && <FieldError id={errId!}>{error}</FieldError>}
    </>
  );
  return asFieldset ? (
    <fieldset className={cn('flex min-w-0 flex-col gap-2', className)} aria-describedby={describedBy}>
      {body}
    </fieldset>
  ) : (
    <div className={cn('flex min-w-0 flex-col gap-2', className)}>{body}</div>
  );
}

export function FieldError({ id, children }: { id: string; children: ReactNode }) {
  return (
    <p id={id} className="flex items-start gap-1 text-sm text-danger animate-fade-in">
      <CircleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
      <span>{children}</span>
    </p>
  );
}

export const controlClasses =
  'w-full rounded-lg border bg-surface text-text placeholder:text-muted/80 ' +
  'transition-[border-color,box-shadow] duration-150 ease-out ' +
  'hover:border-border-strong focus-visible:outline-none focus-visible:border-focus focus-visible:ring-2 focus-visible:ring-focus/30 ' +
  'disabled:opacity-60 disabled:cursor-not-allowed';

export const Input = forwardRef<HTMLInputElement, InputHTMLAttributes<HTMLInputElement>>(function Input(
  { className, ...rest },
  ref,
) {
  const f = useContext(Ctx);
  return (
    <input
      ref={ref}
      id={rest.id ?? f?.id}
      aria-describedby={rest['aria-describedby'] ?? f?.describedBy}
      aria-invalid={f?.invalid || undefined}
      className={cn(
        controlClasses,
        'h-10 px-3 text-base max-md:h-11',
        f?.invalid ? 'border-danger' : 'border-border',
        className,
      )}
      {...rest}
    />
  );
});

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaHTMLAttributes<HTMLTextAreaElement>>(function Textarea(
  { className, ...rest },
  ref,
) {
  const f = useContext(Ctx);
  return (
    <textarea
      ref={ref}
      id={rest.id ?? f?.id}
      aria-describedby={rest['aria-describedby'] ?? f?.describedBy}
      aria-invalid={f?.invalid || undefined}
      className={cn(
        controlClasses,
        'min-h-24 resize-y px-3 py-2 text-base leading-relaxed',
        f?.invalid ? 'border-danger' : 'border-border',
        className,
      )}
      {...rest}
    />
  );
});

/** "120 из 140" counter; turns warning near the limit and danger above it. */
export function Counter({ value, max }: { value: number; max: number }) {
  const over = value > max;
  const near = !over && value > max * 0.9;
  return (
    <span
      className={cn('tabular text-xs', over ? 'text-danger font-medium' : near ? 'text-warning' : 'text-muted')}
      aria-live={near || over ? 'polite' : 'off'}
    >
      {ru.submit.counter(value, max)}
    </span>
  );
}
