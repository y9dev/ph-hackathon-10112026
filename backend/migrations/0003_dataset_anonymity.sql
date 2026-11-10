-- CR1 + CR2: request priority, request type `feedback`, and full anonymity (no author link in the DB).
--
-- * requests.priority low|medium|high (default medium); requests.type and routing_rules.request_type
--   accept `feedback`.
-- * Anonymous requests store NO author: requests.author_id IS NULL exactly when is_anonymous = 1
--   (CHECK). The author follows the request with a secret token; only its SHA-256 is stored in
--   requests.anon_token_hash (UNIQUE when set).
-- * Author-side rows carry by_author = 1 (comments, request_events); for anonymous requests their
--   author_id / actor_id is NULL. attachments.uploaded_by is NULL for anonymous requests.
-- * EXISTING anonymous requests lose the author link (it is nulled, irreversibly): their former
--   authors can no longer open them from "Мои обращения" — there is no token for legacy rows.
--   Notifications of the former author about those requests are deleted for the same reason.
--
-- SQLite cannot alter NOT NULL / CHECK constraints, so tables are rebuilt. The runner applies the
-- migration with foreign keys disabled and verifies integrity before commit. Order matters: rows
-- are classified while the old requests table (with author_id) still exists.

-- 1. comments: author_id nullable + by_author flag
CREATE TABLE comments_new (
    id         INTEGER PRIMARY KEY,
    request_id INTEGER NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    author_id  INTEGER REFERENCES users(id),
    by_author  INTEGER NOT NULL DEFAULT 0 CHECK (by_author IN (0, 1)),
    kind       TEXT    NOT NULL CHECK (kind IN ('comment', 'question', 'answer', 'internal', 'report')),
    body       TEXT    NOT NULL CHECK (length(body) BETWEEN 1 AND 4000),
    created_at INTEGER NOT NULL,
    -- staff rows always name their author; only author-side rows of anonymous requests are NULL
    CHECK (author_id IS NOT NULL OR by_author = 1)
);
INSERT INTO comments_new (id, request_id, author_id, by_author, kind, body, created_at)
SELECT c.id, c.request_id,
       CASE WHEN r.is_anonymous = 1 AND c.author_id = r.author_id THEN NULL ELSE c.author_id END,
       CASE WHEN c.author_id = r.author_id THEN 1 ELSE 0 END,
       c.kind, c.body, c.created_at
FROM comments c JOIN requests r ON r.id = c.request_id;
DROP TABLE comments;
ALTER TABLE comments_new RENAME TO comments;
CREATE INDEX ix_comments_request ON comments(request_id);

-- 2. request_events: by_author flag (actor_id is already nullable)
ALTER TABLE request_events ADD COLUMN by_author INTEGER NOT NULL DEFAULT 0 CHECK (by_author IN (0, 1));
UPDATE request_events SET by_author = 1
WHERE actor_id IS NOT NULL AND actor_id = (SELECT r.author_id FROM requests r WHERE r.id = request_events.request_id);
UPDATE request_events SET actor_id = NULL
WHERE by_author = 1 AND request_id IN (SELECT id FROM requests WHERE is_anonymous = 1);

-- 3. attachments: uploaded_by nullable
CREATE TABLE attachments_new (
    id          INTEGER PRIMARY KEY,
    request_id  INTEGER NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    comment_id  INTEGER REFERENCES comments(id) ON DELETE CASCADE,
    filename    TEXT    NOT NULL,
    stored_name TEXT    NOT NULL UNIQUE,
    mime        TEXT    NOT NULL,
    size        INTEGER NOT NULL CHECK (size >= 0),
    uploaded_by INTEGER REFERENCES users(id),
    created_at  INTEGER NOT NULL
);
INSERT INTO attachments_new (id, request_id, comment_id, filename, stored_name, mime, size, uploaded_by, created_at)
SELECT a.id, a.request_id, a.comment_id, a.filename, a.stored_name, a.mime, a.size,
       CASE WHEN r.is_anonymous = 1 AND a.uploaded_by = r.author_id THEN NULL ELSE a.uploaded_by END,
       a.created_at
