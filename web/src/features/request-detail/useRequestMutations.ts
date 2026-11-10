import { useMutation, useQueryClient } from '@tanstack/react-query';
import { api } from '@/shared/api/client';
import { qk } from '@/shared/api/queries';
import type { CommentKind, RequestDetail, Status } from '@/shared/api/types';

/** All request mutations write the returned RequestDetail into the cache and refresh lists. */
export function useRequestMutations(id: number) {
  const qc = useQueryClient();
  const onSuccess = (r: RequestDetail) => {
    qc.setQueryData(qk.request(id), r);
    qc.invalidateQueries({ queryKey: qk.requests });
    qc.invalidateQueries({ queryKey: qk.notifications });
    qc.invalidateQueries({ queryKey: qk.analytics });
  };
  return {
    status: useMutation({
      mutationFn: (b: { status: Status; result_text?: string; duplicate_of_id?: number }) => api.changeStatus(id, b),
      onSuccess,
    }),
    comment: useMutation({
      mutationFn: (b: { kind: CommentKind; body: string }) => api.addComment(id, b),
      onSuccess,
    }),
    assign: useMutation({
      mutationFn: (b: { department_id?: number; assignee_id?: number | null }) => api.assign(id, b),
      onSuccess,
    }),
    theme: useMutation({
      mutationFn: (b: { theme_id?: number; title?: string }) => api.linkTheme(id, b),
      onSuccess: (r) => {
        onSuccess(r);
        qc.invalidateQueries({ queryKey: qk.themes });
        qc.invalidateQueries({ queryKey: qk.groups });
      },
    }),
  };
}
