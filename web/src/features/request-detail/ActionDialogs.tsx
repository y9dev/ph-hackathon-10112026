// Dialogs are mounted only while open (parent renders them conditionally),
// so their local state starts fresh every time without reset effects.
import { useId, useRef, useState, type FormEvent } from 'react';
import { Plus, Search, Tags } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { api, ApiError } from '@/shared/api/client';
import { qk, useMeta } from '@/shared/api/queries';
import type { RequestDetail, Status } from '@/shared/api/types';
import { actionLabel, parseRequestNumber, statusLabel } from '@/shared/lib/status';
import { ru } from '@/shared/i18n/ru';
import { cn } from '@/shared/lib/cn';
import {
  Button,
  Counter,
  Dialog,
  ErrorState,
  FieldError,
  FormField,
  Input,
  SegmentedControl,
  Select,
  Skeleton,
  StatusChip,
  Textarea,
  useToast,
} from '@/shared/ui';
import { useRequestMutations } from './useRequestMutations';
import { COMMENT_MAX } from './Composer';

export const RESULT_MAX = 4000;

/** Close (implemented/declined/duplicate) with required result text. */
export function CloseDialog({
  request,
  status,
  open,
  onOpenChange,
}: {
  request: RequestDetail;
  status: Status;
  open: boolean;
  onOpenChange: (o: boolean) => void;
}) {
  const [text, setText] = useState('');
  const [dupInput, setDupInput] = useState('');
  const [errors, setErrors] = useState<{ result?: string; dup?: string; form?: string }>({});
  const [resolving, setResolving] = useState(false);
  const resultRef = useRef<HTMLTextAreaElement>(null);
  const dupRef = useRef<HTMLInputElement>(null);
  const m = useRequestMutations(request.id).status;
  const toast = useToast();
  const isDup = status === 'duplicate';

  async function resolveDuplicateId(): Promise<number | null> {
    const num = parseRequestNumber(dupInput);
    if (!dupInput.trim()) {
      setErrors((e) => ({ ...e, dup: ru.detail.duplicateRequired }));
      return null;
    }
    if (!num) {
      setErrors((e) => ({ ...e, dup: ru.detail.duplicateInvalid }));
      return null;
    }
    if (num === request.number) {
      setErrors((e) => ({ ...e, dup: ru.detail.duplicateSelf }));
      return null;
    }
    const similar = request.similar.items?.find((s) => s.number === num);
    if (similar) return similar.id;
    setResolving(true);
    try {
      const res = await api.listRequests({ scope: 'all', tab: 'all', q: num, page_size: 5 });
      const hit = res.items.find((x) => x.number === num);
      if (!hit) {
        setErrors((e) => ({ ...e, dup: `${num}: ${ru.errors.requestNotFoundTitle.toLowerCase()}` }));
        return null;
      }
      return hit.id;
    } catch (err) {
      setErrors((e) => ({ ...e, form: err instanceof Error ? err.message : ru.errors.generic }));
      return null;
    } finally {
      setResolving(false);
    }
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (m.isPending || resolving) return;
    const result = text.trim();
    const next: typeof errors = {};
    if (!result) next.result = ru.detail.resultRequired;
    else if (result.length > RESULT_MAX) next.result = ru.detail.resultMax(RESULT_MAX);
    setErrors(next);
    let dupId: number | undefined;
    if (isDup) {
      const id = await resolveDuplicateId();
      if (id === null) {
        dupRef.current?.focus();
        return;
      }
      dupId = id;
    }
    if (next.result) {
      resultRef.current?.focus();
      return;
    }
    m.mutate(
      { status, result_text: result, duplicate_of_id: dupId },
      {
        onSuccess: () => {
          toast.success(ru.detail.statusChanged(statusLabel(status)));
          onOpenChange(false);
        },
        onError: (err) => {
          if (err instanceof ApiError && err.status === 400) {
            setErrors({ result: err.fields.result_text, dup: err.fields.duplicate_of_id, form: err.message });
          } else setErrors({ form: err.message });
        },
      },
    );
  }

  const similar = request.similar.items ?? [];
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      busy={m.isPending}
      title={ru.detail.closeTitles[status] ?? statusLabel(status)}
      description={ru.detail.closeLead}
      footer={
        <>
          <Button onClick={() => onOpenChange(false)} disabled={m.isPending}>
            {ru.common.cancel}
          </Button>
          <Button type="submit" form="close-form" variant={status === 'declined' ? 'danger' : 'primary'} loading={m.isPending || resolving}>
            {isDup ? ru.detail.closeTitles.duplicate : ru.statusActions[status]}
          </Button>
        </>
      }
    >
      <form id="close-form" onSubmit={submit} noValidate className="flex flex-col gap-4">
        {isDup && (
          <FormField label={ru.detail.duplicateLabel} required hint={ru.detail.duplicateHint} error={errors.dup}>
            <Input ref={dupRef} value={dupInput} onChange={(e) => setDupInput(e.target.value)} autoComplete="off" autoFocus inputMode="text" />
          </FormField>
        )}
        {isDup && similar.length > 0 && (
          <div className="flex flex-col gap-2">
            <p className="text-xs font-medium text-muted">{ru.detail.similar}</p>
            <ul className="flex flex-col gap-1">
              {similar.slice(0, 3).map((s) => (
                <li key={s.id}>
                  <button
                    type="button"
                    onClick={() => {
                      setDupInput(s.number);
                      setErrors((e) => ({ ...e, dup: undefined }));
                    }}
                    className="flex w-full items-center gap-3 rounded-lg border border-border px-3 py-2 text-left text-sm hover:border-border-strong hover:bg-surface-2"
                  >
                    <span className="tabular shrink-0 text-xs font-medium text-muted">{s.number}</span>
                    <span className="min-w-0 flex-1 truncate text-text">{s.title}</span>
                    <StatusChip status={s.status} />
                  </button>
                </li>
              ))}
            </ul>
          </div>
        )}
        <FormField
          label={ru.detail.resultLabel}
          required
          hint={ru.detail.closeHints[status]}
          error={errors.result}
          aside={<Counter value={text.length} max={RESULT_MAX} />}
        >
          <Textarea ref={resultRef} value={text} onChange={(e) => setText(e.target.value)} rows={5} autoFocus={!isDup} />
        </FormField>
        {errors.form && !errors.result && !errors.dup && (
          <p role="alert" className="text-sm text-danger">
            {errors.form}
          </p>
        )}
      </form>
    </Dialog>
  );
}

