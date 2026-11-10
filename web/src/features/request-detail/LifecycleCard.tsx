import { ClipboardList } from 'lucide-react';
import type { RequestDetail, TimelineItem } from '@/shared/api/types';
import { formatDateTime } from '@/shared/lib/format';
import { ru } from '@/shared/i18n/ru';
import { Card, StageStepper } from '@/shared/ui';

export function latestReport(timeline: TimelineItem[]): TimelineItem | undefined {
  for (let i = timeline.length - 1; i >= 0; i--) if (timeline[i]!.kind === 'report') return timeline[i];
  return undefined;
}

/** Stage stepper + plain-language "what next" + latest progress report. Shown to authors and staff. */
export function LifecycleCard({ r, showHint }: { r: RequestDetail; showHint: boolean }) {
  const report = latestReport(r.timeline);
  const showReport = !!report || r.status === 'in_progress';
  return (
    <Card>
      <div className="px-6 py-5 max-sm:px-4">
        <StageStepper status={r.status} />
        {showHint && (
          <p className="mt-4 text-sm text-text" data-testid="what-next">
            <span className="font-semibold">{ru.lifecycle.whatNextTitle}: </span>
            {ru.lifecycle.whatNext[r.status]}
          </p>
        )}
        {showReport && (
          <div className="mt-4 flex gap-3 rounded-lg border border-border bg-surface-2 px-3 py-2" data-testid="latest-report">
            <ClipboardList className="mt-0.5 h-4 w-4 shrink-0 text-muted" aria-hidden="true" />
            <div className="min-w-0 text-sm">
              {report ? (
                <>
                  <p className="font-medium text-text">
                    {ru.lifecycle.latestReport}
                    <span className="font-normal text-muted">
                      {' · '}
                      <time dateTime={report.at} className="tabular">
                        {formatDateTime(report.at)}
                      </time>
                    </span>
                  </p>
                  <p className="mt-1 line-clamp-3 whitespace-pre-line break-words text-text">{report.text}</p>
                </>
              ) : (
                <p className="text-muted">{ru.lifecycle.noReportYet}</p>
              )}
            </div>
          </div>
        )}
      </div>
    </Card>
  );
}
