import { z } from 'zod';
import { ru } from '@/shared/i18n/ru';

const hours = z
  .number({ invalid_type_error: ru.admin.hoursInt })
  .int(ru.admin.hoursInt)
  .min(1, ru.admin.hoursMin);

export const ruleSchema = z.object({
  category_id: z.number({ invalid_type_error: ru.submit.errors.category }).int().positive(ru.submit.errors.category),
  request_type: z.enum(['problem', 'idea']).nullable(),
  department_id: z.number({ invalid_type_error: ru.detail.assignDeptRequired }).int().positive(ru.detail.assignDeptRequired),
  assignee_user_id: z.number().int().positive().nullable(),
  first_response_hours: hours,
  resolution_hours: hours,
  active: z.boolean(),
});
export type RuleValues = z.infer<typeof ruleSchema>;

export const categorySchema = z.object({
  name: z.string().trim().min(1, ru.admin.catNameRequired).max(80),
  description: z.string().trim().max(300),
  icon: z.string(),
  sort: z.number({ invalid_type_error: ru.admin.hoursInt }).int(),
  active: z.boolean(),
});
export type CategoryValues = z.infer<typeof categorySchema>;

/** Password is required (≥8) for new users, optional (empty = unchanged) when editing. */
export function makeUserSchema(isNew: boolean) {
  return z.object({
    name: z.string().trim().min(1, ru.admin.nameRequired).max(120),
    email: z.string().trim().min(1, ru.admin.emailRequired).email(ru.admin.emailInvalid),
    role: z.enum(['employee', 'manager', 'admin']),
    department_id: z.number().int().positive().nullable(),
    active: z.boolean(),
    password: isNew
      ? z.string().min(8, ru.admin.passwordMin)
      : z.string().refine((v) => v === '' || v.length >= 8, ru.admin.passwordMin),
  });
}
export type UserValues = z.infer<ReturnType<typeof makeUserSchema>>;
