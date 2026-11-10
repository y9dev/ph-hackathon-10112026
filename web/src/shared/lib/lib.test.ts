import { describe, expect, it } from 'vitest';
import { plural } from '@/shared/i18n/ru';
import { formatBytes, formatDateTime, formatHours, formatPercent, initials } from './format';
import { slaView, waitingDays } from './sla';
import { parseRequestNumber, sortActions, statusLabel, needsResult, isTerminal } from './status';
import { DEFAULT_FILTERS, filtersFromSearch, filtersToParams, filtersToSearch, hasActiveFilters, patchFilters } from './inboxFilters';
import type { SlaState } from '@/shared/api/types';

describe('format', () => {
  it('pluralizes Russian nouns', () => {
    expect(plural(1, 'день', 'дня', 'дней')).toBe('день');
    expect(plural(3, 'день', 'дня', 'дней')).toBe('дня');
    expect(plural(5, 'день', 'дня', 'дней')).toBe('дней');
    expect(plural(11, 'день', 'дня', 'дней')).toBe('дней');
    expect(plural(21, 'день', 'дня', 'дней')).toBe('день');
    expect(plural(112, 'день', 'дня', 'дней')).toBe('дней');
  });
  it('formats hour durations compactly', () => {
    expect(formatHours(0.2)).toBe('менее 1 ч');
    expect(formatHours(6)).toBe('6 ч');
    expect(formatHours(24)).toBe('1 дн.');
    expect(formatHours(30)).toBe('1 дн. 6 ч');
    expect(formatHours(80)).toBe('3 дн.');
  });
  it('formats date-time with year only when different', () => {
    const now = new Date(2026, 10, 10, 12, 0);
    expect(formatDateTime(new Date(2026, 10, 12, 18, 5).toISOString(), now)).toBe('12.11 18:05');
    expect(formatDateTime(new Date(2025, 0, 2, 9, 0).toISOString(), now)).toBe('02.01.2025 09:00');
    expect(formatDateTime(null)).toBe('—');
  });
  it('formats percent, bytes and initials', () => {
    expect(formatPercent(0.876)).toBe('88%');
    expect(formatPercent(null)).toBe('—');
    expect(formatBytes(512)).toBe('512 Б');
    expect(formatBytes(2048)).toBe('2 КБ');
    expect(formatBytes(1.5 * 1024 * 1024)).toBe('1,5 МБ');
    expect(initials('Тимур Ахметов')).toBe('ТА');
  });
});

describe('sla', () => {
  const base: SlaState = {
    first_response_due: new Date(2026, 10, 12, 18, 0).toISOString(),
    resolution_due: new Date(2026, 10, 15, 18, 0).toISOString(),
    first_responded_at: null,
    state: 'ok',
    overdue_kind: null,
    overdue_hours: 0,
  };
  const now = new Date(2026, 10, 10, 12, 0);
  it('ok shows the first-response deadline in grey', () => {
    expect(slaView(base, 'new', now)).toMatchObject({ tone: 'neutral', label: 'до 12.11 18:00' });
  });
  it('switches to the resolution deadline after the first response', () => {
    const v = slaView({ ...base, first_responded_at: now.toISOString() }, 'in_review', now);
    expect(v.label).toBe('до 15.11 18:00');
  });
  it('overdue is danger with text, never colour only', () => {
    const v = slaView({ ...base, state: 'overdue', overdue_kind: 'first_response', overdue_hours: 6 }, 'new', now);
    expect(v.tone).toBe('danger');
    expect(v.label).toBe('Просрочено на 6 ч');
    expect(v.description).toContain('нет первого ответа');
  });
  it('soon is a warning, paused waits for the author', () => {
    expect(slaView({ ...base, state: 'soon' }, 'new', now).tone).toBe('warning');
    expect(slaView({ ...base, state: 'paused' }, 'needs_info', now)).toMatchObject({ tone: 'info', label: 'Ждём автора' });
  });
  it('counts days waiting for the author', () => {
    expect(waitingDays(new Date(2026, 10, 7, 11, 0).toISOString(), now)).toBe(3);
  });
});

describe('status', () => {
  it('labels statuses in Russian', () => {
    expect(statusLabel('needs_info')).toBe('Нужно уточнение');
    expect(statusLabel('implemented')).toBe('Реализовано');
  });
  it('knows closing statuses need a result', () => {
    expect(needsResult('declined')).toBe(true);
    expect(needsResult('accepted')).toBe(false);
    expect(isTerminal('duplicate')).toBe(true);
  });
  it('orders actions progress-first, closing last', () => {
    expect(sortActions(['duplicate', 'declined', 'in_review', 'accepted', 'needs_info'])).toEqual([
      'in_review',
      'needs_info',
      'accepted',
      'declined',
      'duplicate',
    ]);
  });
  it('parses request numbers in several spellings', () => {
    expect(parseRequestNumber('MV-0012')).toBe('MV-0012');
    expect(parseRequestNumber('mv 12')).toBe('MV-0012');
    expect(parseRequestNumber('12')).toBe('MV-0012');
    expect(parseRequestNumber('abc')).toBeNull();
  });
});

describe('inbox filters <-> URL', () => {
  it('defaults when the URL is empty', () => {
    expect(filtersFromSearch(new URLSearchParams())).toEqual(DEFAULT_FILTERS);
  });
  it('round-trips all filters', () => {
    const f = { tab: 'closed' as const, status: 'declined' as const, category_id: 3, department_id: 2, assignee: 'me' as const, overdue: true, q: 'VPN', sort: 'newest' as const, page: 2 };
    expect(filtersFromSearch(filtersToSearch(f))).toEqual(f);
  });
  it('omits defaults from the URL', () => {
    expect(filtersToSearch(DEFAULT_FILTERS).toString()).toBe('');
  });
  it('ignores malformed values', () => {
    const f = filtersFromSearch(new URLSearchParams('tab=nope&status=bad&category=-1&page=0&sort=x&assignee=all'));
    expect(f).toEqual(DEFAULT_FILTERS);
  });
  it('resets page when another filter changes', () => {
    const f = { ...DEFAULT_FILTERS, page: 4 };
    expect(patchFilters(f, { q: 'x' }).page).toBe(1);
    expect(patchFilters(f, { page: 5 }).page).toBe(5);
  });
  it('maps to API params with page size', () => {
    expect(filtersToParams({ ...DEFAULT_FILTERS, overdue: true }, 'inbox')).toMatchObject({ scope: 'inbox', tab: 'needs_action', overdue: true, page_size: 20 });
    expect(hasActiveFilters(DEFAULT_FILTERS)).toBe(false);
    expect(hasActiveFilters({ ...DEFAULT_FILTERS, q: 'a' })).toBe(true);
  });
});
