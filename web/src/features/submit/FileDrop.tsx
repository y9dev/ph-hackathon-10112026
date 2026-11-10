import { useId, useRef, useState, type DragEvent } from 'react';
import { Paperclip, X, FileText, Image as ImageIcon, UploadCloud, CircleAlert, TriangleAlert } from 'lucide-react';
import type { MetaLimits } from '@/shared/api/types';
import { cn } from '@/shared/lib/cn';
import { formatBytes } from '@/shared/lib/format';
import { ru } from '@/shared/i18n/ru';
import { checkFiles, type FileCheck } from './schema';

export function FileDrop({
  files,
  onChange,
  limits,
  serverError,
  disabled,
}: {
  files: File[];
  onChange: (f: File[]) => void;
  limits: MetaLimits;
  serverError?: string;
  disabled?: boolean;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [drag, setDrag] = useState(false);
  const [rejected, setRejected] = useState<FileCheck['rejected']>([]);
  const id = useId().replace(/:/g, '');
  const hintId = `fd-${id}-hint`;
  const errId = `fd-${id}-err`;
  const accept = limits.allowed_extensions.map((e) => `.${e.replace(/^\./, '')}`).join(',');

  function add(list: FileList | File[]) {
    const res = checkFiles(files, Array.from(list), limits);
    setRejected(res.rejected);
    onChange(res.accepted);
  }
  function onDrop(e: DragEvent) {
    e.preventDefault();
    setDrag(false);
    if (!disabled && e.dataTransfer.files.length) add(e.dataTransfer.files);
  }
  const hasImage = files.some((f) => f.type.startsWith('image/') || /\.(jpe?g|png)$/i.test(f.name));
  const exts = limits.allowed_extensions.map((e) => e.toUpperCase()).join(', ');

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-baseline justify-between gap-3">
        <span className="text-sm font-medium text-text" id={`fd-${id}-label`}>
          {ru.submit.files} <span className="font-normal text-muted">({ru.common.optional})</span>
        </span>
        <span className="tabular text-xs text-muted">
          {files.length} / {limits.max_files}
        </span>
      </div>
      <p id={hintId} className="text-sm text-muted">
        {ru.submit.filesHint(limits.max_files, limits.max_file_mb, exts)}
      </p>
      <div
        onDragOver={(e) => {
          e.preventDefault();
          if (!disabled) setDrag(true);
        }}
        onDragLeave={() => setDrag(false)}
        onDrop={onDrop}
        className={cn(
          'flex flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed px-4 py-6 text-center transition-colors duration-150',
          drag ? 'border-brand bg-brand-soft' : 'border-border bg-surface-2/50',
          serverError && 'border-danger',
        )}
      >
        <UploadCloud className={cn('h-6 w-6', drag ? 'text-brand' : 'text-muted')} aria-hidden="true" />
        <p className="text-sm text-muted">
          {drag ? (
            ru.submit.dropActive
          ) : (
            <>
              {ru.submit.drop}{' '}
              <button
                type="button"
                disabled={disabled || files.length >= limits.max_files}
                onClick={() => inputRef.current?.click()}
                aria-describedby={[serverError ? errId : null, hintId].filter(Boolean).join(' ')}
                className="inline-flex min-h-11 items-center font-medium text-brand underline-offset-4 hover:underline disabled:text-muted md:min-h-0"
              >
                {ru.submit.dropButton}
              </button>
            </>
          )}
        </p>
        <input
          ref={inputRef}
          type="file"
          multiple
          accept={accept}
          className="sr-only"
          tabIndex={-1}
          aria-hidden="true"
          aria-label={ru.submit.files}
          onChange={(e) => {
            if (e.target.files) add(e.target.files);
            e.target.value = '';
          }}
        />
      </div>

      {serverError && (
        <p id={errId} className="flex items-start gap-1 text-sm text-danger">
          <CircleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
          {serverError}
        </p>
      )}

      {rejected.length > 0 && (
        <ul className="flex flex-col gap-1" role="alert">
          {rejected.map((r) => (
            <li key={r.name} className="flex items-start gap-2 rounded-lg bg-danger-soft px-3 py-2 text-sm text-danger">
              <CircleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden="true" />
              <span className="min-w-0 break-words">
                <span className="font-medium">{r.name}</span>: {r.reason}
              </span>
            </li>
          ))}
        </ul>
      )}

      {files.length > 0 && (
        <ul className="flex flex-col gap-2" aria-label={ru.submit.files}>
          {files.map((f, i) => {
            const isImg = f.type.startsWith('image/');
            const Icon = isImg ? ImageIcon : f.name.endsWith('.txt') || f.name.endsWith('.pdf') ? FileText : Paperclip;
            return (
              <li key={`${f.name}-${f.size}`} className="flex items-center gap-3 rounded-lg border border-border bg-surface px-3 py-2 animate-fade-in">
                <Icon className="h-4 w-4 shrink-0 text-muted" aria-hidden="true" />
                <span className="min-w-0 flex-1 truncate text-sm text-text">{f.name}</span>
                <span className="tabular shrink-0 text-xs text-muted">{formatBytes(f.size)}</span>
                <button
                  type="button"
                  onClick={() => {
                    onChange(files.filter((_, j) => j !== i));
                    setRejected([]);
                  }}
                  aria-label={ru.submit.removeFile(f.name)}
                  className="-mr-1 inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-muted hover:bg-surface-2 hover:text-text max-md:h-11 max-md:w-11"
                >
                  <X className="h-4 w-4" aria-hidden="true" />
                </button>
              </li>
            );
          })}
        </ul>
      )}
      {hasImage && (
        <p className="flex items-start gap-2 text-sm text-muted">
          <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0 text-warning" aria-hidden="true" />
          {ru.submit.exifWarning}
        </p>
      )}
    </div>
  );
}
