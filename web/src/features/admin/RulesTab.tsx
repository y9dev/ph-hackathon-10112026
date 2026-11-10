import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Controller, useForm, useWatch } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Plus, Pencil, Trash2, Route, TriangleAlert } from 'lucide-react';
import { api, ApiError } from '@/shared/api/client';
import { qk } from '@/shared/api/queries';
import type { Category, RoutingRule } from '@/shared/api/types';
import { ru } from '@/shared/i18n/ru';
import { Button, Dialog, EmptyState, ErrorState, FormField, Input, Select, Switch, useToast } from '@/shared/ui';
import { ActiveBadge, FormAlert, TableShell, TableSkeleton, Td, Th } from './common';
import { ruleSchema, type RuleValues } from './schemas';

const ANY = '__any';
const HEAD = '__head';

/** Active categories that no active rule covers (a rule without a type covers both). A partly covered category names the missing type. */
export function categoriesWithoutRule(categories: Category[], rules: RoutingRule[]): string[] {
  const out: string[] = [];
  for (const c of categories.filter((x) => x.active)) {
    const mine = rules.filter((r) => r.active && r.category_id === c.id);
    if (mine.some((r) => r.request_type === null)) continue;
    const have = new Set(mine.map((r) => r.request_type));
    if (have.has('problem') && have.has('idea')) continue;
    if (have.size === 0) out.push(c.name);
    else out.push(`${c.name} (${ru.admin.noRuleType(ru.types[have.has('problem') ? 'idea' : 'problem'].toLowerCase())})`);
  }
  return out;
}

export function RulesTab() {
  const rules = useQuery({ queryKey: [...qk.admin, 'rules'], queryFn: api.adminRules });
  const cats = useQuery({ queryKey: [...qk.admin, 'categories'], queryFn: api.adminCategories });
  const uncovered = rules.data && cats.data ? categoriesWithoutRule(cats.data.items, rules.data.items) : [];
  const [editing, setEditing] = useState<RoutingRule | 'new' | null>(null);
  const [deleting, setDeleting] = useState<RoutingRule | null>(null);

  return (
    <section aria-labelledby="rules-h">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2 id="rules-h" className="text-xl font-semibold text-text">
          {ru.admin.routing}
        </h2>
        <Button variant="primary" onClick={() => setEditing('new')} icon={<Plus className="h-4 w-4" aria-hidden="true" />}>
          {ru.admin.addRule}
        </Button>
      </div>
      {uncovered.length > 0 && (
        <div className="mb-4 flex items-start gap-2 rounded-lg border border-warning/40 bg-warning-soft px-4 py-3 text-sm text-text" data-testid="no-rule-banner">
          <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0 text-warning" aria-hidden="true" />
          <p>{ru.admin.noRule(uncovered.join(', '))}</p>
        </div>
      )}
      {rules.isPending ? (
        <TableSkeleton />
      ) : rules.isError ? (
        <ErrorState error={rules.error} onRetry={() => rules.refetch()} />
      ) : rules.data.items.length === 0 ? (
        <div className="rounded-xl border border-border bg-surface">
          <EmptyState icon={<Route className="h-6 w-6" />} title={ru.admin.rulesEmpty} />
        </div>
      ) : (
        <TableShell
          caption={ru.admin.routing}
          head={
            <>
              <Th>{ru.admin.ruleCategory}</Th>
              <Th>{ru.admin.ruleType}</Th>
              <Th>{ru.admin.ruleDept}</Th>
              <Th>{ru.admin.ruleAssignee}</Th>
              <Th className="text-right">{ru.admin.ruleFirst}</Th>
              <Th className="text-right">{ru.admin.ruleResolution}</Th>
              <Th>{ru.admin.status}</Th>
              <Th className="w-28 text-right">
                <span className="sr-only">{ru.admin.actions}</span>
              </Th>
            </>
          }
        >
          {[...rules.data.items]
            .sort((a, b) => a.category_name.localeCompare(b.category_name, 'ru'))
            .map((r) => (
              <tr key={r.id} className={r.active ? '' : 'text-muted'}>
                <Td className="font-medium text-text">{r.category_name}</Td>
                <Td>{r.request_type ? ru.types[r.request_type] : ru.admin.ruleTypeAny}</Td>
                <Td>{r.department_name}</Td>
                <Td>{r.assignee_name ?? <span className="text-muted">{ru.admin.ruleAssigneeHead}</span>}</Td>
                <Td className="tabular whitespace-nowrap text-right">{r.first_response_hours}</Td>
                <Td className="tabular whitespace-nowrap text-right">{r.resolution_hours}</Td>
                <Td>
                  <ActiveBadge active={r.active} />
                </Td>
                <Td className="text-right">
                  <div className="flex justify-end gap-1">
                    <Button size="icon-sm" variant="ghost" aria-label={ru.admin.rowActions(r.category_name)} onClick={() => setEditing(r)}>
                      <Pencil className="h-4 w-4" aria-hidden="true" />
                    </Button>
                    <Button size="icon-sm" variant="ghost" aria-label={`${ru.common.delete}: ${r.category_name}`} onClick={() => setDeleting(r)}>
                      <Trash2 className="h-4 w-4" aria-hidden="true" />
                    </Button>
                  </div>
                </Td>
              </tr>
            ))}
        </TableShell>
      )}
      {editing && <RuleDialog rule={editing} onClose={() => setEditing(null)} />}
      <DeleteRuleDialog rule={deleting} onClose={() => setDeleting(null)} />
    </section>
  );
}