FROM attachments a JOIN requests r ON r.id = a.request_id;
DROP TABLE attachments;
ALTER TABLE attachments_new RENAME TO attachments;
CREATE INDEX ix_attachments_request ON attachments(request_id);

-- 4. notifications of the former author about their anonymous requests
DELETE FROM notifications
WHERE EXISTS (SELECT 1 FROM requests r WHERE r.id = notifications.request_id AND r.is_anonymous = 1 AND r.author_id = notifications.user_id);

-- 5. routing_rules: request_type accepts feedback
CREATE TABLE routing_rules_new (
    id                   INTEGER PRIMARY KEY,
    category_id          INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    request_type         TEXT    CHECK (request_type IS NULL OR request_type IN ('problem', 'idea', 'feedback')),
    department_id        INTEGER NOT NULL REFERENCES departments(id),
    assignee_user_id     INTEGER REFERENCES users(id) ON DELETE SET NULL,
    first_response_hours INTEGER NOT NULL CHECK (first_response_hours BETWEEN 1 AND 8760),
    resolution_hours     INTEGER NOT NULL CHECK (resolution_hours BETWEEN 1 AND 8760),
    active               INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1))
);
INSERT INTO routing_rules_new SELECT id, category_id, request_type, department_id, assignee_user_id, first_response_hours, resolution_hours, active FROM routing_rules;
DROP TABLE routing_rules;
ALTER TABLE routing_rules_new RENAME TO routing_rules;
CREATE UNIQUE INDEX ux_routing_rules_active ON routing_rules(category_id, IFNULL(request_type, '*')) WHERE active = 1;

-- 6. requests: priority, feedback type, nullable author, token hash
CREATE TABLE requests_new (
    id                 INTEGER PRIMARY KEY,
    number             TEXT    NOT NULL UNIQUE,
    type               TEXT    NOT NULL CHECK (type IN ('problem', 'idea', 'feedback')),
    priority           TEXT    NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high')),
    category_id        INTEGER NOT NULL REFERENCES categories(id),
    title              TEXT    NOT NULL,
    body               TEXT    NOT NULL,
    proposed_solution  TEXT,
    author_id          INTEGER REFERENCES users(id),
    is_anonymous       INTEGER NOT NULL DEFAULT 0 CHECK (is_anonymous IN (0, 1)),
    anon_token_hash    TEXT    CHECK (anon_token_hash IS NULL OR length(anon_token_hash) = 64),
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
    CHECK (status <> 'duplicate' OR duplicate_of IS NOT NULL),
    -- CR2: an anonymous request has no author link, a named one always has one
    CHECK ((is_anonymous = 1 AND author_id IS NULL) OR (is_anonymous = 0 AND author_id IS NOT NULL)),
    CHECK (anon_token_hash IS NULL OR is_anonymous = 1)
);
INSERT INTO requests_new (id, number, type, priority, category_id, title, body, proposed_solution, author_id, is_anonymous, anon_token_hash,
        department_id, assignee_id, status, routing_source, first_response_due, resolution_due, first_responded_at, paused_at, closed_at,
        result_text, duplicate_of, theme_id, created_at, updated_at)
SELECT id, number, type, 'medium', category_id, title, body, proposed_solution,
        CASE WHEN is_anonymous = 1 THEN NULL ELSE author_id END, is_anonymous, NULL,
        department_id, assignee_id, status, routing_source, first_response_due, resolution_due, first_responded_at, paused_at, closed_at,
        result_text, duplicate_of, theme_id, created_at, updated_at
FROM requests;
DROP TABLE requests;
ALTER TABLE requests_new RENAME TO requests;
CREATE INDEX ix_requests_status_assignee ON requests(status, assignee_id);
CREATE INDEX ix_requests_department_status ON requests(department_id, status);
CREATE INDEX ix_requests_author ON requests(author_id);
CREATE INDEX ix_requests_first_response_due ON requests(first_response_due);
CREATE INDEX ix_requests_theme ON requests(theme_id);
CREATE INDEX ix_requests_created ON requests(created_at);
CREATE INDEX ix_requests_category ON requests(category_id);
-- POST /anonymous/* resolves the request by token hash
CREATE UNIQUE INDEX ux_requests_anon_token ON requests(anon_token_hash) WHERE anon_token_hash IS NOT NULL;
