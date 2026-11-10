import * as RSelect from '@radix-ui/react-select';
import { Check, ChevronDown } from 'lucide-react';
import { useContext, type ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';
import { controlClasses, FieldContext } from './Field';

export interface SelectOption {
  value: string;
  label: ReactNode;
  disabled?: boolean;
}

export interface SelectProps {
  value: string;
  onValueChange: (v: string) => void;
  options: SelectOption[];
  placeholder?: string;
  /** Accessible name when not inside a FormField. */
  'aria-label'?: string;
  id?: string;
  className?: string;
  disabled?: boolean;
  size?: 'sm' | 'md';
}

/** Radix Select styled as a form control. Use the sentinel value "__all" for "any". */
export function Select({
  value,
  onValueChange,
  options,
  placeholder,
  id,
  className,
  disabled,
  size = 'md',
  ...rest
}: SelectProps) {
  const f = useContext(FieldContext);
  return (
    <RSelect.Root value={value} onValueChange={onValueChange} disabled={disabled}>
      <RSelect.Trigger
        id={id ?? f?.id}
        aria-label={rest['aria-label']}
        aria-describedby={f?.describedBy}
        aria-invalid={f?.invalid || undefined}
        className={cn(
          controlClasses,
          'inline-flex items-center justify-between gap-2 px-3 text-left',
          size === 'sm' ? 'h-9 text-sm max-md:h-11' : 'h-10 text-base max-md:h-11',
          f?.invalid ? 'border-danger' : 'border-border',
          'data-[placeholder]:text-muted',
          className,
        )}
      >
        <span className="min-w-0 truncate">
          <RSelect.Value placeholder={placeholder} />
        </span>
        <RSelect.Icon>
          <ChevronDown className="h-4 w-4 shrink-0 text-muted" aria-hidden="true" />
        </RSelect.Icon>
      </RSelect.Trigger>
      <RSelect.Portal>
        <RSelect.Content
          position="popper"
          sideOffset={4}
          className="z-50 max-h-[min(var(--radix-select-content-available-height),360px)] min-w-[var(--radix-select-trigger-width)] overflow-hidden rounded-lg border border-border bg-surface shadow-md animate-pop-in"
        >
          <RSelect.Viewport className="p-1">
            {options.map((o) => (
              <RSelect.Item
                key={o.value}
                value={o.value}
                disabled={o.disabled}
                className="relative flex min-h-9 cursor-pointer select-none items-center rounded-md py-2 pl-8 pr-3 text-sm text-text outline-none data-[highlighted]:bg-surface-2 data-[disabled]:opacity-50 max-md:min-h-11"
              >
                <RSelect.ItemIndicator className="absolute left-2 inline-flex">
                  <Check className="h-4 w-4 text-brand" aria-hidden="true" />
                </RSelect.ItemIndicator>
                <RSelect.ItemText>{o.label}</RSelect.ItemText>
              </RSelect.Item>
            ))}
          </RSelect.Viewport>
        </RSelect.Content>
      </RSelect.Portal>
    </RSelect.Root>
  );
}