/**
 * Stage move (take into work, pass to decision, start implementation, return...): explains what will happen
 * and lets the staff member add an optional one-line message that is posted as a normal comment after the status change.
 */
export function MoveDialog({
  request,
  to,
  open,
  onOpenChange,
}: {
  request: RequestDetail;
  to: Status;
  open: boolean;
  onOpenChange: (o: boolean) => void;
}) {
  const [message, setMessage] = useState('');
  const [error, setError] = useState<string>();
  const [busy, setBusy] = useState(false);
  const { status, comment } = useRequestMutations(request.id);
  const toast = useToast();
  const label = actionLabel(request.status, to);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (busy) return;
    const body = message.trim();
    if (body.length > COMMENT_MAX) return setError(ru.detail.composerMax(COMMENT_MAX));
    setBusy(true);
    setError(undefined);
    try {
      await status.mutateAsync({ status: to });
    } catch (err) {
      setError(err instanceof Error ? err.message : ru.errors.generic);
      setBusy(false);
      return;
    }
    toast.success(ru.detail.statusChanged(statusLabel(to)));
    if (body) {
      try {
        await comment.mutateAsync({ kind: 'comment', body });
      } catch {
        toast.error(ru.detail.moveMessageFailed);
      }
    }
    setBusy(false);
    onOpenChange(false);
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      busy={busy}
      size="sm"
      title={label}
      description={ru.detail.moveLead(statusLabel(to))}
      footer={
        <>
          <Button onClick={() => onOpenChange(false)} disabled={busy}>
            {ru.common.cancel}
          </Button>
          <Button type="submit" form="move-form" variant="primary" loading={busy}>
            {label}
          </Button>
        </>
      }
    >
      <form id="move-form" onSubmit={submit} noValidate>
        <FormField label={ru.detail.moveMessageLabel} hint={ru.detail.moveMessageHint} optional error={error}>
          <Input value={message} onChange={(e) => setMessage(e.target.value)} autoComplete="off" autoFocus />
        </FormField>
      </form>
    </Dialog>
  );
}

