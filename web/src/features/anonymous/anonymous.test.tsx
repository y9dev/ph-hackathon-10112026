import { useState } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ToastProvider } from '@/shared/ui';
import { canonicalToken, formatToken, loadAnonymousRequests, removeAnonymousRequest, saveAnonymousRequest } from './deviceStore';
import { TokenCard } from './TokenCard';
import { tokenInputError } from './AnonymousPages';
import { validateThemeTitle } from '@/features/request-detail/ActionDialogs';

const entry = { id: 21, number: 'MV-0021', title: 'Тест', token: 'ABCD-EFGH-JKMN-PQRS-TVWX-YZ01-2345', created_at: '2026-11-10T10:00:00Z' };

afterEach(() => {
  localStorage.clear();
  vi.restoreAllMocks();
});

describe('anonymous token helpers (CR2)', () => {
  it('canonicalises pasted tokens like the server and groups by 4 without a fixed group count', () => {
    expect(canonicalToken(' abcd efgh-ijkl o ')).toBe('ABCDEFGH1JK10');
    expect(formatToken('abcdefghjk')).toBe('ABCD-EFGH-JK');
  });
  it('validates input before calling the server', () => {
    expect(tokenInputError('   ')).toBe('Введите токен');
    expect(tokenInputError('ABCD-ЖЖЖЖ')).toMatch(/латинских букв и цифр/);
    expect(tokenInputError('dem0-a005 7k3q')).toBeUndefined();
  });
  it('stores requests on this device newest first, dedupes by token, removes, survives broken storage', () => {
    saveAnonymousRequest(entry);
    saveAnonymousRequest({ ...entry, id: 22, number: 'MV-0022', token: 'X' });
    saveAnonymousRequest(entry);
    expect(loadAnonymousRequests().map((e) => e.number)).toEqual(['MV-0021', 'MV-0022']);
    removeAnonymousRequest('X');
    expect(loadAnonymousRequests()).toHaveLength(1);
    localStorage.setItem('mv-anon-requests', '{broken');
    expect(loadAnonymousRequests()).toEqual([]);
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('quota');
    });
    expect(saveAnonymousRequest(entry)).toBe(false);
  });
});

describe('TokenCard', () => {
  function Harness() {
    const [saved, setSaved] = useState(false);
    return (
      <ToastProvider>
        <TokenCard number="MV-0021" token={entry.token} saved={saved} onSavedChange={setSaved} />
        <button type="button" disabled={!saved}>
          Готово
        </button>
      </ToastProvider>
    );
  }
  it('shows the token prominently with the warning and gates actions on "Я сохранил токен"', async () => {
    const user = userEvent.setup();
    render(<Harness />);
    expect(screen.getByTestId('anonymous-token')).toHaveTextContent(entry.token);
    expect(screen.getByText(/восстановить токен невозможно/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Готово' })).toBeDisabled();
    await user.click(screen.getByLabelText('Я сохранил токен'));
    expect(screen.getByRole('button', { name: 'Готово' })).toBeEnabled();
  });
});

describe('theme title validation (CR3)', () => {
  it('mirrors the server rule 2..140 after trimming', () => {
    expect(validateThemeTitle('')).toBeTruthy();
    expect(validateThemeTitle(' А ')).toBeTruthy();
    expect(validateThemeTitle('Ок')).toBeUndefined();
    expect(validateThemeTitle('x'.repeat(141))).toBeTruthy();
  });
});
