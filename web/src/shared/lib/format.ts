import { ru, plural } from '@/shared/i18n/ru';

const pad = (n: number) => String(n).padStart(2, '0');

/** 12.11 18:00 (same year) or 12.11.2025 18:00. */
export function formatDateTime(iso: string | null | undefined, now: Date = new Date()): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '—';
  const base = `${pad(d.getDate())}.${pad(d.getMonth() + 1)}`;
  const year = d.getFullYear() !== now.getFullYear() ? `.${d.getFullYear()}` : '';
  return `${base}${year} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

/** 12.11.2026 */
export function formatDate(iso: string | null | undefined): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '—';
  return `${pad(d.getDate())}.${pad(d.getMonth() + 1)}.${d.getFullYear()}`;
}

/** Short day label for charts: 12.11 */
export function formatDay(isoDate: string): string {
  const [, m, d] = isoDate.split('-');
  return `${d}.${m}`;
}

/** Hours → "6 ч", "1 дн. 4 ч", "3 дн." */
export function formatHours(hours: number): string {
  const h = Math.max(0, Math.round(hours));
  if (h < 1) return 'менее 1 ч';
  if (h < 24) return `${h} ч`;
  const days = Math.floor(h / 24);
  const rest = h % 24;
  if (days >= 3 || rest === 0) return `${days} дн.`;
  return `${days} дн. ${rest} ч`;
}

/** "5 минут назад", "вчера", or a date. */
export function formatRelative(iso: string, now: Date = new Date()): string {
  const d = new Date(iso);
  const diffMin = Math.round((now.getTime() - d.getTime()) / 60000);
  if (diffMin < 1) return 'только что';
  if (diffMin < 60) return `${diffMin} ${plural(diffMin, 'минуту', 'минуты', 'минут')} назад`;
  const diffH = Math.floor(diffMin / 60);
  if (diffH < 24) return `${diffH} ${plural(diffH, 'час', 'часа', 'часов')} назад`;
  const diffD = Math.floor(diffH / 24);
  if (diffD === 1) return 'вчера';
  if (diffD < 7) return `${diffD} ${plural(diffD, 'день', 'дня', 'дней')} назад`;
  return formatDate(iso);
}

export function formatPercent(share: number | null | undefined): string {
  if (share === null || share === undefined || Number.isNaN(share)) return '—';
  return `${Math.round(share * 100)}%`;
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} Б`;
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} КБ`;
  return `${(bytes / (1024 * 1024)).toFixed(1).replace('.', ',')} МБ`;
}

export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return '?';
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[1][0]).toUpperCase();
}

export { ru };
