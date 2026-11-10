import { forwardRef, useEffect, useState, type FormEvent } from 'react';
import { MessageSquare, MessageCircleQuestion, Reply, Lock, Send } from 'lucide-react';
import type { CommentKind, RequestDetail } from '@/shared/api/types';
import { ApiError } from '@/shared/api/client';
import { cn } from '@/shared/lib/cn';
import { ru } from '@/shared/i18n/ru';
import { Button, Counter, FormField, SegmentedControl, Textarea, useToast } from '@/shared/ui';
import { useRequestMutations } from './useRequestMutations';

export const COMMENT_MAX = 4000;
const ICON: Record<CommentKind, typeof MessageSquare> = {
  comment: MessageSquare,
  question: MessageCircleQuestion,
  answer: Reply,
  internal: Lock,
};
const SENT: Record<CommentKind, string> = {
  comment: ru.detail.sentComment,
  question: ru.detail.sentQuestion,
  answer: ru.detail.sentAnswer,
  internal: ru.detail.sentInternal,
};

/** Order kinds so the most likely action is first; answer wins when the author is asked. */
export function defaultKind(kinds: CommentKind[]): CommentKind | undefined {
  if (kinds.includes('answer')) return 'answer';
  if (kinds.includes('comment')) return 'comment';
  return kinds[0];
}

export const Composer = forwardRef<HTMLTextAreaElement, { request: RequestDetail }>(function Composer({ request }, ref) {
  const kinds = request.allowed_comment_kinds;
  const [kind, setKind] = useState<CommentKind | undefined>(() => defaultKind(kinds));
  const [text, setText] = useState('');
  const [error, setError] = useState<string | undefined>();
  const { comment } = useRequestMutations(request.id);
  const toast = useToast();

  // Keep the selected kind valid when allowed kinds change (e.g. after an answer).
  const kindsKey = kinds.join(',');
  useEffect(() => {
    setKind((k) => (k && kinds.includes(k) ? k : defaultKind(kinds)));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [kindsKey]);

  if (!kind || kinds.length === 0) return null;

  function submit(e: FormEvent) {
    e.preventDefault();
    if (comment.isPending || !kind) return;
    const body = text.trim();
    if (!body) return setError(kind === 'question' ? ru.detail.questionRequired : ru.detail.composerEmpty);
    if (body.length > COMMENT_MAX) return setError(ru.detail.composerMax(COMMENT_MAX));
    setError(undefined);
    comment.mutate(
      { kind, body },
      {
        onSuccess: () => {
          setText('');
          toast.success(SENT[kind]);
        },
        onError: (err) => {
          if (err instanceof ApiError && err.fields.body) setError(err.fields.body);
          else toast.error(err.message);
        },
      },
    );
  }

  const internal = kind === 'internal';
  return (
    <form
      onSubmit={submit}
      noValidate
      className={cn(
        'rounded-xl border p-4 transition-colors duration-150 md:p-6',
        internal ? 'border-dashed border-warning/50 bg-warning-soft/50' : 'border-border bg-surface',
      )}
      aria-labelledby="composer-title"
    >
      <h3 id="composer-title" className="sr-only">
        {ru.detail.composer}
      </h3>
      {kinds.length > 1 && (
        <SegmentedControl
          className="mb-4"
          size="sm"
          aria-label={ru.detail.composerKind}
          value={kind}
          onChange={(k) => {
            setKind(k);
            setError(undefined);
          }}
          options={kinds.map((k) => {
            const Icon = ICON[k];
            return { value: k, label: ru.commentKinds[k], icon: <Icon className="h-4 w-4" aria-hidden="true" /> };
          })}
        />
      )}
      <FormField
        label={kinds.length > 1 ? ru.detail.composerLabel : ru.commentKinds[kind]}
        hint={ru.commentKindHints[kind]}
        error={error}
        aside={text.length > COMMENT_MAX * 0.8 ? <Counter value={text.length} max={COMMENT_MAX} /> : undefined}
      >
        <Textarea
          ref={ref}
          rows={3}
          value={text}
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) submit(e as unknown as FormEvent);
          }}
        />
      </FormField>
      <div className="mt-4 flex items-center justify-end gap-3">
        <span className="text-xs text-muted max-md:hidden">Ctrl + Enter</span>
        <Button type="submit" variant={internal ? 'secondary' : 'primary'} loading={comment.isPending} loadingText={ru.detail.composerSending} icon={<Send className="h-4 w-4" aria-hidden="true" />}>
          {ru.detail.composerSend}
        </Button>
      </div>
    </form>
  );
});
