-- Anonymous spam moderation. A request marked as spam is hidden from every list and detail view.
-- anon_spam_blocks keeps ONLY the SHA-256 of the token (the same form as requests.anon_token_hash):
-- a blocked token cannot be used for lookup, comments or anonymous submission from a browser that
-- sends the token back. Tokens themselves are never stored by the server.

ALTER TABLE requests ADD COLUMN spam_hidden INTEGER NOT NULL DEFAULT 0;

CREATE TABLE anon_spam_blocks (
    token_hash TEXT PRIMARY KEY,
    blocked_at INTEGER NOT NULL
);