const NONE = '__none';

export function AssignDialog({ request, open, onOpenChange }: { request: RequestDetail; open: boolean; onOpenChange: (o: boolean) => void }) {
  const meta = useMeta();
  const [dept, setDept] = useState<string>(() => (request.department ? String(request.department.id) : ''));
  const [assignee, setAssignee] = useState<string>(() => (request.assignee ? String(request.assignee.id) : NONE));
  const [error, setError] = useState<string>();
  const m = useRequestMutations(request.id).assign;
  const toast = useToast();

  const people = (meta.data?.assignable_users ?? []).filter((u) => !dept || u.department_id === Number(dept));

  function submit(e: FormEvent) {
    e.preventDefault();
    if (m.isPending) return;
    if (!dept) return setError(ru.detail.assignDeptRequired);
    m.mutate(
      { department_id: Number(dept), assignee_id: assignee === NONE ? null : Number(assignee) },
      {
        onSuccess: () => {
          toast.success(ru.detail.assigned);
          onOpenChange(false);
        },
        onError: (err) => setError(err.message),
      },
    );
  }
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      busy={m.isPending}
      title={ru.detail.assignTitle}
      description={ru.detail.assignLead}
      footer={
        <>
          <Button onClick={() => onOpenChange(false)} disabled={m.isPending}>
            {ru.common.cancel}
          </Button>
          <Button type="submit" form="assign-form" variant="primary" loading={m.isPending}>
            {ru.detail.assignSubmit}
          </Button>
        </>
      }
    >
      <form id="assign-form" onSubmit={submit} noValidate className="flex flex-col gap-4">
        <FormField label={ru.detail.department}>
          <Select
            value={dept}
            onValueChange={(v) => {
              setDept(v);
              const stillValid = (meta.data?.assignable_users ?? []).some((u) => String(u.id) === assignee && u.department_id === Number(v));
              if (!stillValid) setAssignee(NONE);
            }}
            placeholder={ru.inbox.department}
            options={(meta.data?.departments ?? []).map((d) => ({ value: String(d.id), label: d.name }))}
          />
        </FormField>
        <FormField label={ru.detail.assignee} hint={people.length === 0 ? ru.common.none : undefined}>
          <Select
            value={assignee}
            onValueChange={setAssignee}
            options={[{ value: NONE, label: ru.common.none }, ...people.map((u) => ({ value: String(u.id), label: u.name }))]}
          />
        </FormField>
        {error && (
          <p role="alert" className="text-sm text-danger">
            {error}
          </p>
        )}
      </form>
    </Dialog>
  );
}

export const THEME_TITLE_MIN = 2;
export const THEME_TITLE_MAX = 140;
type ThemeMode = 'new' | 'existing';

/** Client copy of the server rule (title 2..140 after trimming). */
export function validateThemeTitle(title: string): string | undefined {
  const t = title.trim();
  if (t.length < THEME_TITLE_MIN || t.length > THEME_TITLE_MAX) return ru.detail.themeTitleMin;
  return undefined;
}

