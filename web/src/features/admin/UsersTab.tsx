import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Controller, useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Plus, Pencil, Users } from 'lucide-react';
import { api, ApiError } from '@/shared/api/client';
import { qk } from '@/shared/api/queries';
import type { AdminUser, Role } from '@/shared/api/types';
import { ru } from '@/shared/i18n/ru';
import { Avatar, Button, Dialog, EmptyState, ErrorState, FormField, Input, Select, Switch, useToast } from '@/shared/ui';
import { ActiveBadge, FormAlert, TableShell, TableSkeleton, Td, Th } from './common';
import { makeUserSchema, type UserValues } from './schemas';

const NO_DEPT = '__none';
const ROLES: Role[] = ['employee', 'manager', 'admin'];

export function UsersTab() {
  const users = useQuery({ queryKey: [...qk.admin, 'users'], queryFn: api.adminUsers });
  const depts = useQuery({ queryKey: [...qk.admin, 'departments'], queryFn: api.adminDepartments });
  const [editing, setEditing] = useState<AdminUser | 'new' | null>(null);
  const deptName = (id: number | null) => depts.data?.items.find((d) => d.id === id)?.name;

  return (
    <section aria-labelledby="users-h">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <h2 id="users-h" className="text-xl font-semibold text-text">
          {ru.admin.users}
        </h2>
        <Button variant="primary" onClick={() => setEditing('new')} icon={<Plus className="h-4 w-4" aria-hidden="true" />}>
          {ru.admin.addUser}
        </Button>
      </div>
      {users.isPending ? (
        <TableSkeleton />
      ) : users.isError ? (
        <ErrorState error={users.error} onRetry={() => users.refetch()} />
      ) : users.data.items.length === 0 ? (
        <div className="rounded-xl border border-border bg-surface">
          <EmptyState icon={<Users className="h-6 w-6" />} title={ru.admin.userEmpty} />
        </div>
      ) : (
        <TableShell
          caption={ru.admin.users}
          head={
            <>
              <Th>{ru.admin.userName}</Th>
              <Th>{ru.admin.userEmail}</Th>
              <Th>{ru.admin.userRole}</Th>
              <Th>{ru.admin.userDept}</Th>
              <Th>{ru.admin.status}</Th>
              <Th className="w-20 text-right">
                <span className="sr-only">{ru.admin.actions}</span>
              </Th>
            </>
          }
        >
          {[...users.data.items]
            .sort((a, b) => a.name.localeCompare(b.name, 'ru'))
            .map((u) => (
              <tr key={u.id} className={u.active ? '' : 'text-muted'}>
                <Td>
                  <span className="flex items-center gap-2 font-medium text-text">
                    <Avatar name={u.name} size="sm" />
                    {u.name}
                  </span>
                </Td>
                <Td className="text-muted">{u.email}</Td>
                <Td>{ru.roles[u.role]}</Td>
                <Td>{deptName(u.department_id) ?? <span className="text-muted">{ru.admin.userNoDept}</span>}</Td>
                <Td>
                  <ActiveBadge active={u.active} user />
                </Td>
                <Td className="text-right">
                  <Button size="icon-sm" variant="ghost" aria-label={ru.admin.rowActions(u.name)} onClick={() => setEditing(u)}>
                    <Pencil className="h-4 w-4" aria-hidden="true" />
                  </Button>
                </Td>
              </tr>
            ))}
        </TableShell>
      )}
      {editing && <UserDialog user={editing} onClose={() => setEditing(null)} departments={depts.data?.items ?? []} />}
    </section>
  );
}

function UserDialog({
  user,
  onClose,
  departments,
}: {
  user: AdminUser | 'new' | null;
  onClose: () => void;
  departments: { id: number; name: string }[];
}) {
  const qc = useQueryClient();
  const toast = useToast();
  const open = user !== null;
  const isNew = user === 'new';
  const [formError, setFormError] = useState<string | null>(null);
  const form = useForm<UserValues>({
    resolver: zodResolver(makeUserSchema(isNew)),
    defaultValues:
      user && user !== 'new'
        ? { name: user.name, email: user.email, role: user.role, department_id: user.department_id, active: user.active, password: '' }
        : { name: '', email: '', role: 'employee', department_id: null, active: true, password: '' },
  });

  const save = useMutation({
    mutationFn: (v: UserValues) => {
      const body = { ...v, email: v.email.trim(), name: v.name.trim(), password: v.password || undefined };
      return user && user !== 'new' ? api.updateUser(user.id, body) : api.createUser(body);
    },
    onSuccess: () => {
      toast.success(ru.admin.userSaved);
      qc.invalidateQueries({ queryKey: qk.admin });
      onClose();
    },
    onError: (e) => {
      if (e instanceof ApiError && (e.status === 400 || e.status === 409) && Object.keys(e.fields).length) {
        for (const [k, msg] of Object.entries(e.fields)) form.setError(k as keyof UserValues, { message: msg });
      } else if (e instanceof ApiError && e.status === 409) setFormError(e.message || ru.admin.emailConflict);
      else setFormError(e.message);
    },
  });
  const err = form.formState.errors;
  return (
    <Dialog
      open={open}
      onOpenChange={(o) => !o && onClose()}
      busy={save.isPending}
      title={isNew ? ru.admin.newUser : ru.admin.editUser}
      footer={
        <>
          <Button onClick={onClose} disabled={save.isPending}>
            {ru.common.cancel}
          </Button>
          <Button type="submit" form="user-form" variant="primary" loading={save.isPending} loadingText={ru.common.saving}>
            {ru.common.save}
          </Button>
        </>
      }
    >
      <form id="user-form" noValidate onSubmit={form.handleSubmit((v) => save.mutate(v))} className="flex flex-col gap-4">
        <FormAlert message={formError} />
        <FormField label={ru.admin.userName} error={err.name?.message}>
          <Input {...form.register('name')} autoComplete="off" />
        </FormField>
        <FormField label={ru.admin.userEmail} error={err.email?.message}>
          <Input type="email" inputMode="email" autoComplete="off" {...form.register('email')} />
        </FormField>
        <div className="grid gap-4 sm:grid-cols-2">
          <FormField label={ru.admin.userRole}>
            <Controller
              control={form.control}
              name="role"
              render={({ field }) => (
                <Select value={field.value} onValueChange={field.onChange} options={ROLES.map((r) => ({ value: r, label: ru.roles[r] }))} />
              )}
            />
          </FormField>
          <FormField label={ru.admin.userDept}>
            <Controller
              control={form.control}
              name="department_id"
              render={({ field }) => (
                <Select
                  value={field.value ? String(field.value) : NO_DEPT}
                  onValueChange={(v) => field.onChange(v === NO_DEPT ? null : Number(v))}
                  options={[{ value: NO_DEPT, label: ru.admin.userNoDept }, ...departments.map((d) => ({ value: String(d.id), label: d.name }))]}
                />
              )}
            />
          </FormField>
        </div>
        <FormField
          label={isNew ? ru.admin.userPassword : ru.admin.userPasswordEdit}
          hint={isNew ? ru.admin.userPasswordNewHint : ru.admin.userPasswordHint}
          optional={!isNew}
          error={err.password?.message}
        >
          <Input type="password" autoComplete="new-password" {...form.register('password')} />
        </FormField>
        <Controller
          control={form.control}
          name="active"
          render={({ field }) => <Switch checked={Boolean(field.value)} onCheckedChange={field.onChange} label={ru.admin.userActive} />}
        />
      </form>
    </Dialog>
  );
}
