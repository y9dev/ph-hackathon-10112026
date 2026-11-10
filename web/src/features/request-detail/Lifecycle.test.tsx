import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import type { RequestDetail, TimelineItem } from '@/shared/api/types';
import { LifecycleCard, latestReport } from './LifecycleCard';
import { Timeline } from './Timeline';

const staff = { name: 'Анна', anonymous: false, is_me: false, role: 'Руководитель' };
const report = (id: string, text: string, at: string): TimelineItem => ({ id, at, actor: staff, kind: 'report', text });

function req(status: RequestDetail['status'], timeline: TimelineItem[]): RequestDetail {
  return { status, timeline } as RequestDetail;
}

describe('latestReport', () => {
  it('returns the newest report', () => {
    const t = [report('a', 'первый', '2026-11-01T10:00:00Z'), report('b', 'второй', '2026-11-02T10:00:00Z')];
    expect(latestReport(t)?.text).toBe('второй');
    expect(latestReport([])).toBeUndefined();
  });
});

describe('LifecycleCard', () => {
  it('shows the latest report and the plain-language hint for the author', () => {
    render(<LifecycleCard r={req('in_progress', [report('b', 'Ночное окно настроено', '2026-11-02T10:00:00Z')])} showHint />);
    expect(screen.getByTestId('latest-report')).toHaveTextContent('Последний отчёт');
    expect(screen.getByTestId('latest-report')).toHaveTextContent('Ночное окно настроено');
    expect(screen.getByTestId('what-next')).toHaveTextContent('идёт реализация');
  });
  it('says there are no reports yet during implementation and hides the hint for staff', () => {
    render(<LifecycleCard r={req('in_progress', [])} showHint={false} />);
    expect(screen.getByText('Отчётов о ходе пока нет')).toBeInTheDocument();
    expect(screen.queryByTestId('what-next')).toBeNull();
  });
  it('does not show a report block before implementation when there are no reports', () => {
    render(<LifecycleCard r={req('in_review', [])} showHint />);
    expect(screen.queryByTestId('latest-report')).toBeNull();
  });
});

describe('Timeline report entry', () => {
  it('renders a progress report as a labelled card with author role', () => {
    render(
      <MemoryRouter>
        <Timeline items={[report('r1', 'Закупили оборудование', '2026-11-02T10:00:00Z')]} />
      </MemoryRouter>,
    );
    expect(screen.getAllByText('Отчёт о ходе').length).toBeGreaterThan(0);
    expect(screen.getByText('Закупили оборудование')).toBeInTheDocument();
    expect(screen.getByText('Руководитель')).toBeInTheDocument();
  });
});
