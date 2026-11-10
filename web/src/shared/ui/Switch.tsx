import * as RSwitch from '@radix-ui/react-switch';
import { useId, type ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';

export interface SwitchProps {
  checked: boolean;
  onCheckedChange: (v: boolean) => void;
  label: ReactNode;
  description?: ReactNode;
  disabled?: boolean;
  className?: string;
  id?: string;
}

/** Switch with a visible label and optional description tied via aria-describedby. */
export function Switch({ checked, onCheckedChange, label, description, disabled, className, id }: SwitchProps) {
  const auto = useId();
  const sid = id ?? `sw${auto.replace(/:/g, '')}`;
  const descId = description ? `${sid}-desc` : undefined;
  return (
    <div className={cn('flex items-start gap-3', className)}>
      <RSwitch.Root
        id={sid}
        checked={checked}
        onCheckedChange={onCheckedChange}
        disabled={disabled}
        aria-describedby={descId}
        className={cn(
          'relative mt-0.5 inline-flex h-6 w-10 shrink-0 items-center rounded-full border transition-colors duration-150',
          'border-border-strong bg-surface-2 data-[state=checked]:border-brand data-[state=checked]:bg-brand',
          'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-focus disabled:opacity-50',
          // 44px touch area on mobile without changing the visual size
          'before:absolute before:-inset-2.5 before:content-[""]',
        )}
      >
        <RSwitch.Thumb className="block h-4 w-4 translate-x-1 rounded-full bg-surface shadow-sm ring-1 ring-border-strong transition-transform duration-150 data-[state=checked]:translate-x-5 data-[state=checked]:ring-0" />
      </RSwitch.Root>
      <div className="min-w-0">
        <label htmlFor={sid} className="text-sm font-medium text-text">
          {label}
        </label>
        {description && (
          <div id={descId} className="mt-1 text-sm text-muted">
            {description}
          </div>
        )}
      </div>
    </div>
  );
}
