import { Send } from 'lucide-react';

/** Paper-plane glyph used for Telegram entry points. Decorative: label the control instead. */
export function TelegramIcon({ className }: { className?: string }) {
  return <Send className={className ?? 'h-5 w-5'} aria-hidden="true" focusable="false" />;
}
