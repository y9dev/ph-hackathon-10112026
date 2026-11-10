import { z } from 'zod';
import type { CreateRequest, MetaLimits } from '@/shared/api/types';
import { ru } from '@/shared/i18n/ru';

export const TITLE_MIN = 5;
export const BODY_MIN = 20;
export const SOLUTION_MAX = 2000;

/** Same limits as the server (CreateRequest in openapi.yaml + /meta limits). */
export function makeSubmitSchema(limits: Pick<MetaLimits, 'title_max' | 'body_max'>) {
  const e = ru.submit.errors;
  return z.object({
    type: z.enum(['problem', 'idea'], { errorMap: () => ({ message: e.type }) }),
    category_id: z.number({ invalid_type_error: e.category, required_error: e.category }).int().positive(e.category),
    title: z.string().trim().min(TITLE_MIN, e.titleMin(TITLE_MIN)).max(limits.title_max, e.titleMax(limits.title_max)),
    body: z.string().trim().min(BODY_MIN, e.bodyMin(BODY_MIN)).max(limits.body_max, e.bodyMax(limits.body_max)),
    proposed_solution: z.string().max(SOLUTION_MAX, e.solutionMax(SOLUTION_MAX)),
    is_anonymous: z.boolean(),
  });
}

export type SubmitValues = z.infer<ReturnType<typeof makeSubmitSchema>>;

export const EMPTY_VALUES: SubmitValues = {
  type: 'problem',
  category_id: undefined as unknown as number,
  title: '',
  body: '',
  proposed_solution: '',
  is_anonymous: false,
};

export function toPayload(v: SubmitValues): CreateRequest {
  const sol = v.proposed_solution.trim();
  return {
    type: v.type,
    category_id: v.category_id,
    title: v.title.trim(),
    body: v.body.trim(),
    ...(sol ? { proposed_solution: sol } : {}),
    is_anonymous: v.is_anonymous,
  };
}

export interface FileCheck {
  accepted: File[];
  rejected: { name: string; reason: string }[];
}

/** Client-side attachment validation mirroring server limits; returns the new accepted list. */
export function checkFiles(existing: File[], incoming: File[], limits: MetaLimits): FileCheck {
  const accepted = [...existing];
  const rejected: FileCheck['rejected'] = [];
  const allowed = limits.allowed_extensions.map((x) => x.toLowerCase().replace(/^\./, ''));
  let total = existing.reduce((s, f) => s + f.size, 0);
  for (const f of incoming) {
    const ext = f.name.includes('.') ? f.name.split('.').pop()!.toLowerCase() : '';
    if (!allowed.includes(ext)) {
      rejected.push({ name: f.name, reason: ru.submit.fileBadType });
      continue;
    }
    if (f.size > limits.max_file_mb * 1024 * 1024) {
      rejected.push({ name: f.name, reason: ru.submit.fileTooBig(limits.max_file_mb) });
      continue;
    }
    if (accepted.length >= limits.max_files) {
      rejected.push({ name: f.name, reason: ru.submit.fileTooMany(limits.max_files) });
      continue;
    }
    if (total + f.size > limits.max_total_mb * 1024 * 1024) {
      rejected.push({ name: f.name, reason: ru.submit.fileTotalTooBig(limits.max_total_mb) });
      continue;
    }
    if (accepted.some((a) => a.name === f.name && a.size === f.size)) continue;
    accepted.push(f);
    total += f.size;
  }
  return { accepted, rejected };
}
