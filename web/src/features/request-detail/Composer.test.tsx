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
});
