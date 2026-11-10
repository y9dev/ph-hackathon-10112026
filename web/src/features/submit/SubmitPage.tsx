import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { Controller, useForm, useWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router';
import { TriangleAlert, Lightbulb, Send, Eraser, Check, CircleAlert } from 'lucide-react';
import { api, ApiError } from '@/shared/api/client';
import { qk, useMeta } from '@/shared/api/queries';
import type { Category, Meta } from '@/shared/api/types';
import { cn } from '@/shared/lib/cn';
import { useDebounced } from '@/shared/lib/useDebounced';
import { ru } from '@/shared/i18n/ru';
import {
  Button,
  CategoryIcon,
  Counter,
  ErrorState,
  FormField,
  Input,
  LoadingBlock,
  PageHeader,
  SegmentedControl,
  Skeleton,
  Switch,
  Textarea,
  useDocumentTitle,
  useToast,
} from '@/shared/ui';
import { useUser } from '@/features/auth/AuthProvider';
import { EMPTY_VALUES, makeSubmitSchema, SOLUTION_MAX, toPayload, type SubmitValues } from './schema';
import { FileDrop } from './FileDrop';
import { NextStepsPanel } from './NextStepsPanel';
import { clearDraft, loadDraft, saveDraft } from './draft';

export default function SubmitPage() {
  useDocumentTitle(ru.submit.title);
  const meta = useMeta();
  return (
    <div>
      <PageHeader title={ru.submit.title} lead={ru.submit.lead} />
      {meta.isPending ? (
        <LoadingBlock>
          <div className="grid gap-8 lg:grid-cols-[minmax(0,1fr)_360px]">
            <div className="flex flex-col gap-6">
              <Skeleton className="h-12" />
              <div className="grid grid-cols-2 gap-3 md:grid-cols-3">
                {[0, 1, 2, 3, 4, 5].map((i) => (
                  <Skeleton key={i} className="h-24" />
                ))}
              </div>
              <Skeleton className="h-10" />
              <Skeleton className="h-36" />
            </div>
            <Skeleton className="h-64 max-lg:hidden" />
          </div>
        </LoadingBlock>
      ) : meta.isError ? (
        <ErrorState error={meta.error} onRetry={() => meta.refetch()} />
      ) : (
        <SubmitForm meta={meta.data} />
      )}
    </div>
  );
}

function SubmitForm({ meta }: { meta: Meta }) {
  const user = useUser();
  const toast = useToast();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const limits = meta.limits;
  const categories = [...meta.categories].filter((c) => c.active).sort((a, b) => a.sort - b.sort);
  // Draft is read once on mount and used as the form's initial values.
  const [initial] = useState<SubmitValues>(() => {
    const d = loadDraft(user.id);
    if (!d) return EMPTY_VALUES;
    const validCat = meta.categories.some((c) => c.active && c.id === d.category_id);
    return { ...EMPTY_VALUES, ...d, category_id: validCat ? d.category_id! : EMPTY_VALUES.category_id };
  });
  const [restored, setRestored] = useState(initial !== EMPTY_VALUES);
  const [files, setFiles] = useState<File[]>([]);
  const [formError, setFormError] = useState<string | null>(null);
  const [fileError, setFileError] = useState<string | undefined>();

  const form = useForm<SubmitValues>({
    resolver: zodResolver(makeSubmitSchema(limits)),
    defaultValues: initial,
    mode: 'onTouched',
  });
  const { register, control, handleSubmit, formState, setError, reset, setFocus } = form;
  const values = useWatch({ control });

  // Autosave draft (debounced) to localStorage.
  const debounced = useDebounced(values, 500);
  useEffect(() => {
    saveDraft(user.id, debounced as Partial<SubmitValues>);
  }, [debounced, user.id]);

  const previewKey = useDebounced({ category_id: values.category_id, type: values.type }, 250);
  const preview = useQuery({
    queryKey: ['routing-preview', previewKey],
    queryFn: () => api.routingPreview(previewKey.category_id!, previewKey.type!),
    enabled: Boolean(previewKey.category_id && previewKey.type),
    staleTime: 60_000,
  });

  const create = useMutation({
    mutationFn: (v: SubmitValues) => api.createRequest(toPayload(v), files),
    onSuccess: (r) => {
      clearDraft(user.id);
      qc.invalidateQueries({ queryKey: qk.requests });
      qc.setQueryData(qk.request(r.id), r);
      toast.success(ru.submit.toastCreated(r.number));
      navigate(`/new/done/${r.id}`, { replace: false });
    },
    onError: (e) => {
      if (e instanceof ApiError && e.status === 400 && Object.keys(e.fields).length) {
        const map: Record<string, keyof SubmitValues> = {
          type: 'type',
          category_id: 'category_id',
          title: 'title',
          body: 'body',
          proposed_solution: 'proposed_solution',
        };
        let first: keyof SubmitValues | null = null;
        for (const [k, msg] of Object.entries(e.fields)) {
          if (k === 'files') setFileError(msg);
          else if (map[k]) {
            setError(map[k], { message: msg });
            first ??= map[k];
          }
        }
        if (first && first !== 'type' && first !== 'category_id' && first !== 'is_anonymous') setFocus(first);
        setFormError(ru.errors.fixFields);
      } else {
        setFormError(e instanceof Error ? e.message : ru.errors.generic);
      }
    },
  });

  function onSubmit(v: SubmitValues) {
    setFormError(null);
    setFileError(undefined);
    create.mutate(v);
  }

  function onInvalid() {
    setFormError(ru.errors.fixFields);
    // Focus the first invalid field in visual order.
    const order: (keyof SubmitValues)[] = ['type', 'category_id', 'title', 'body', 'proposed_solution'];
    const firstKey = order.find((k) => form.formState.errors[k]);
    if (firstKey === 'category_id') document.getElementById('category-group')?.querySelector<HTMLElement>('[tabindex="0"]')?.focus();
    else if (firstKey && firstKey !== 'type') setFocus(firstKey);
  }

  function clearAll() {
    reset(EMPTY_VALUES);
    setFiles([]);
    clearDraft(user.id);
    setRestored(false);
    setFormError(null);
  }

  const selectedCategory = categories.find((c) => c.id === values.category_id);
  const titleLen = values.title?.length ?? 0;
  const bodyLen = values.body?.length ?? 0;
  const solLen = values.proposed_solution?.length ?? 0;
  const dirty = Boolean(values.title || values.body || values.proposed_solution || values.category_id || files.length);

  return (
    <form noValidate onSubmit={handleSubmit(onSubmit, onInvalid)} className="grid items-start gap-8 lg:grid-cols-[minmax(0,1fr)_360px]">
      <div className="flex min-w-0 flex-col gap-8">
        {restored && (
          <div role="status" className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border bg-surface px-4 py-3 text-sm text-text">
            <span className="flex items-center gap-2">
              <Check className="h-4 w-4 text-brand" aria-hidden="true" />
              {ru.submit.draftRestored}
            </span>
            <Button size="sm" variant="ghost" onClick={clearAll} icon={<Eraser className="h-4 w-4" aria-hidden="true" />}>
              {ru.submit.draftClear}
            </Button>
          </div>
        )}

        <FormField label={ru.submit.type} asFieldset error={formState.errors.type?.message}>
          <Controller
            control={control}
            name="type"
            render={({ field }) => (
              <div className="flex flex-col gap-2">
                <SegmentedControl
                  aria-label={ru.submit.type}
                  value={field.value}
                  onChange={field.onChange}
                  options={[
                    { value: 'problem', label: ru.types.problem, icon: <TriangleAlert className="h-4 w-4" aria-hidden="true" /> },
                    { value: 'idea', label: ru.types.idea, icon: <Lightbulb className="h-4 w-4" aria-hidden="true" /> },
                  ]}
                />
                <p className="text-sm text-muted">{ru.typeHints[field.value]}</p>
              </div>
            )}
          />
        </FormField>

        <FormField label={ru.submit.category} hint={ru.submit.categoryHint} asFieldset error={formState.errors.category_id?.message}>
          <Controller
            control={control}
            name="category_id"
            render={({ field, fieldState }) => (
              <CategoryPicker
                categories={categories}
                value={field.value}
                onChange={(v) => {
                  field.onChange(v);
                  field.onBlur();
                }}
                invalid={Boolean(fieldState.error)}
              />
            )}
          />
        </FormField>

        <FormField
          label={ru.submit.titleLabel}
          hint={ru.submit.titleHint}
          error={formState.errors.title?.message}
          aside={<Counter value={titleLen} max={limits.title_max} />}
        >
          <Input {...register('title')} maxLength={limits.title_max + 20} autoComplete="off" />
        </FormField>

        <FormField
          label={ru.submit.body}
          hint={ru.submit.bodyHint}
          error={formState.errors.body?.message}
          aside={<Counter value={bodyLen} max={limits.body_max} />}
        >
          <Textarea {...register('body')} rows={6} />
        </FormField>

        <FormField
          label={ru.submit.solution}
          hint={ru.submit.solutionHint}
          optional
          error={formState.errors.proposed_solution?.message}
          aside={solLen > 0 ? <Counter value={solLen} max={SOLUTION_MAX} /> : undefined}
        >
          <Textarea {...register('proposed_solution')} rows={3} />
        </FormField>

        <FileDrop
          files={files}
          onChange={(f) => {
            setFiles(f);
            setFileError(undefined);
          }}
          limits={limits}
          serverError={fileError}
          disabled={create.isPending}
        />

        <div className="rounded-xl border border-border bg-surface p-4 md:p-6">
          <Controller
            control={control}
            name="is_anonymous"
            render={({ field }) => (
              <Switch checked={field.value} onCheckedChange={field.onChange} label={ru.submit.anonymous} description={ru.submit.anonymousText} />
            )}
          />
        </div>

        {/* Mobile: next-steps summary above the sticky submit bar */}
        <div className="lg:hidden">
          <NextStepsPanel category={selectedCategory} type={values.type} anonymous={Boolean(values.is_anonymous)} preview={preview} />
        </div>

        <div className="sticky bottom-16 z-10 -mx-4 flex flex-col gap-3 border-t border-border bg-bg/95 px-4 py-3 backdrop-blur md:static md:mx-0 md:border-0 md:bg-transparent md:p-0 md:backdrop-blur-none">
          {formError && (
            <p role="alert" className="flex items-start gap-2 text-sm text-danger">
              <CircleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
              {formError}
            </p>
          )}
          <div className="flex flex-wrap items-center gap-3">
            <Button
              type="submit"
              variant="primary"
              size="lg"
              loading={create.isPending}
              loadingText={ru.submit.submitting}
              icon={<Send className="h-4 w-4" aria-hidden="true" />}
              className="max-md:w-full"
            >
              {ru.submit.submit}
            </Button>
            {dirty && !create.isPending && (
              <span className="text-xs text-muted max-md:hidden">{ru.submit.draftSaved}</span>
            )}
          </div>
        </div>
      </div>

      <aside className="sticky top-24 max-lg:hidden" aria-label={ru.submit.nextTitle}>
        <NextStepsPanel category={selectedCategory} type={values.type} anonymous={Boolean(values.is_anonymous)} preview={preview} />
      </aside>
    </form>
  );
}

function CategoryPicker({
  categories,
  value,
  onChange,
  invalid,
}: {
  categories: Category[];
  value: number | undefined;
  onChange: (id: number) => void;
  invalid: boolean;
}) {
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  const active = Math.max(
    0,
    categories.findIndex((c) => c.id === value),
  );
  function onKey(e: KeyboardEvent, i: number) {
    let n = -1;
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') n = (i + 1) % categories.length;
    if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') n = (i - 1 + categories.length) % categories.length;
    if (n >= 0) {
      e.preventDefault();
      onChange(categories[n].id);
      refs.current[n]?.focus();
    }
  }
  return (
    <div id="category-group" role="radiogroup" aria-label={ru.submit.category} aria-invalid={invalid || undefined} className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
      {categories.map((c, i) => {
        const selected = c.id === value;
        return (
          <button
            key={c.id}
            ref={(el) => {
              refs.current[i] = el;
            }}
            type="button"
            role="radio"
            aria-checked={selected}
            tabIndex={i === active ? 0 : -1}
            onClick={() => onChange(c.id)}
            onKeyDown={(e) => onKey(e, i)}
            className={cn(
              'group relative flex items-start gap-3 rounded-xl border bg-surface p-4 text-left transition-[border-color,background-color,box-shadow] duration-150',
              selected
                ? 'border-brand bg-brand-soft/60 ring-1 ring-brand'
                : invalid
                  ? 'border-danger/60 hover:border-danger'
                  : 'border-border hover:border-border-strong hover:shadow-sm',
            )}
          >
            <span
              className={cn(
                'flex h-9 w-9 shrink-0 items-center justify-center rounded-lg transition-colors duration-150',
                selected ? 'bg-brand text-on-brand' : 'bg-surface-2 text-muted group-hover:text-text',
              )}
              aria-hidden="true"
            >
              <CategoryIcon name={c.icon} className="h-5 w-5" />
            </span>
            <span className="min-w-0">
              <span className="block text-sm font-semibold text-text">{c.name}</span>
              <span className="mt-0.5 block text-xs leading-snug text-muted">{c.description}</span>
            </span>
            {selected && <Check className="absolute right-3 top-3 h-4 w-4 text-brand" aria-hidden="true" />}
          </button>
        );
      })}
    </div>
  );
}
