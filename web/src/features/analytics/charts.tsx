import { useId, useMemo, useState, type ReactNode } from 'react';
import { Link } from 'react-router';
import { cn } from '@/shared/lib/cn';
import { formatDay } from '@/shared/lib/format';
import { ru } from '@/shared/i18n/ru';

export interface BarDatum {
  key: string;
  label: ReactNode;
  value: number;
  /** CSS colour (token) for the bar. */
  color?: string;
  href?: string;
  /** Secondary text on the right, e.g. "РѕС‚РєСЂС‹С‚Рѕ 3". */
  meta?: ReactNode;
  icon?: ReactNode;
}

/** Horizontal bar list: labels stay readable, values are always printed (no colour-only meaning). */
export function BarList({ data, max: maxProp }: { data: BarDatum[]; max?: number }) {
  const max = Math.max(1, maxProp ?? Math.max(...data.map((d) => d.value), 0));
  return (
    <ul className="flex flex-col gap-3">
      {data.map((d) => {
        const pct = (d.value / max) * 100;
        const inner = (
          <>
            <span className="flex items-baseline justify-between gap-3">
              <span className="flex min-w-0 items-center gap-2 text-sm text-text">
                {d.icon}
                <span className="truncate">{d.label}</span>
              </span>
              <span className="flex shrink-0 items-baseline gap-2">
                {d.meta && <span className="text-xs text-muted">{d.meta}</span>}
                <span className="tabular text-sm font-semibold text-text">{d.value}</span>
              </span>
            </span>
            <span className="mt-1.5 block h-2 overflow-hidden rounded-full bg-chart-track" aria-hidden="true">
              <span
                className="block h-full rounded-full transition-[width] duration-200 ease-out"
                style={{ width: `${Math.max(pct, d.value > 0 ? 2 : 0)}%`, background: d.color ?? 'var(--chart-1)' }}
              />
            </span>
          </>
        );
        return (
          <li key={d.key}>
            {d.href ? (
              <Link to={d.href} className="block rounded-lg p-1 -m-1 transition-colors duration-150 hover:bg-surface-2">
                {inner}
              </Link>
            ) : (
              inner
            )}
          </li>
        );
      })}
    </ul>
  );
}

