import { describe, expect, it } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { StageStepper, StageStepperCompact, stageStates } from './StageStepper';

describe('stageStates', () => {
  it('derives stage states from status', () => {
    expect(stageStates('new')).toEqual({ development: 'current', decision: 'upcoming', implementation: 'upcoming' });
    expect(stageStates('needs_info').development).toBe('current');
    expect(stageStates('decision')).toEqual({ development: 'done', decision: 'current', implementation: 'upcoming' });
    expect(stageStates('in_progress')).toEqual({ development: 'done', decision: 'done', implementation: 'current' });
    expect(stageStates('implemented')).toEqual({ development: 'done', decision: 'done', implementation: 'done' });
    expect(stageStates('declined').decision).toBe('skipped');
  });
});

describe('StageStepper', () => {
  it('renders an ordered list with the current step marked and text labels', () => {
    render(<StageStepper status="decision" />);
    const list = screen.getByRole('list', { name: 'Этапы обращения' });
    const items = within(list).getAllByRole('listitem');
    expect(items).toHaveLength(3);
    expect(items[0]).toHaveTextContent('Разработка');
    expect(items[1]).toHaveTextContent('Принятие решения');
    expect(items[2]).toHaveTextContent('Реализация');
    expect(items[1]).toHaveAttribute('aria-current', 'step');
    expect(items[0]).not.toHaveAttribute('aria-current');
    expect(items[1]).toHaveTextContent('текущий этап');
    expect(items[0]).toHaveTextContent('пройден');
  });

  it('shows "Ждём автора" on the development stage for needs_info only', () => {
    const { rerender } = render(<StageStepper status="needs_info" />);
    expect(screen.getByText('Ждём автора')).toBeInTheDocument();
    rerender(<StageStepper status="in_review" />);
    expect(screen.queryByText('Ждём автора')).toBeNull();
  });

  it('adds the final state for closed requests', () => {
    const { rerender } = render(<StageStepper status="implemented" />);
    expect(screen.getByText('Завершено')).toBeInTheDocument();
    rerender(<StageStepper status="declined" />);
    expect(screen.getByText('Отклонено')).toBeInTheDocument();
    rerender(<StageStepper status="duplicate" />);
    expect(screen.getByText('Дубликат')).toBeInTheDocument();
    rerender(<StageStepper status="closed" />);
    expect(screen.getByText('Закрыто')).toBeInTheDocument();
    expect(screen.getByRole('list', { name: 'Этапы обращения' }).querySelector('[data-stage="final"]')).toHaveAttribute('aria-current', 'step');
  });
});

describe('StageStepper terminal states', () => {
  it('shows the final item without "текущий этап" and keeps it as the current step', () => {
    render(<StageStepper status="closed" />);
    const final = screen.getByRole('list', { name: 'Этапы обращения' }).querySelector('[data-stage="final"]')!;
    expect(final).toHaveTextContent('Закрыто');
    expect(final).not.toHaveTextContent('текущий этап');
    expect(final).toHaveAttribute('aria-current', 'step');
  });
  it('shows earlier stages neutral ("—") for declined / duplicate / closed, never "не пройден"', () => {
    for (const s of ['declined', 'duplicate', 'closed'] as const) {
      const { container, unmount } = render(<StageStepper status={s} />);
      const stages = container.querySelectorAll('[data-stage]:not([data-stage="final"])');
      expect(stages).toHaveLength(3);
      stages.forEach((li) => {
        expect(li).toHaveAttribute('data-state', 'skipped');
        expect(li).toHaveTextContent('—');
        expect(li).not.toHaveTextContent('не пройден');
        expect(li).not.toHaveTextContent('текущий этап');
      });
      unmount();
    }
  });
  it('implemented keeps all three stages done', () => {
    const { container } = render(<StageStepper status="implemented" />);
    expect(container.querySelectorAll('[data-state="done"]')).toHaveLength(3);
  });
});

describe('StageStepperCompact', () => {
  it('names the current stage in text', () => {
    render(<StageStepperCompact status="in_progress" />);
    expect(screen.getByText('Этап 3 из 3: Реализация')).toBeInTheDocument();
  });
  it('shows the final label for closed requests', () => {
    render(<StageStepperCompact status="declined" />);
    expect(screen.getByText('Отклонено')).toBeInTheDocument();
  });
});
