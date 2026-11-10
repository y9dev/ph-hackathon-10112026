import { useId, useState } from 'react';
import { Copy, Download, KeyRound, ShieldAlert, Check } from 'lucide-react';
import { saveBlob } from '@/shared/lib/download';
import { ru } from '@/shared/i18n/ru';
import { Button, useToast } from '@/shared/ui';

/** Copies text; falls back to a hidden textarea + execCommand for non-secure contexts. */
export async function copyText(text: string): Promise<boolean> {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    // try the legacy path below
  }
  try {
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.setAttribute('readonly', '');
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.select();
    const ok = document.execCommand('copy');
    ta.remove();
    return ok;
  } catch {
    return false;
  }
}

/** The one-time token of an anonymous request: shown prominently with copy / download and a "saved" gate. */
export function TokenCard({
  number,
  token,
  saved,
  onSavedChange,
}: {
  number: string;
  token: string;
  saved: boolean;
  onSavedChange: (v: boolean) => void;
}) {
  const toast = useToast();
  const [copied, setCopied] = useState(false);
  const checkId = `tok${useId().replace(/:/g, '')}`;
  const leadId = `${checkId}-lead`;

  async function copy() {
    if (await copyText(token)) {
      setCopied(true);
      toast.success(ru.success.tokenCopied);
      window.setTimeout(() => setCopied(false), 2000);
    } else toast.error(ru.success.tokenCopyFailed);
  }
  function download() {
    const url = `${window.location.origin}/anonymous`;
    saveBlob(new Blob([ru.success.tokenFile(number, token, url)], { type: 'text/plain;charset=utf-8' }), ru.success.tokenFileName(number));
  }

  return (
    <section
      aria-labelledby={`${checkId}-title`}
      className="mt-8 rounded-xl border-2 border-warning/60 bg-surface p-6 shadow-sm max-sm:p-4"
      data-testid="token-card"
    >
      <div className="flex items-center gap-2">
        <KeyRound className="h-5 w-5 shrink-0 text-warning" aria-hidden="true" />
        <h2 id={`${checkId}-title`} className="text-lg font-semibold text-text">
          {ru.success.tokenTitle}
        </h2>
      </div>
      <p id={leadId} className="mt-3 flex items-start gap-2 rounded-lg bg-warning-soft px-3 py-2 text-sm font-medium text-text">
        <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0 text-warning" aria-hidden="true" />
        <span>{ru.success.tokenLead}</span>
      </p>
      <p
        className="tabular mt-4 select-all rounded-lg border border-border bg-surface-2 px-4 py-4 text-center font-mono text-lg font-semibold tracking-wider text-text sm:text-xl"
        data-testid="anonymous-token"
        aria-describedby={leadId}
      >
        {/* Wrap only between groups so a group is never split across lines. */}
        {token.split('-').map((g, i, all) => (
          <span key={i} className="inline-block whitespace-nowrap">
            {g}
            {i < all.length - 1 ? '-' : ''}
          </span>
        ))}
      </p>
      <div className="mt-3 flex flex-wrap gap-2 max-sm:flex-col">
        <Button onClick={copy} icon={copied ? <Check className="h-4 w-4" aria-hidden="true" /> : <Copy className="h-4 w-4" aria-hidden="true" />}>
          {copied ? ru.success.tokenCopied : ru.success.tokenCopy}
        </Button>
        <Button onClick={download} icon={<Download className="h-4 w-4" aria-hidden="true" />}>
          {ru.success.tokenDownload}
        </Button>
      </div>
      <p className="mt-4 text-sm text-muted">{ru.success.tokenHow}</p>
      <p className="mt-2 text-sm text-muted">{ru.success.tokenDevice}</p>
      <div className="relative mt-4 flex min-h-11 items-center gap-3 rounded-lg border border-border px-3">
        <input
          id={checkId}
          type="checkbox"
          checked={saved}
          onChange={(e) => onSavedChange(e.target.checked)}
          className="relative z-[1] h-5 w-5 shrink-0 accent-[var(--brand)]"
        />
        <label htmlFor={checkId} className="cursor-pointer py-2 text-sm font-medium text-text after:absolute after:inset-0 after:content-['']">
          {ru.success.tokenSaved}
        </label>
      </div>
    </section>
  );
}
