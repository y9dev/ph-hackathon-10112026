-- Reverts 0003 schema changes. NOT lossless: the author link of anonymous requests was never stored,
-- so author-less rows are attributed to the lowest-id admin (the 0002 schema requires an author);
-- `feedback` requests/rules become `idea`; priority and token hashes are dropped (tokens stop working).
-- Requires at least one admin user. Run with foreign keys disabled, like the up migration.

CREATE TABLE requests_old (
    id                 INTEGER PRIMARY KEY,
    number             TEXT    NOT NULL UNIQUE,
    type               TEXT    NOT NULL CHECK (type IN ('problem', 'idea')),
    category_id        INTEGER NOT NULL REFERENCES categories(id),
    title              TEXT    NOT NULL,
    body               TEXT    NOT NULL,
    proposed_solution  TEXT,
    author_id          INTEGER NOT NULL REFERENCES users(id),
    is_anonymous       INTEGER NOT NULL DEFAULT 0 CHECK (is_anonymous IN (0, 1)),
    department_id      INTEGER REFERENCES departments(id),
    assignee_id        INTEGER REFERENCES users(id),
    status             TEXT    NOT NULL CHECK (status IN ('new', 'in_review', 'needs_info', 'decision', 'in_progress', 'implemented', 'declined', 'duplicate', 'closed')),
    routing_source     TEXT    NOT NULL CHECK (routing_source IN ('rule', 'manual', 'unrouted')),
    first_response_due INTEGER NOT NULL,
    resolution_due     INTEGER NOT NULL,
    first_responded_at INTEGER,
    paused_at          INTEGER,
    closed_at          INTEGER,
    result_text        TEXT,
    duplicate_of       INTEGER REFERENCES requests(id),
    theme_id           INTEGER REFERENCES themes(id) ON DELETE SET NULL,
    created_at         INTEGER NOT NULL,
    updated_at         INTEGER NOT NULL,
    CHECK (status NOT IN ('implemented', 'declined', 'duplicate', 'closed') OR (result_text IS NOT NULL AND length(trim(result_text)) > 0 AND closed_at IS NOT NULL)),
    CHECK (status <> 'duplicate' OR duplicate_of IS NOT NULL)
);
INSERT INTO requests_old (id, number, type, category_id, title, body, proposed_solution, author_id, is_anonymous, department_id, assignee_id,
        status, routing_source, first_response_due, resolution_due, first_responded_at, paused_at, closed_at, result_text, duplicate_of,
        theme_id, created_at, updated_at)
SELECT id, number, CASE type WHEN 'feedback' THEN 'idea' ELSE type END, category_id, title, body, proposed_solution,
        COALESCE(author_id, (SELECT MIN(id) FROM users WHERE role = 'admin')), is_anonymous, department_id, assignee_id,
        status, routing_source, first_response_due, resolution_due, first_responded_at, paused_at, closed_at, result_text, duplicate_of,
        theme_id, created_at, updated_at
FROM requests;

CREATE TABLE comments_old (
    id         INTEGER PRIMARY KEY,
    request_id INTEGER NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    author_id  INTEGER NOT NULL REFERENCES users(id),
    kind       TEXT    NOT NULL CHECK (kind IN ('comment', 'question', 'answer', 'internal', 'report')),
    body       TEXT    NOT NULL CHECK (length(body) BETWEEN 1 AND 4000),
    created_at INTEGER NOT NULL
);
INSERT INTO comments_old (id, request_id, author_id, kind, body, created_at)
SELECT c.id, c.request_id, COALESCE(c.author_id, r.author_id), c.kind, c.body, c.created_at
FROM comments c JOIN requests_old r ON r.id = c.request_id;

CREATE TABLE attachments_old (
    id          INTEGER PRIMARY KEY,
    request_id  INTEGER NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    comment_id  INTEGER REFERENCES comments(id) ON DELETE CASCADE,
    filename    TEXT    NOT NULL,
    stored_name TEXT    NOT NULL UNIQUE,
    mime        TEXT    NOT NULL,
    size        INTEGER NOT NULL CHECK (size >= 0),
    uploaded_by INTEGER NOT NULL REFERENCES users(id),
    created_at  INTEGER NOT NULL
);
INSERT INTO attachments_old (id, request_id, comment_id, filename, stored_name, mime, size, uploaded_by, created_at)
SELECT a.id, a.request_id, a.comment_id, a.filename, a.stored_name, a.mime, a.size, COALESCE(a.uploaded_by, r.author_id), a.created_at
FROM attachments a JOIN requests_old r ON r.id = a.request_id;

UPDATE request_events SET actor_id = (SELECT r.author_id FROM requests_old r WHERE r.id = request_events.request_id)
WHERE by_author = 1 AND actor_id IS NULL;
ALTER TABLE request_events DROP COLUMN by_author;

DROP TABLE attachments;
DROP TABLE comments;
DROP TABLE requests;
ALTER TABLE requests_old RENAME TO requests;
ALTER TABLE comments_old RENAME TO comments;
ALTER TABLE attachments_old RENAME TO attachments;
CREATE INDEX ix_requests_status_assignee ON requests(status, assignee_id);
CREATE INDEX ix_requests_department_status ON requests(department_id, status);
CREATE INDEX ix_requests_author ON requests(author_id);
CREATE INDEX ix_requests_first_response_due ON requests(first_response_due);
CREATE INDEX ix_requests_theme ON requests(theme_id);
CREATE INDEX ix_requests_created ON requests(created_at);
CREATE INDEX ix_requests_category ON requests(category_id);
CREATE INDEX ix_comments_request ON comments(request_id);
CREATE INDEX ix_attachments_request ON attachments(request_id);

CREATE TABLE routing_rules_old (
    id                   INTEGER PRIMARY KEY,
    category_id          INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    request_type         TEXT    CHECK (request_type IS NULL OR request_type IN ('problem', 'idea')),
    department_id        INTEGER NOT NULL REFERENCES departments(id),
    assignee_user_id     INTEGER REFERENCES users(id) ON DELETE SET NULL,
    first_response_hours INTEGER NOT NULL CHECK (first_response_hours BETWEEN 1 AND 8760),
    resolution_hours     INTEGER NOT NULL CHECK (resolution_hours BETWEEN 1 AND 8760),
    active               INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1))
);
-- feedback-specific rules are dropped (they would collide with idea rules on the unique index)
INSERT INTO routing_rules_old SELECT id, category_id, request_type, department_id, assignee_user_id, first_response_hours, resolution_hours, active
FROM routing_rules WHERE request_type IS NULL OR request_type <> 'feedback';
DROP TABLE routing_rules;
ALTER TABLE routing_rules_old RENAME TO routing_rules;
CREATE UNIQUE INDEX ux_routing_rules_active ON routing_rules(category_id, IFNULL(request_type, '*')) WHERE active = 1;