/**
 * Add a request to a theme (CR3 redesign). Two explicit options instead of one select that mixed
 * "new" and "existing": «Новая тема» (title field only) and «Добавить в существующую» (searchable list,
 * no portal dropdown, so nothing is clipped on mobile and Esc only closes the dialog). The dialog is
 * mounted only while open, so every opening starts from the request's current state.
 */
export function ThemeDialog({ request, open, onOpenChange }: { request: RequestDetail; open: boolean; onOpenChange: (o: boolean) => void }) {
  const themes = useQuery({ queryKey: qk.themes, queryFn: api.themes, enabled: open });
  const [mode, setMode] = useState<ThemeMode>(request.theme ? 'existing' : 'new');
  const [picked, setPicked] = useState<number | null>(request.theme?.id ?? null);
  const [title, setTitle] = useState('');
  const [search, setSearch] = useState('');
  const [errors, setErrors] = useState<{ title?: string; pick?: string; form?: string }>({});
  const titleRef = useRef<HTMLInputElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const m = useRequestMutations(request.id).theme;
  const toast = useToast();
  const searchId = `theme-search${useId().replace(/:/g, '')}`;

  const all = themes.data?.items ?? [];
  const q = search.trim().toLowerCase().replace(/ё/g, 'е');
  const shown = q ? all.filter((t) => t.title.toLowerCase().replace(/ё/g, 'е').includes(q)) : all;
  const alreadyIn = mode === 'existing' && picked !== null && picked === request.theme?.id;

  // Autofocus only on opening; after switching the mode focus stays on the switch (arrow keys keep working).
  const [switched, setSwitched] = useState(false);
  function switchMode(next: ThemeMode) {
    setMode(next);
    setSwitched(true);
    setErrors({});
  }

  function submit(e: FormEvent) {
    e.preventDefault();
    if (m.isPending) return;
    if (mode === 'new') {
      const err = validateThemeTitle(title);
      if (err) {
        setErrors({ title: err });
        titleRef.current?.focus();
        return;
      }
    } else if (picked === null) {
      setErrors({ pick: ru.detail.themePick });
      listRef.current?.querySelector<HTMLElement>('input[type="radio"]')?.focus();
      if (!all.length) searchRef.current?.focus();
      return;
    } else if (alreadyIn) {
      onOpenChange(false);
      return;
    }
    setErrors({});
    const body = mode === 'new' ? { title: title.trim() } : { theme_id: picked! };
    m.mutate(body, {
      onSuccess: () => {
        toast.success(ru.detail.themeLinked);
        onOpenChange(false);
      },
      onError: (err) => {
        if (err instanceof ApiError && err.fields.title) {
          setErrors({ title: err.fields.title });
          titleRef.current?.focus();
        } else if (err instanceof ApiError && err.status === 404 && mode === 'existing') {
          setErrors({ pick: ru.detail.themeUnavailable });
        } else setErrors({ form: err.message });
      },
    });
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      busy={m.isPending}
      title={ru.detail.themeTitle}
      description={ru.detail.themeLead}
      footer={
        <>
          <Button onClick={() => onOpenChange(false)} disabled={m.isPending}>
            {ru.common.cancel}
          </Button>
          <Button type="submit" form="theme-form" variant="primary" loading={m.isPending}>
            {ru.detail.themeSubmit}
          </Button>
        </>
      }
    >
      <form id="theme-form" onSubmit={submit} noValidate className="flex flex-col gap-4">
        <FormField label={ru.detail.themeMode} asFieldset>
          <SegmentedControl<ThemeMode>
            wrap
            aria-label={ru.detail.themeMode}
            value={mode}
            onChange={switchMode}
            options={[
              { value: 'new', label: ru.detail.themeModeNew, icon: <Plus className="h-4 w-4 shrink-0 max-sm:hidden" aria-hidden="true" /> },
              { value: 'existing', label: ru.detail.themeModeExisting, icon: <Tags className="h-4 w-4 shrink-0 max-sm:hidden" aria-hidden="true" /> },
            ]}
          />
        </FormField>

        {mode === 'new' ? (
          <FormField
            label={ru.detail.themeNewLabel}
            hint={ru.detail.themeNewHint}
            error={errors.title}
            required
            aside={title.length > THEME_TITLE_MAX * 0.8 ? <Counter value={title.length} max={THEME_TITLE_MAX} /> : undefined}
          >
            <Input
              ref={titleRef}
              value={title}
              onChange={(e) => {
                setTitle(e.target.value);
                if (errors.title) setErrors({});
              }}
              placeholder={ru.detail.themeNewPlaceholder}
              maxLength={THEME_TITLE_MAX + 20}
              autoComplete="off"
              autoFocus={!switched}
            />
          </FormField>
        ) : (
          <div className="flex flex-col gap-2">
            <label htmlFor={searchId} className="text-sm font-medium text-text">
              {ru.detail.themeSearch}
            </label>
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" aria-hidden="true" />
              <Input
                id={searchId}
                ref={searchRef}
                type="search"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                className="pl-9"
                autoComplete="off"
                autoFocus={!switched}
                disabled={themes.isPending}
              />
            </div>
            <div
              ref={listRef}
              role="radiogroup"
              aria-label={ru.detail.themeList}
              aria-describedby={errors.pick ? 'theme-pick-err' : undefined}
              aria-busy={themes.isPending || undefined}
              className={cn(
                'max-h-[min(280px,40dvh)] overflow-y-auto overscroll-contain rounded-lg border p-1',
                errors.pick ? 'border-danger' : 'border-border',
              )}
            >
              {themes.isPending ? (
                <div className="flex flex-col gap-1 p-1" aria-hidden="true">
                  {[0, 1, 2].map((i) => (
                    <Skeleton key={i} className="h-11" />
                  ))}
                </div>
              ) : themes.isError ? (
                <ErrorState compact error={themes.error} onRetry={() => themes.refetch()} />
              ) : shown.length === 0 ? (
                <p className="px-3 py-4 text-sm text-muted">{all.length === 0 ? ru.detail.themeListEmpty : ru.detail.themeListNoMatch}</p>
              ) : (
                shown.map((t) => {
                  const id = `theme-opt-${t.id}`;
                  const checked = picked === t.id;
                  return (
                    <label
                      key={t.id}
                      htmlFor={id}
                      className={cn(
                        'flex min-h-11 cursor-pointer items-start gap-3 rounded-md px-3 py-2 transition-colors duration-150 hover:bg-surface-2',
                        checked && 'bg-brand-soft hover:bg-brand-soft',
                      )}
                    >
                      <input
                        id={id}
                        type="radio"
                        name="theme"
                        value={t.id}
                        checked={checked}
                        onChange={() => {
                          setPicked(t.id);
                          setErrors({});
                        }}
                        className="mt-1 h-4 w-4 shrink-0 accent-[var(--brand)]"
                      />
                      <span className="min-w-0 flex-1">
                        <span className="block text-sm font-medium text-text [overflow-wrap:anywhere]">
                          {t.title}
                          {t.id === request.theme?.id && <span className="ml-2 text-xs font-normal text-muted">({ru.detail.themeCurrent})</span>}
                        </span>
                        <span className="block text-xs text-muted">
                          {ru.common.requests(t.count)}
                          {t.categories.length > 0 && ` · ${t.categories.join(', ')}`}
                        </span>
                      </span>
                    </label>
                  );
                })
              )}
            </div>
            {errors.pick && <FieldError id="theme-pick-err">{errors.pick}</FieldError>}
            {alreadyIn && !errors.pick && <p className="text-sm text-muted">{ru.detail.themeAlready}</p>}
          </div>
        )}

        {errors.form && (
          <p role="alert" className="text-sm text-danger">
            {errors.form}
          </p>
        )}
      </form>
    </Dialog>
  );
}