function RuleDialog({ rule, onClose }: { rule: RoutingRule | 'new' | null; onClose: () => void }) {
  const qc = useQueryClient();
  const toast = useToast();
  const isNew = rule === 'new';
  const open = rule !== null;
  const cats = useQuery({ queryKey: [...qk.admin, 'categories'], queryFn: api.adminCategories, enabled: open });
  const depts = useQuery({ queryKey: [...qk.admin, 'departments'], queryFn: api.adminDepartments, enabled: open });
  const users = useQuery({ queryKey: [...qk.admin, 'users'], queryFn: api.adminUsers, enabled: open });
  const [formError, setFormError] = useState<string | null>(null);

  const form = useForm<RuleValues>({
    resolver: zodResolver(ruleSchema),
    defaultValues:
      rule && rule !== 'new'
        ? {
            category_id: rule.category_id,
            request_type: rule.request_type,
            department_id: rule.department_id,
            assignee_user_id: rule.assignee_user_id,
            first_response_hours: rule.first_response_hours,
            resolution_hours: rule.resolution_hours,
            active: rule.active,
          }
        : { category_id: NaN, request_type: null, department_id: NaN, assignee_user_id: null, first_response_hours: 24, resolution_hours: 120, active: true },
  });

  const save = useMutation({
    mutationFn: (v: RuleValues) => (rule && rule !== 'new' ? api.updateRule(rule.id, v) : api.createRule(v)),
    onSuccess: () => {
      toast.success(ru.admin.ruleSaved);
      qc.invalidateQueries({ queryKey: qk.admin });
      onClose();
    },
    onError: (e) => {
      if (e instanceof ApiError && e.status === 409) setFormError(ru.admin.ruleConflict);
      else if (e instanceof ApiError && e.status === 400) {
        for (const [k, msg] of Object.entries(e.fields)) form.setError(k as keyof RuleValues, { message: msg });
        setFormError(e.message);
      } else setFormError(e.message);
    },
  });

  const deptId = useWatch({ control: form.control, name: 'department_id' });
  const staff = (users.data?.items ?? []).filter((u) => u.active && u.role !== 'employee' && u.department_id === deptId);
  const err = form.formState.errors;

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => !o && onClose()}
      busy={save.isPending}
      title={isNew ? ru.admin.newRule : ru.admin.editRule}
      description={ru.admin.ruleLead}
      footer={
        <>
          <Button onClick={onClose} disabled={save.isPending}>
            {ru.common.cancel}
          </Button>
          <Button type="submit" form="rule-form" variant="primary" loading={save.isPending} loadingText={ru.common.saving}>
            {ru.common.save}
          </Button>
        </>
      }
    >
      <form id="rule-form" noValidate onSubmit={form.handleSubmit((v) => save.mutate(v))} className="flex flex-col gap-4">
        <FormAlert message={formError} />
        <div className="grid gap-4 sm:grid-cols-2">
          <FormField label={ru.admin.ruleCategory} error={err.category_id?.message}>
            <Controller
              control={form.control}
              name="category_id"
              render={({ field }) => (
                <Select
                  value={Number.isFinite(field.value) ? String(field.value) : ''}
                  onValueChange={(v) => field.onChange(Number(v))}
                  placeholder={ru.admin.ruleCategory}
                  options={(cats.data?.items ?? []).map((c) => ({ value: String(c.id), label: c.active ? c.name : `${c.name} (${ru.common.inactive.toLowerCase()})` }))}
                />
              )}
            />
          </FormField>
          <FormField label={ru.admin.ruleType}>
            <Controller
              control={form.control}
              name="request_type"
              render={({ field }) => (
                <Select
                  value={field.value ?? ANY}
                  onValueChange={(v) => field.onChange(v === ANY ? null : v)}
                  options={[
                    { value: ANY, label: ru.admin.ruleTypeAny },
                    { value: 'problem', label: ru.types.problem },
                    { value: 'idea', label: ru.types.idea },
                  ]}
                />
              )}
            />
          </FormField>
          <FormField label={ru.admin.ruleDept} error={err.department_id?.message}>
            <Controller
              control={form.control}
              name="department_id"
              render={({ field }) => (
                <Select
                  value={Number.isFinite(field.value) ? String(field.value) : ''}
                  onValueChange={(v) => {
                    field.onChange(Number(v));
                    form.setValue('assignee_user_id', null);
                  }}
                  placeholder={ru.admin.ruleDept}
                  options={(depts.data?.items ?? []).map((d) => ({ value: String(d.id), label: d.name }))}
                />
              )}
            />
          </FormField>
          <FormField label={ru.admin.ruleAssignee}>
            <Controller
              control={form.control}
              name="assignee_user_id"
              render={({ field }) => (
                <Select
                  value={field.value ? String(field.value) : HEAD}
                  onValueChange={(v) => field.onChange(v === HEAD ? null : Number(v))}
                  disabled={!Number.isFinite(deptId)}
                  options={[{ value: HEAD, label: ru.admin.ruleAssigneeHead }, ...staff.map((u) => ({ value: String(u.id), label: u.name }))]}
                />
              )}
            />
          </FormField>
          <FormField label={ru.admin.ruleFirst} hint={ru.admin.ruleHoursHint} error={err.first_response_hours?.message}>
            <Input type="number" inputMode="numeric" min={1} step={1} {...form.register('first_response_hours', { valueAsNumber: true })} />
          </FormField>
          <FormField label={ru.admin.ruleResolution} hint={ru.admin.ruleHoursHint} error={err.resolution_hours?.message}>
            <Input type="number" inputMode="numeric" min={1} step={1} {...form.register('resolution_hours', { valueAsNumber: true })} />
          </FormField>
        </div>
        <Controller
          control={form.control}
          name="active"
          render={({ field }) => <Switch checked={field.value} onCheckedChange={field.onChange} label={ru.admin.ruleActive} />}
        />
      </form>
    </Dialog>
  );
}

function DeleteRuleDialog({ rule, onClose }: { rule: RoutingRule | null; onClose: () => void }) {
  const qc = useQueryClient();
  const toast = useToast();
  const del = useMutation({
    mutationFn: (id: number) => api.deleteRule(id),
    onSuccess: () => {
      toast.success(ru.admin.ruleDeleted);
      qc.invalidateQueries({ queryKey: qk.admin });
      onClose();
    },
    onError: (e) => toast.error(e.message),
  });
  return (
    <Dialog
      size="sm"
      open={rule !== null}
      onOpenChange={(o) => !o && onClose()}
      busy={del.isPending}
      title={ru.admin.ruleDeleteTitle}
      description={rule ? ru.admin.ruleDeleteText(rule.category_name) : undefined}
      footer={
        <>
          <Button onClick={onClose} disabled={del.isPending}>
            {ru.common.cancel}
          </Button>
          <Button variant="danger" loading={del.isPending} onClick={() => rule && del.mutate(rule.id)}>
            {ru.common.delete}
          </Button>
        </>
      }
    >
      <p className="text-sm text-muted">{rule && `${rule.category_name} → ${rule.department_name}`}</p>
    </Dialog>
  );
}
