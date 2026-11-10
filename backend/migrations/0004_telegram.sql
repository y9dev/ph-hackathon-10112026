-- CR4: Telegram bot. telegram_links binds one user to exactly one chat (permanent; the product has
-- no unlink). notifications gain a delivery marker so the mirroring worker is at-least-once and
-- restart safe: telegram_sent_at stays NULL until Telegram acknowledged the message (or the
-- message was given up on after repeated failures / the user linked after it was created).

CREATE TABLE telegram_links (
    user_id     INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    chat_id     INTEGER NOT NULL UNIQUE,
    tg_username TEXT,
    linked_at   INTEGER NOT NULL
);

ALTER TABLE notifications ADD COLUMN telegram_sent_at INTEGER;
ALTER TABLE notifications ADD COLUMN telegram_attempts INTEGER NOT NULL DEFAULT 0;

-- the mirroring worker scans unsent notifications of linked users in id order
CREATE INDEX ix_notifications_tg_pending ON notifications(user_id, id) WHERE telegram_sent_at IS NULL;
