import { ApiError } from '@/shared/api/client';
import { ru } from '@/shared/i18n/ru';

/** Downloads a same-origin file with the session cookie and saves it under `filename`. */
export async function downloadFile(url: string, filename: string, headers?: Record<string, string>): Promise<void> {
  let res: Response;
  try {
    res = await fetch(url, { credentials: 'same-origin', headers });
  } catch {
    throw new ApiError(0, 'network', ru.errors.network);
  }
  if (!res.ok) throw new ApiError(res.status, String(res.status), res.status === 404 ? ru.errors.notFound : ru.errors.generic);
  saveBlob(await res.blob(), filename);
}

/** Saves a blob under `filename` via a temporary object URL. */
export function saveBlob(blob: Blob, filename: string) {
  const href = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = href;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(href), 1000);
}
