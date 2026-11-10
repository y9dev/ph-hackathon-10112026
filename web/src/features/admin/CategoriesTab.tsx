import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Controller, useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Plus, Pencil, Folder } from 'lucide-react';
import { api, ApiError } from '@/shared/api/client';
import { qk } from '@/shared/api/queries';
import type { Category } from '@/shared/api/types';
import { ru } from '@/shared/i18n/ru';
import {
  Button,
  CATEGORY_ICON_NAMES,
  CategoryIcon,
  Dialog,
  EmptyState,
  ErrorState,
  FormField,
  Input,
  Select,
  Switch,
  Textarea,
  useToast,
} from '@/shared/ui';
import { ActiveBadge, FormAlert, TableShell, TableSkeleton, Td, Th } from './common';
import { categorySchema, type CategoryValues } from './schemas';

export function CategoriesTab() {
  const cats = useQuery({ queryKey: [...qk.admin, 'categories'], queryFn: api.adminCategories });
  const [editing, setEditing] = useState<Category | 'new' | null>(null);
  return (
    <section aria-labelledby="cats-h">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2 id="cats-h" className="text-xl font-semibold text-text">
          {ru.admin.categories}
        </h2>
        <Button variant="primary" onClick={() => setEditing('new')} icon={<Plus className="h-4 w-4" aria-hidden="true" />}>
          {ru.admin.addCategory}
        </Button>
      </div>
      {cats.isPending ? (
        <TableSkeleton />
      ) : cats.isError ? (
        <ErrorState error={cats.error} onRetry={() => cats.refetch()} />
      ) : cats.data.items.length === 0 ? (
        <div className="rounded-xl border border-border bg-surface">
          <EmptyState icon={<Folder className="h-6 w-6" />} title={ru.admin.catEmpty} />
        </div>
      ) : (
        <TableShell
          caption={ru.admin.categories}
          head={
            <>
              <Th className="w-20">{ru.admin.catSort}</Th>
              <Th>{ru.admin.catName}</Th>
              <Th>{ru.admin.catDescription}</Th>
              <Th>{ru.admin.status}</Th>
              <Th className="w-20 text-right">
                <span className="sr-only">{ru.admin.actions}</span>
              </Th>
            </>
          }
        >
          {[...cats.data.items]
            .sort((a, b) => a.sort - b.sort)
            .map((c) => (
              <tr key={c.id} className={c.active ? '' : 'text-muted'}>
                <Td className="tabular text-muted">{c.sort}</Td>
                <Td>
                  <span className="flex items-center gap-2 font-medium text-text">
                    <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-surface-2 text-muted">
                      <CategoryIcon name={c.icon} className="h-4 w-4" />
                    </span>
                    {c.name}
                  </span>
                </Td>
                <Td className="max-w-80 text-muted">
                  <span className="line-clamp-2">{c.description}</span>
                </Td>
                <Td>
                  <ActiveBadge active={c.active} />
                </Td>
                <Td className="text-right">
                  <Button size="icon-sm" variant="ghost" aria-label={ru.admin.rowActions(c.name)} onClick={() => setEditing(c)}>
                    <Pencil className="h-4 w-4" aria-hidden="true" />
                  </Button>
                </Td>
              </tr>
            ))}
        </TableShell>
      )}
      {editing && <CategoryDialog cat={editing} onClose={() => setEditing(null)} nextSort={(cats.data?.items.length ?? 0) * 10 + 10} />}
    </section>
  );
}

function CategoryDialog({ cat, onClose, nextSort }: { cat: Category | 'new' | null; onClose: () => void; nextSort: number }) {
  const qc = useQueryClient();
  const toast = useToast();
  const open = cat !== null;
  const [formError, setFormError] = useState<string | null>(null);
  const form = useForm<CategoryValues>({
    resolver: zodResolver(categorySchema),
    defaultValues:
      cat && cat !== 'new'
        ? { name: cat.name, description: cat.description, icon: cat.icon || 'folder', sort: cat.sort, active: cat.active }
        : { name: '', description: '', icon: 'folder', sort: nextSort, active: true },
  });

  const save = useMutation({
    mutationFn: (v: CategoryValues) => (cat && cat !== 'new' ? api.updateCategory(cat.id, v) : api.createCategory(v)),
    onSuccess: () => {
      toast.success(ru.admin.catSaved);
      qc.invalidateQueries({ queryKey: qk.admin });
      qc.invalidateQueries({ queryKey: qk.meta });
      onClose();
    },
    onError: (e) => {
      if (e instanceof ApiError && (e.status === 400 || e.status === 409) && Object.keys(e.fields).length) {
        for (const [k, msg] of Object.entries(e.fields)) form.setError(k as keyof CategoryValues, { message: msg });
      } else setFormError(e.message);
    },
  });
  const err = form.formState.errors;
  return (
    <Dialog
      open={open}
      onOpenChange={(o) => !o && onClose()}
      busy={save.isPending}
      title={cat === 'new' ? ru.admin.newCategory : ru.admin.editCategory}
      footer={
        <>
          <Button onClick={onClose} disabled={save.isPending}>
            {ru.common.cancel}
          </Button>
          <Button type="submit" form="cat-form" variant="primary" loading={save.isPending} loadingText={ru.common.saving}>
            {ru.common.save}
          </Button>
        </>
      }
    >
      <form id="cat-form" noValidate onSubmit={form.handleSubmit((v) => save.mutate(v))} className="flex flex-col gap-4">
        <FormAlert message={formError} />
        <FormField label={ru.admin.catName} error={err.name?.message}>
          <Input {...form.register('name')} autoComplete="off" />
        </FormField>
        <FormField label={ru.admin.catDescription} hint={ru.admin.catDescriptionHint} error={err.description?.message}>
          <Textarea rows={2} {...form.register('description')} />
        </FormField>
        <div className="grid gap-4 sm:grid-cols-2">
          <FormField label={ru.admin.catIcon}>
            <Controller
              control={form.control}
              name="icon"
              render={({ field }) => (
                <Select
                  value={field.value || 'folder'}
                  onValueChange={field.onChange}
                  options={CATEGORY_ICON_NAMES.map((n) => ({
                    value: n,
                    label: (
                      <span className="inline-flex items-center gap-2">
                        <CategoryIcon name={n} className="h-4 w-4" />
                        {n}
                      </span>
                    ),
                  }))}
                />
              )}
            />
          </FormField>
          <FormField label={ru.admin.catSort} error={err.sort?.message}>
            <Input type="number" inputMode="numeric" step={1} {...form.register('sort', { valueAsNumber: true })} />
          </FormField>
        </div>
        <Controller
          control={form.control}
          name="active"
          render={({ field }) => <Switch checked={Boolean(field.value)} onCheckedChange={field.onChange} label={ru.admin.catActive} />}
        />
      </form>
    </Dialog>
  );
}
