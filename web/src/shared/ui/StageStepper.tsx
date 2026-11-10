import { Check, CircleCheckBig, CircleX, Copy, Archive, MessageCircleQuestion } from 'lucide-react';
import type { Status } from '@/shared/api/types';
import { cn } from '@/shared/lib/cn';
import { STAGES, stageOf, type Stage } from '@/shared/lib/status';
import { ru } from '@/shared/i18n/ru';

type StepState = 'done' | 'current' | 'upcoming' | 'skipped';

/** Per-stage state derived from status only (no history is needed). */
export function stageStates(status: Status): Record<Stage, StepState> {
  const cur = stageOf(status);
  if (status === 'implemented') return { development: 'done', decision: 'done', implementation: 'done' };
  if (!cur) return { development: 'skipped', decision: 'skipped', implementation: 'skipped' };
  const idx = STAGES.indexOf(cur);
  const out = {} as Record<Stage, StepState>;
  STAGES.forEach((s, i) => (out[s] = i < idx ? 'done' : i === idx ? 'current' : 'upcoming'));
  return out;
}

const STATE_TEXT: Record<StepState, string> = {
  done: ru.lifecycle.stateDone,
  current: ru.lifecycle.stateCurrent,
  upcoming: ru.lifecycle.stateUpcoming,
  skipped: ru.lifecycle.stateSkipped,
};

const FINAL_ICON = { implemented: CircleCheckBig, declined: CircleX, duplicate: Copy, closed: Archive } as const;

function finalOf(status: Status): 'implemented' | 'declined' | 'duplicate' | 'closed' | null {
  return status === 'implemented' || status === 'declined' || status === 'duplicate' || status === 'closed' ? status : null;
}

/**
 * Full stepper: Разработка -> Принятие решения -> Реализация (+ final state when closed).
 * An ordered list; the current step has aria-current="step"; every step has a text label, never colour only.
 */
export function StageStepper({ status, className }: { status: Status; className?: string }) {
  const states = stageStates(status);
  const final = finalOf(status);
  const waiting = status === 'needs_info';
  return (
    <ol aria-label={ru.lifecycle.stepperLabel} className={cn('flex flex-col gap-3 md:flex-row md:items-start md:gap-0', className)}>
      {STAGES.map((stage, i) => {
        const st = states[stage];
        const isLast = i === STAGES.length - 1 && !final;
        return (
          <li
            key={stage}
            aria-current={st === 'current' ? 'step' : undefined}
            data-stage={stage}
            data-state={st}
            className="relative flex min-w-0 items-start gap-3 md:flex-1 md:flex-col md:gap-2"
          >
            <div className="flex items-center md:w-full">
              <span
                className={cn(
                  'flex h-8 w-8 shrink-0 items-center justify-center rounded-full border text-sm font-semibold',
                  st === 'done' && 'border-success bg-success text-[var(--on-brand)]',
                  st === 'current' && 'border-brand bg-brand text-[var(--on-brand)] ring-4 ring-brand-soft',
                  st === 'upcoming' && 'border-border-strong bg-surface text-muted',
                  st === 'skipped' && 'border-border bg-surface-2 text-muted',
                )}
                aria-hidden="true"
              >
                {st === 'done' ? <Check className="h-4 w-4" /> : i + 1}
              </span>
              {!isLast && (
                <span
                  className={cn('ml-2 hidden h-0.5 flex-1 rounded-full md:block', st === 'done' ? 'bg-success' : 'bg-border')}
                  aria-hidden="true"
                />
              )}
            </div>
            <div className="min-w-0 md:pr-3">
              <p className={cn('text-sm font-semibold', st === 'current' ? 'text-text' : st === 'upcoming' || st === 'skipped' ? 'text-muted' : 'text-text')}>
                {ru.lifecycle.stages[stage]}
              </p>
              <p className="text-xs text-muted">
                {st === 'skipped' ? (
                  <>
                    <span aria-hidden="true">{STATE_TEXT.skipped}</span>
                    <span className="sr-only">{ru.lifecycle.stateSkippedSr}</span>
                  </>
                ) : (
                  STATE_TEXT[st]
                )}
              </p>
              {stage === 'development' && waiting && (
                <p className="mt-1 inline-flex items-center gap-1 rounded-full border border-warning/40 bg-warning-soft px-2 py-0.5 text-xs font-medium text-warning">
                  <MessageCircleQuestion className="h-3.5 w-3.5" aria-hidden="true" />
                  {ru.lifecycle.waitingAuthor}
                </p>
              )}
            </div>
          </li>
        );
      })}
      {final && (
        <li aria-current="step" data-stage="final" data-state="current" className="relative flex min-w-0 items-start gap-3 md:flex-col md:gap-2">
          <span
            className={cn(
              'flex h-8 w-8 shrink-0 items-center justify-center rounded-full border',
              final === 'implemented' ? 'border-success bg-success-soft text-success' : 'border-border-strong bg-surface-2 text-text',
            )}
            aria-hidden="true"
          >
            {(() => {
              const Icon = FINAL_ICON[final];
              return <Icon className="h-4 w-4" />;
            })()}
          </span>
          <div className="min-w-0">
            <p className="text-sm font-semibold text-text">{ru.lifecycle.final[final]}</p>
          </div>
        </li>
      )}
    </ol>
  );
}

/** Compact form for list rows/cards: three segments + the current stage name. */
export function StageStepperCompact({ status, className }: { status: Status; className?: string }) {
  const states = stageStates(status);
  const cur = stageOf(status);
  const final = finalOf(status);
  const n = cur ? STAGES.indexOf(cur) + 1 : 0;
  const text = final ? ru.lifecycle.final[final] : ru.lifecycle.stages[cur as Stage];
  return (
    <div className={cn('flex items-center gap-2', className)} data-testid="stage-compact">
      <ol aria-label={ru.lifecycle.stepperLabel} className="flex items-center gap-1">
        {STAGES.map((stage) => {
          const st = states[stage];
          return (
            <li
              key={stage}
              aria-current={st === 'current' ? 'step' : undefined}
              className={cn(
                'h-1.5 w-6 rounded-full',
                st === 'done' && 'bg-success',
                st === 'current' && 'bg-brand',
                st === 'upcoming' && 'bg-border-strong',
                st === 'skipped' && 'bg-border',
              )}
            >
              <span className="sr-only">
                {ru.lifecycle.stages[stage]}: {st === 'skipped' ? ru.lifecycle.stateSkippedSr : STATE_TEXT[st]}
              </span>
            </li>
          );
        })}
      </ol>
      <span className="text-xs font-medium text-muted">
        {final ? text : ru.lifecycle.compact(text, STAGES.length, n)}
      </span>
    </div>
  );
}
