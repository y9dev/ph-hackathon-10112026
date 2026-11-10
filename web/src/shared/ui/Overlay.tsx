import * as RTooltip from '@radix-ui/react-tooltip';
import * as RPopover from '@radix-ui/react-popover';
import * as RMenu from '@radix-ui/react-dropdown-menu';
import { Info } from 'lucide-react';
import type { ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';

const floating = 'z-50 rounded-lg border border-border bg-surface shadow-md animate-pop-in';

export function Tooltip({ content, children }: { content: ReactNode; children: ReactNode }) {
  return (
    <RTooltip.Root delayDuration={200}>
      <RTooltip.Trigger asChild>{children}</RTooltip.Trigger>
      <RTooltip.Portal>
        <RTooltip.Content sideOffset={6} className={cn(floating, 'max-w-72 px-3 py-2 text-sm text-text')}>
          {content}
        </RTooltip.Content>
      </RTooltip.Portal>
    </RTooltip.Root>
  );
}

/**
 * "What does this metric mean" — a popover (works with touch and keyboard,
 * unlike a hover-only tooltip). Content is also reachable by screen readers.
 */
export function InfoTip({ label, children }: { label: string; children: ReactNode }) {
  return (
    <RPopover.Root>
      <RPopover.Trigger
        aria-label={label}
        className="inline-flex h-6 w-6 items-center justify-center rounded-full text-muted transition-colors duration-150 hover:bg-surface-2 hover:text-text max-md:h-11 max-md:w-11"
      >
        <Info className="h-4 w-4" aria-hidden="true" />
      </RPopover.Trigger>
      <RPopover.Portal>
        <RPopover.Content
          sideOffset={6}
          collisionPadding={16}
          className={cn(floating, 'max-w-72 px-4 py-3 text-sm leading-relaxed text-text')}
        >
          {children}
          <RPopover.Arrow className="fill-surface" />
        </RPopover.Content>
      </RPopover.Portal>
    </RPopover.Root>
  );
}

export const Popover = RPopover.Root;
export const PopoverTrigger = RPopover.Trigger;
export function PopoverContent({ children, className, ...rest }: RPopover.PopoverContentProps) {
  return (
    <RPopover.Portal>
      <RPopover.Content sideOffset={8} collisionPadding={12} className={cn(floating, className)} {...rest}>
        {children}
      </RPopover.Content>
    </RPopover.Portal>
  );
}

export const Menu = RMenu.Root;
export const MenuTrigger = RMenu.Trigger;
export function MenuContent({ children, className, ...rest }: RMenu.DropdownMenuContentProps) {
  return (
    <RMenu.Portal>
      <RMenu.Content sideOffset={8} collisionPadding={12} className={cn(floating, 'min-w-56 p-1', className)} {...rest}>
        {children}
      </RMenu.Content>
    </RMenu.Portal>
  );
}
export function MenuItem({ className, ...rest }: RMenu.DropdownMenuItemProps) {
  return (
    <RMenu.Item
      className={cn(
        'flex min-h-9 cursor-pointer select-none items-center gap-2 rounded-md px-3 py-2 text-sm text-text outline-none data-[highlighted]:bg-surface-2 max-md:min-h-11',
        className,
      )}
      {...rest}
    />
  );
}
export const MenuLabel = ({ className, ...rest }: RMenu.DropdownMenuLabelProps) => (
  <RMenu.Label className={cn('px-3 py-2 text-xs font-medium text-muted', className)} {...rest} />
);
export const MenuSeparator = () => <RMenu.Separator className="my-1 h-px bg-border" />;
export const MenuRadioGroup = RMenu.RadioGroup;
export function MenuRadioItem({ className, children, ...rest }: RMenu.DropdownMenuRadioItemProps) {
  return (
    <RMenu.RadioItem
      className={cn(
        'relative flex min-h-9 cursor-pointer select-none items-center gap-2 rounded-md py-2 pl-8 pr-3 text-sm text-text outline-none data-[highlighted]:bg-surface-2 max-md:min-h-11',
        className,
      )}
      {...rest}
    >
      <RMenu.ItemIndicator className="absolute left-3 h-2 w-2 rounded-full bg-brand" />
      {children}
    </RMenu.RadioItem>
  );
}
