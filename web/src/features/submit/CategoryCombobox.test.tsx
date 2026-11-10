import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { Category } from '@/shared/api/types';
import { FormField } from '@/shared/ui';
import { CategoryCombobox, filterCategories } from './CategoryCombobox';

const cats: Category[] = [
  { id: 1, name: 'Документооборот', description: 'Движение документов', icon: 'file-text', sort: 1, active: true },
  { id: 2, name: 'Обучение', description: 'Семинары', icon: 'graduation-cap', sort: 2, active: true },
  { id: 3, name: 'Хранение документов', description: 'Общие папки', icon: 'archive', sort: 3, active: true },
  { id: 4, name: 'Корпоративная среда', description: 'Помещения и мероприятия', icon: 'no-such-icon', sort: 4, active: true },
];

function Harness() {
  const [v, setV] = useState<number>();
  return (
    <FormField id="category-input" label="Категория">
      <CategoryCombobox categories={cats} value={v} onChange={setV} />
    </FormField>
  );
}

describe('filterCategories', () => {
  it('matches name first, then description, case and ё insensitive', () => {
    expect(filterCategories(cats, 'ДОК').map((c) => c.id)).toEqual([1, 3]);
    expect(filterCategories(cats, 'папки').map((c) => c.id)).toEqual([3]);
    expect(filterCategories(cats, '  ')).toHaveLength(4);
  });
});

describe('CategoryCombobox', () => {
  it('is a labelled combobox: type to filter, arrows + Enter select, Esc closes, description shown', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const box = screen.getByRole('combobox', { name: 'Категория' });
    expect(box).toHaveAttribute('aria-expanded', 'false');
    await user.click(box);
    expect(box).toHaveAttribute('aria-expanded', 'true');
    expect(screen.getAllByRole('option')).toHaveLength(4);
    await user.type(box, 'док');
    expect(screen.getAllByRole('option').map((o) => o.textContent)).toEqual(['ДокументооборотДвижение документов', 'Хранение документовОбщие папки']);
    await user.keyboard('{ArrowDown}');
    expect(box.getAttribute('aria-activedescendant')).toMatch(/-3$/);
    await user.keyboard('{Enter}');
    expect(box).toHaveValue('Хранение документов');
    expect(box).toHaveAttribute('aria-expanded', 'false');
    expect(screen.getByTestId('category-description')).toHaveTextContent('Общие папки');
    await user.keyboard('{ArrowDown}');
    expect(box).toHaveAttribute('aria-expanded', 'true');
    await user.keyboard('{Escape}');
    expect(box).toHaveAttribute('aria-expanded', 'false');
    expect(box).toHaveValue('Хранение документов');
  });

  it('shows a plain message when nothing matches and renders unknown icons with the fallback', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    const box = screen.getByRole('combobox', { name: 'Категория' });
    await user.click(box);
    expect(screen.getByRole('option', { name: /Корпоративная среда/ }).querySelector('svg')).not.toBeNull();
    await user.type(box, 'zzz');
    expect(screen.queryAllByRole('option')).toHaveLength(0);
    expect(screen.getByText(/Ничего не нашли/)).toBeInTheDocument();
  });
});
