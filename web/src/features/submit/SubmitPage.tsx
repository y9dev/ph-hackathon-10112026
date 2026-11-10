import { useEffect, useState } from 'react';
import { Controller, useForm, useWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from 'react-router';
import { TriangleAlert, Lightbulb, Send, Eraser, Check, CircleAlert, ThumbsUp } from 'lucide-react';
import { api, ApiError } from '@/shared/api/client';
import { qk, useMeta } from '@/shared/api/queries';
import type { Meta, Priority } from '@/shared/api/types';
import { formatDateTime } from '@/shared/lib/format';
import { useDebounced } from '@/shared/lib/useDebounced';
import { ru } from '@/shared/i18n/ru';
import {
  Button,
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
import { CategoryCombobox } from './CategoryCombobox';
import { rememberCreated } from './createdStore';
import { saveAnonymousRequest } from '@/features/anonymous/deviceStore';

export default function SubmitPage() {
  useDocumentTitle(ru.submit.title);
  const meta = useMeta();
  return (
    <div>
      <PageHeader title={ru.submit.title} lead={ru.submit.lead} />
      {meta.isPending ? (
        <LoadingBlock>
          <div className="grid grid-cols-1 gap-8 lg:grid-cols-[minmax(0,1fr)_360px]">
            <div className="flex flex-col gap-6">
              <Skeleton className="h-12" />
              <Skeleton className="h-11" />
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
    const validType = d.type === 'problem' || d.type === 'idea' || d.type === 'feedback';
    const validPrio = d.priority === 'low' || d.priority === 'medium' || d.priority === 'high';
    return {
      ...EMPTY_VALUES,
      ...d,
      type: validType ? d.type! : EMPTY_VALUES.type,
      priority: validPrio ? d.priority! : EMPTY_VALUES.priority,
      category_id: validCat ? d.category_id! : EMPTY_VALUES.category_id,
    };
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
      toast.success(ru.submit.toastCreated(r.number));
      if (r.anonymous_token) {
        // CR2: the token is returned once. Keep it in memory for the success screen and on this device;
        // never in the query cache, the URL or history state. The session cannot read this request (404).
        saveAnonymousRequest({ id: r.id, number: r.number, title: r.title, token: r.anonymous_token, created_at: r.created_at });
        rememberCreated(r);
      } else {
        qc.setQueryData(qk.request(r.id), r);
      }
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
          priority: 'priority',
        };
        let first: keyof SubmitValues | null = null;
        for (const [k, msg] of Object.entries(e.fields)) {
          if (k === 'files') setFileError(msg);
          else if (map[k]) {
            setError(map[k], { message: msg });
            first ??= map[k];
          }
        }
        if (first === 'category_id') document.getElementById('category-input')?.focus();
        else if (first && first !== 'type' && first !== 'is_anonymous' && first !== 'priority') setFocus(first);
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
    if (firstKey === 'category_id') document.getElementById('category-input')?.focus();
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
  const type = values.type ?? 'problem';
  const dirty = Boolean(values.title || values.body || values.proposed_solution || values.category_id || files.length);

  return (
    <form noValidate onSubmit={handleSubmit(onSubmit, onInvalid)} className="grid grid-cols-1 items-start gap-8 lg:grid-cols-[minmax(0,1fr)_360px]">
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
                  wrap
                  aria-label={ru.submit.type}
                  value={field.value}
                  onChange={field.onChange}
                  options={[
                    { value: 'problem', label: ru.types.problem, icon: <TriangleAlert className="h-4 w-4 shrink-0 max-lg:hidden" aria-hidden="true" /> },
                    { value: 'idea', label: ru.types.idea, icon: <Lightbulb className="h-4 w-4 shrink-0 max-lg:hidden" aria-hidden="true" /> },
                    {
                      value: 'feedback',
                      label: (
                        <>
                          <span className="sm:hidden">{ru.typesShort.feedback}</span>
                          <span className="max-sm:hidden">{ru.types.feedback}</span>
                        </>
                      ),
                      ariaLabel: ru.types.feedback,
                      icon: <ThumbsUp className="h-4 w-4 shrink-0 max-lg:hidden" aria-hidden="true" />,
                    },
                  ]}
                />
                <p className="text-sm text-muted">{ru.typeHints[field.value]}</p>
              </div>
            )}
          />
        </FormField>

        <FormField
          id="category-input"
          label={ru.submit.category}
          hint={ru.submit.categoryHint}
          error={formState.errors.category_id?.message}
          aside={<span className="tabular text-xs text-muted">{ru.submit.categoryCount(categories.length)}</span>}
        >
          <Controller
            control={control}
            name="category_id"
            render={({ field }) => (
              <>
                <CategoryCombobox
                  categories={categories}
                  value={field.value}
                  onChange={(v) => {
                    field.onChange(v);
                    field.onBlur();
                  }}
                  onBlur={field.onBlur}
                />
                {/* Mobile: the full "what happens next" panel is far below, so show the routing right under the choice. */}
                {selectedCategory && preview.data && (
                  <p className="text-sm text-text lg:hidden" data-testid="routing-compact">
                    {preview.data.routed && preview.data.department
                      ? ru.submit.nextCompact(preview.data.department.name, formatDateTime(preview.data.first_response_due))
                      : ru.submit.nextCompactUnrouted}
                  </p>
                )}
              </>
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
          label={ru.submit.bodyByType[type]}
          hint={ru.submit.bodyHintByType[type]}
          error={formState.errors.body?.message}
          aside={<Counter value={bodyLen} max={limits.body_max} />}
        >
          <Textarea {...register('body')} rows={6} />
        </FormField>

        {type !== 'feedback' && (
          <FormField
            label={type === 'idea' ? ru.submit.solutionIdea : ru.submit.solution}
            hint={ru.submit.solutionHint}
            optional
            error={formState.errors.proposed_solution?.message}
            aside={solLen > 0 ? <Counter value={solLen} max={SOLUTION_MAX} /> : undefined}
          >
            <Textarea {...register('proposed_solution')} rows={3} />
          </FormField>
        )}

        <FormField label={ru.submit.priority} hint={ru.submit.priorityHint} optional asFieldset error={formState.errors.priority?.message}>
          <Controller
            control={control}
            name="priority"
            render={({ field }) => (
              <SegmentedControl<Priority>
                aria-label={ru.submit.priority}
                className="sm:max-w-md"
                value={field.value}
                onChange={field.onChange}
                options={(['low', 'medium', 'high'] as Priority[]).map((p) => ({ value: p, label: ru.priorities[p] }))}
              />
            )}
          />
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
