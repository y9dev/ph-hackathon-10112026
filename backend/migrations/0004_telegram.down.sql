DROP INDEX IF EXISTS ix_notifications_tg_pending;
ALTER TABLE notifications DROP COLUMN telegram_attempts;
ALTER TABLE notifications DROP COLUMN telegram_sent_at;
DROP TABLE IF EXISTS telegram_links;
