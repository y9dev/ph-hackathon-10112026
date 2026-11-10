-- Reverts 0002 (manual, never applied automatically). `decision` and `in_progress` collapse to
-- `accepted`, `closed` to `declined`; `report` comments are deleted. Run with PRAGMA foreign_keys = OFF, as the runner does.
DELETE FROM comments WHERE kind = 'report';
UPDATE request_events SET from_status = 'declined' WHERE from_status = 'closed';
UPDATE request_events SET to_status = 'declined' WHERE to_status = 'closed';
UPDATE request_events SET from_status = 'accepted' WHERE from_status IN ('decision', 'in_progress');
UPDATE request_events SET to_status = 'accepted' WHERE to_status IN ('decision', 'in_progress');

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
    status             TEXT    NOT NULL CHECK (status IN ('new', 'in_review', 'needs_info', 'accepted', 'implemented', 'declined', 'duplicate')),
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
    CHECK (status NOT IN ('implemented', 'declined', 'duplicate') OR (result_text IS NOT NULL AND length(trim(result_text)) > 0 AND closed_at IS NOT NULL)),
    CHECK (status <> 'duplicate' OR duplicate_of IS NOT NULL)
);
INSERT INTO requests_old SELECT id, number, type, category_id, title, body, proposed_solution, author_id, is_anonymous, department_id, assignee_id,
        CASE WHEN status IN ('decision', 'in_progress') THEN 'accepted' WHEN status = 'closed' THEN 'declined' ELSE status END,
        routing_source, first_response_due, resolution_due, first_responded_at, paused_at, closed_at, result_text, duplicate_of,
        theme_id, created_at, updated_at FROM requests;
DROP TABLE requests;
ALTER TABLE requests_old RENAME TO requests;
CREATE INDEX ix_requests_status_assignee ON requests(status, assignee_id);
CREATE INDEX ix_requests_department_status ON requests(department_id, status);
CREATE INDEX ix_requests_author ON requests(author_id);
CREATE INDEX ix_requests_first_response_due ON requests(first_response_due);
CREATE INDEX ix_requests_theme ON requests(theme_id);
CREATE INDEX ix_requests_created ON requests(created_at);
CREATE INDEX ix_requests_category ON requests(category_id);

CREATE TABLE comments_old (
    id         INTEGER PRIMARY KEY,
    request_id INTEGER NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    author_id  INTEGER NOT NULL REFERENCES users(id),
    kind       TEXT    NOT NULL CHECK (kind IN ('comment', 'question', 'answer', 'internal')),
    body       TEXT    NOT NULL CHECK (length(body) BETWEEN 1 AND 4000),
    created_at INTEGER NOT NULL
);
INSERT INTO comments_old SELECT id, request_id, author_id, kind, body, created_at FROM comments;
DROP TABLE comments;
ALTER TABLE comments_old RENAME TO comments;
CREATE INDEX ix_comments_request ON comments(request_id);
