import { useMeta } from '@/shared/api/queries';

/** CR4: every Telegram entry point is gated on /meta features.telegram (missing flag = off). */
export function useTelegramEnabled(): boolean {
  const meta = useMeta();
  return meta.data?.features?.telegram === true;
}
