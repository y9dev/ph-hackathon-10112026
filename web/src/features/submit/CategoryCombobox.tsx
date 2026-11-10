import { useContext, useEffect, useId, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import { Check, ChevronDown, Search } from 'lucide-react';
import type { Category } from '@/shared/api/types';
import { cn } from '@/shared/lib/cn';
import { ru } from '@/shared/i18n/ru';
import { CategoryIcon, FieldContext, controlClasses } from '@/shared/ui';

const norm = (s: string) => s.toLowerCase().replace(/ё/g, 'е').trim();

/** Case-insensitive search over name and description (ё = е). Name matches come first. */
export function filterCategories(categories: Category[], query: string): Category[] {
  const q = norm(query);
  if (!q) return categories;
  const byName = categories.filter((c) => norm(c.name).includes(q));
  const byDesc = categories.filter((c) => !byName.includes(c) && norm(c.description).includes(q));
  return [...byName, ...byDesc];
}

/**
 * Searchable category picker (WAI-ARIA combobox with a listbox popup). Built for 18+ categories:
 * compact when closed (one field + the selected category's description), filter as you type,
 * arrows / Home / End / Enter / Esc, mouse and touch. Used inside <FormField> (label + errors).
 */
export function CategoryCombobox({
  categories,
  value,
  onChange,
  onBlur,
  inputId = 'category-input',
}: {
  categories: Category[];
  value: number | undefined;
  onChange: (id: number) => void;
  onBlur?: () => void;
  inputId?: string;
}) {
  const field = useContext(FieldContext);
  const listId = `${useId().replace(/:/g, '')}-list`;
  const selected = categories.find((c) => c.id === value);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [active, setActive] = useState(0);
  const wrapRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLUListElement>(null);
  const items = useMemo(() => filterCategories(categories, query), [categories, query]);

  // Keep the active option visible while navigating with the keyboard.
  useEffect(() => {
    if (!open) return;
    const el = listRef.current?.querySelector<HTMLElement>(`[data-index="${active}"]`);
    el?.scrollIntoView?.({ block: 'nearest' });
  }, [active, open]);

  function openList(fromQuery = '') {
    setQuery(fromQuery);
    const list = filterCategories(categories, fromQuery);
    const idx = fromQuery ? 0 : Math.max(0, list.findIndex((c) => c.id === value));
    setActive(idx);
    setOpen(true);
  }
  function close() {
    setOpen(false);
    setQuery('');
  }
  function pick(c: Category) {
    onChange(c.id);
    close();
    inputRef.current?.focus();
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      if (!open) return openList(query);
      if (items.length === 0) return;
      setActive((i) => (e.key === 'ArrowDown' ? (i + 1) % items.length : (i - 1 + items.length) % items.length));
    } else if (e.key === 'Home' && open) {
      e.preventDefault();
      setActive(0);
    } else if (e.key === 'End' && open) {
      e.preventDefault();
      setActive(Math.max(0, items.length - 1));
    } else if (e.key === 'Enter') {
      if (open && items[active]) {
        e.preventDefault();
        pick(items[active]);
      }
    } else if (e.key === 'Escape') {
      if (open) {
        e.preventDefault();
        e.stopPropagation();
        close();
      }
    } else if (e.key === 'Tab') {
      close();
    }
  }

  const activeId = open && items[active] ? `${listId}-${items[active].id}` : undefined;

  return (
    <div
      ref={wrapRef}
      className="relative flex flex-col gap-2"
      onBlur={(e) => {
        if (!wrapRef.current?.contains(e.relatedTarget as Node | null)) {
          close();
          onBlur?.();
        }
      }}
    >
      <div className="relative">
        <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-muted" aria-hidden="true">
          {selected && !open ? <CategoryIcon name={selected.icon} className="h-4 w-4 text-brand" /> : <Search className="h-4 w-4" />}
        </span>
        <input
          ref={inputRef}
          id={field?.id ?? inputId}
          type="text"
          role="combobox"
          aria-expanded={open}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={activeId}
          aria-describedby={field?.describedBy}
          aria-invalid={field?.invalid || undefined}
          autoComplete="off"
          spellCheck={false}
          value={open ? query : (selected?.name ?? '')}
          placeholder={open ? ru.submit.categorySearch : ru.submit.categoryPlaceholder}
          onChange={(e) => {
            setQuery(e.target.value);
            setActive(0);
            setOpen(true);
          }}
          onClick={() => (open ? undefined : openList())}
          onKeyDown={onKeyDown}
          className={cn(
            controlClasses,
            'h-11 pl-9 pr-11 text-base',
            field?.invalid ? 'border-danger' : 'border-border',
            selected && !open && 'font-medium',
          )}
        />
        <button
          type="button"
          tabIndex={-1}
          aria-label={ru.submit.categoryToggle}
          aria-controls={listId}
          aria-expanded={open}
          onMouseDown={(e) => e.preventDefault()}
          onClick={() => {
            if (open) close();
            else openList();
            inputRef.current?.focus();
          }}
          className="absolute right-1 top-1/2 inline-flex h-9 w-9 -translate-y-1/2 items-center justify-center rounded-md text-muted hover:bg-surface-2 hover:text-text"
        >
          <ChevronDown className={cn('h-4 w-4 transition-transform duration-150 motion-reduce:transition-none', open && 'rotate-180')} aria-hidden="true" />
        </button>
      </div>

      <ul
        ref={listRef}
        id={listId}
        role="listbox"
        aria-label={ru.submit.categoryList}
        hidden={!open}
        className="absolute inset-x-0 top-12 z-20 max-h-[min(360px,55dvh)] overflow-y-auto overscroll-contain rounded-lg border border-border bg-surface p-1 shadow-md"
      >
        {items.length === 0 ? (
          <li role="presentation" className="px-3 py-3 text-sm text-muted">
            {ru.submit.categoryEmpty}
          </li>
        ) : (
          items.map((c, i) => {
            const isSel = c.id === value;
            return (
              <li
                key={c.id}
                id={`${listId}-${c.id}`}
                role="option"
                aria-selected={isSel}
                data-index={i}
                onMouseDown={(e) => e.preventDefault()}
                onMouseMove={() => setActive(i)}
                onClick={() => pick(c)}
                className={cn(
                  'flex min-h-11 cursor-pointer items-start gap-3 rounded-md px-3 py-2',
                  i === active ? 'bg-surface-2' : '',
                )}
              >
                <span
                  className={cn(
                    'mt-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-md',
                    isSel ? 'bg-brand text-on-brand' : 'bg-surface-2 text-muted',
                  )}
                  aria-hidden="true"
                >
                  <CategoryIcon name={c.icon} className="h-4 w-4" />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block text-sm font-medium text-text">{c.name}</span>
                  <span className="block text-xs leading-snug text-muted">{c.description}</span>
                </span>
                {isSel && <Check className="mt-1 h-4 w-4 shrink-0 text-brand" aria-hidden="true" />}
              </li>
            );
          })
        )}
      </ul>

      {selected && !open && (
        <p className="text-sm text-muted" data-testid="category-description">
          {selected.description}
        </p>
      )}
    </div>
  );
}