/** Stacked single bar with a legend; each segment also listed as text. */
export function StackedBar({ data }: { data: BarDatum[] }) {
  const total = data.reduce((s, d) => s + d.value, 0);
  return (
    <div>
      <div className="flex h-3 w-full overflow-hidden rounded-full bg-chart-track" aria-hidden="true">
        {data
          .filter((d) => d.value > 0)
          .map((d) => (
            <span key={d.key} style={{ width: `${(d.value / Math.max(1, total)) * 100}%`, background: d.color }} className="h-full border-r-2 border-surface last:border-r-0" />
          ))}
      </div>
      <ul className="mt-4 grid grid-cols-1 gap-x-6 gap-y-2 sm:grid-cols-2">
        {data.map((d) => {
          const content = (
            <>
              <span className="flex min-w-0 items-center gap-2">
                <span className="h-2.5 w-2.5 shrink-0 rounded-full" style={{ background: d.color }} aria-hidden="true" />
                {d.icon}
                <span className="truncate text-text">{d.label}</span>
              </span>
              <span className="tabular shrink-0 text-muted">
                <span className="font-semibold text-text">{d.value}</span>
                {total > 0 && <span className="ml-1 text-xs">{Math.round((d.value / total) * 100)}%</span>}
              </span>
            </>
          );
          return (
            <li key={d.key}>
              {d.href ? (
                <Link to={d.href} className="-mx-2 flex items-center justify-between gap-3 rounded-md px-2 py-1 text-sm transition-colors duration-150 hover:bg-surface-2">
                  {content}
                </Link>
              ) : (
                <span className="flex items-center justify-between gap-3 py-1 text-sm">{content}</span>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}

export interface SeriesPoint {
  date: string;
  created: number;
  closed: number;
}

/** Two-series line chart (created vs closed) with hover/focus readout and an accessible data table. */
export function TrendChart({ data }: { data: SeriesPoint[] }) {
  // Small viewBox so axis text (12 units) stays legible when the card is ~400 px wide.
  const W = 440;
  const H = 180;
  const pad = { l: 28, r: 12, t: 10, b: 26 };
  const [hover, setHover] = useState<number | null>(null);
  const titleId = useId();
  const max = Math.max(2, ...data.map((d) => Math.max(d.created, d.closed)));
  const niceMax = Math.ceil(max / 2) * 2;
  const x = (i: number) => pad.l + (data.length <= 1 ? 0 : (i / (data.length - 1)) * (W - pad.l - pad.r));
  const y = (v: number) => pad.t + (1 - v / niceMax) * (H - pad.t - pad.b);
  const path = (key: 'created' | 'closed') => data.map((d, i) => `${i ? 'L' : 'M'}${x(i).toFixed(1)},${y(d[key]).toFixed(1)}`).join(' ');
  const ticks = [0, niceMax / 2, niceMax];
  const labelEvery = Math.max(1, Math.ceil(data.length / 7));
  const area = useMemo(
    () => (data.length ? `${path('created')} L${x(data.length - 1)},${y(0)} L${x(0)},${y(0)} Z` : ''),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [data],
  );

  return (
    <figure>
      <div className="mb-3 flex flex-wrap items-center gap-4 text-sm">
        <Legend color="var(--chart-1)" label={ru.analytics.trendCreated} />
        <Legend color="var(--chart-2)" label={ru.analytics.trendClosed} dashed />
        {hover !== null && data[hover] && (
          <span className="tabular ml-auto text-xs text-muted" aria-live="polite">
            {formatDay(data[hover].date)}: {ru.analytics.trendCreated.toLowerCase()} {data[hover].created}, {ru.analytics.trendClosed.toLowerCase()} {data[hover].closed}
          </span>
        )}
      </div>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        className="h-auto w-full touch-pan-y"
        role="img"
        aria-labelledby={titleId}
        onMouseLeave={() => setHover(null)}
        onMouseMove={(e) => {
          const rect = (e.currentTarget as SVGSVGElement).getBoundingClientRect();
          const px = ((e.clientX - rect.left) / rect.width) * W;
          const i = Math.round(((px - pad.l) / (W - pad.l - pad.r)) * (data.length - 1));
          setHover(Math.min(data.length - 1, Math.max(0, i)));
        }}
      >
        <title id={titleId}>{ru.analytics.trend}</title>
        {ticks.map((t) => (
          <g key={t}>
            <line x1={pad.l} x2={W - pad.r} y1={y(t)} y2={y(t)} stroke="var(--border)" strokeDasharray={t === 0 ? undefined : '3 4'} />
            <text x={pad.l - 8} y={y(t) + 4} textAnchor="end" fontSize="12" fill="var(--text-muted)" className="tabular">
              {t}
            </text>
          </g>
        ))}
        {data.map((d, i) =>
          i % labelEvery === 0 || i === data.length - 1 ? (
            <text key={d.date} x={x(i)} y={H - 8} textAnchor="middle" fontSize="12" fill="var(--text-muted)" className="tabular">
              {formatDay(d.date)}
            </text>
          ) : null,
        )}
        <path d={area} fill="var(--chart-1)" opacity="0.08" />
        <path d={path('created')} fill="none" stroke="var(--chart-1)" strokeWidth="2.25" strokeLinejoin="round" strokeLinecap="round" />
        <path d={path('closed')} fill="none" stroke="var(--chart-2)" strokeWidth="2.25" strokeDasharray="6 4" strokeLinejoin="round" strokeLinecap="round" />
        {hover !== null && data[hover] && (
          <g>
            <line x1={x(hover)} x2={x(hover)} y1={pad.t} y2={H - pad.b} stroke="var(--border-strong)" />
            <circle cx={x(hover)} cy={y(data[hover].created)} r="4" fill="var(--surface)" stroke="var(--chart-1)" strokeWidth="2" />
            <circle cx={x(hover)} cy={y(data[hover].closed)} r="4" fill="var(--surface)" stroke="var(--chart-2)" strokeWidth="2" />
          </g>
        )}
      </svg>
      <details className="mt-2 text-sm">
        <summary className="inline-flex min-h-9 items-center text-muted hover:text-text">{ru.analytics.chartTable}</summary>
        <div className="mt-2 max-h-56 overflow-auto rounded-lg border border-border">
          <table className="w-full text-left text-sm">
            <thead className="bg-surface-2 text-xs text-muted">
              <tr>
                <th scope="col" className="px-3 py-2 font-medium">{ru.analytics.date}</th>
                <th scope="col" className="px-3 py-2 font-medium">{ru.analytics.trendCreated}</th>
                <th scope="col" className="px-3 py-2 font-medium">{ru.analytics.trendClosed}</th>
              </tr>
            </thead>
            <tbody>
              {data.map((d) => (
                <tr key={d.date} className="border-t border-border">
                  <td className="tabular px-3 py-1.5">{formatDay(d.date)}</td>
                  <td className="tabular px-3 py-1.5">{d.created}</td>
                  <td className="tabular px-3 py-1.5">{d.closed}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </details>
    </figure>
  );
}

function Legend({ color, label, dashed }: { color: string; label: string; dashed?: boolean }) {
  return (
    <span className="inline-flex items-center gap-2 text-muted">
      <svg width="20" height="8" aria-hidden="true">
        <line x1="0" x2="20" y1="4" y2="4" stroke={color} strokeWidth="2.5" strokeDasharray={dashed ? '5 3' : undefined} />
      </svg>
      <span className={cn('text-text')}>{label}</span>
    </span>
  );
}
