import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Dialog } from './Dialog';

/** Mirrors how request-detail mounts dialogs: a parent flag, no Radix Trigger. */
function Host({ vanish = false }: { vanish?: boolean }) {
  const [open, setOpen] = useState(false);
  const [gone, setGone] = useState(false);
  return (
    <main id="main" tabIndex={-1}>
      {!gone && <button onClick={() => setOpen(true)}>Открыть</button>}
      {open && (
        <Dialog
          open
          onOpenChange={(o) => !o && setOpen(false)}
          title="Заголовок"
          footer={
            <>
              <button onClick={() => setOpen(false)}>Отмена</button>
              <button
                onClick={() => {
                  if (vanish) setGone(true);
                  setOpen(false);
                }}
              >
                Готово
              </button>
            </>
          }
        >
          <input aria-label="Поле" />
        </Dialog>
      )}
    </main>
  );
}

describe('Dialog focus management', () => {
  it('is announced as modal', async () => {
    render(<Host />);
    await userEvent.click(screen.getByRole('button', { name: 'Открыть' }));
    expect(await screen.findByRole('dialog')).toHaveAttribute('aria-modal', 'true');
  });
  it('returns focus to the opener on Escape', async () => {
    render(<Host />);
    const opener = screen.getByRole('button', { name: 'Открыть' });
    await userEvent.click(opener);
    await screen.findByRole('dialog');
    await userEvent.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    await waitFor(() => expect(opener).toHaveFocus());
  });
  it('returns focus to the opener on Cancel', async () => {
    render(<Host />);
    const opener = screen.getByRole('button', { name: 'Открыть' });
    await userEvent.click(opener);
    await userEvent.click(await screen.findByRole('button', { name: 'Отмена' }));
    await waitFor(() => expect(opener).toHaveFocus());
  });
  it('falls back to <main> when the opener is gone after success', async () => {
    render(<Host vanish />);
    await userEvent.click(screen.getByRole('button', { name: 'Открыть' }));
    await userEvent.click(await screen.findByRole('button', { name: 'Готово' }));
    await waitFor(() => expect(document.getElementById('main')).toHaveFocus());
  });
});
