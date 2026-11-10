import * as RTabs from '@radix-ui/react-tabs';
import type { ReactNode } from 'react';
import { cn } from '@/shared/lib/cn';

export const Tabs = RTabs.Root;
export const TabsContent = ({ className, ...rest }: RTabs.TabsContentProps) => (
  <RTabs.Content className={cn('outline-none focus-visible:outline-2 focus-visible:outline-focus', className)} {...rest} />
);

export function TabsList({ className, children, ...rest }: RTabs.TabsListProps) {
  return (
    <RTabs.List
      className={cn(
        'flex gap-1 overflow-x-auto border-b border-border [scrollbar-width:none] [&::-webkit-scrollbar]:hidden',
        className,
      )}
      {...rest}
    >
      {children}
    </RTabs.List>
  );
}

export function TabsTrigger({
  value,
  children,
  count,
  countTone = 'neutral',
}: {
  value: string;
  children: ReactNode;
  count?: number;
  countTone?: 'neutral' | 'danger';
}) {
  return (
    <RTabs.Trigger
      value={value}
      className={cn(
        'group relative -mb-px inline-flex h-11 shrink-0 items-center gap-2 whitespace-nowrap border-b-2 border-transparent px-3 text-sm font-medium text-muted',
        'transition-colors duration-150 hover:text-text',
        'data-[state=active]:border-brand data-[state=active]:text-text',
        'focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-focus',
      )}
    >
      {children}
      {count !== undefined && (
        <span
          className={cn(
            'tabular inline-flex min-w-6 items-center justify-center rounded-full px-1.5 text-xs leading-5',
            countTone === 'danger' && count > 0
              ? 'bg-danger-soft text-danger'
              : 'bg-surface-2 text-muted group-data-[state=active]:bg-brand-soft group-data-[state=active]:text-brand',
          )}
        >
          {count}
        </span>
      )}
    </RTabs.Trigger>
  );
}
