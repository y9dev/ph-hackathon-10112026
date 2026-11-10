import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { ToastProvider } from '@/shared/ui';
import type { RequestDetail } from '@/shared/api/types';
import { Composer, defaultKind } from './Composer';

function renderComposer(kinds: RequestDetail['allowed_comment_kinds']) {
  const r = { id: 1, allowed_comment_kinds: kinds } as RequestDetail;
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <ToastProvider>
        <Composer request={r} />
      </ToastProvider>
    </QueryClientProvider>,
  );
}

describe('Composer kinds', () => {
  it('offers "Отчёт" only when report is allowed', () => {
    renderComposer(['comment', 'report', 'internal']);
    expect(screen.getByRole('radio', { name: /Отчёт/ })).toBeInTheDocument();
  });
  it('does not offer a report option otherwise', () => {
    renderComposer(['comment', 'internal']);
    expect(screen.queryByRole('radio', { name: /Отчёт/ })).toBeNull();
  });
  it('keeps comment as the default kind', () => {
    expect(defaultKind(['report', 'comment', 'internal'])).toBe('comment');
  });
  it('preselects the report kind during implementation when it is allowed', () => {
    expect(defaultKind(['comment', 'report', 'internal'], 'in_progress')).toBe('report');
    expect(defaultKind(['comment', 'internal'], 'in_progress')).toBe('comment');
    expect(defaultKind(['comment', 'report', 'internal'], 'decision')).toBe('comment');
  });
  it('renders the "Отчёт" tab as selected for an in_progress request', () => {
    const r = { id: 1, status: 'in_progress', allowed_comment_kinds: ['comment', 'report', 'internal'] } as RequestDetail;
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ToastProvider>
          <Composer request={r} />
        </ToastProvider>
      </QueryClientProvider>,
    );
    expect(screen.getByRole('radio', { name: /Отчёт/ })).toHaveAttribute('aria-checked', 'true');
  });
  it('selects the requested kind and focuses the field', () => {
    const r = { id: 1, status: 'in_review', allowed_comment_kinds: ['comment', 'question', 'internal'] } as RequestDetail;
    const qc = new QueryClient();
    const ui = (n: number) => (
      <QueryClientProvider client={qc}>
        <ToastProvider>
          <Composer request={r} kindRequest={n ? { kind: 'question', n } : undefined} />
        </ToastProvider>
      </QueryClientProvider>
    );
    const { rerender } = render(ui(0));
    expect(screen.getByRole('radio', { name: /Комментарий/ })).toHaveAttribute('aria-checked', 'true');
    rerender(ui(1));
    expect(screen.getByRole('radio', { name: /Вопрос/ })).toHaveAttribute('aria-checked', 'true');
    expect(screen.getByLabelText('Текст сообщения')).toHaveFocus();
  });
});
