-- 0001_init: core schema for Modus Voice (SQLite dialect, portable to PostgreSQL with type tweaks).
-- All timestamps are INTEGER unix seconds (UTC).

CREATE TABLE departments (
    id           INTEGER PRIMARY KEY,
    name         TEXT    NOT NULL UNIQUE CHECK (length(name) BETWEEN 1 AND 120),
    head_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    email         TEXT    NOT NULL UNIQUE COLLATE NOCASE CHECK (length(email) BETWEEN 3 AND 254),
    name          TEXT    NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    title         TEXT    NOT NULL DEFAULT '',
    password_hash TEXT    NOT NULL,
    role          TEXT    NOT NULL CHECK (role IN ('employee', 'manager', 'admin')),
    department_id INTEGER REFERENCES departments(id) ON DELETE SET NULL,
    active        INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    external_id   TEXT,
    source        TEXT    NOT NULL DEFAULT 'local',
    created_at    INTEGER NOT NULL
);

CREATE TABLE sessions (
    token_hash TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE INDEX ix_sessions_user ON sessions(user_id);

CREATE TABLE categories (
    id          INTEGER PRIMARY KEY,
    name        TEXT    NOT NULL UNIQUE CHECK (length(name) BETWEEN 1 AND 80),
    description TEXT    NOT NULL DEFAULT '',
    icon        TEXT    NOT NULL DEFAULT 'circle',
    sort        INTEGER NOT NULL DEFAULT 0,
    active      INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1))
);

CREATE TABLE routing_rules (
    id                   INTEGER PRIMARY KEY,
    category_id          INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    request_type         TEXT    CHECK (request_type IS NULL OR request_type IN ('problem', 'idea')),
    department_id        INTEGER NOT NULL REFERENCES departments(id),
    assignee_user_id     INTEGER REFERENCES users(id) ON DELETE SET NULL,
    first_response_hours INTEGER NOT NULL CHECK (first_response_hours BETWEEN 1 AND 8760),
    resolution_hours     INTEGER NOT NULL CHECK (resolution_hours BETWEEN 1 AND 8760),
    active               INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1))
);
-- R5: at most one ACTIVE rule per (category, type); type NULL means "any type".
CREATE UNIQUE INDEX ux_routing_rules_active ON routing_rules(category_id, IFNULL(request_type, '*')) WHERE active = 1;

CREATE TABLE themes (
    id         INTEGER PRIMARY KEY,
    title      TEXT    NOT NULL UNIQUE COLLATE NOCASE CHECK (length(title) BETWEEN 1 AND 140),
    created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at INTEGER NOT NULL
);

-- Counter rows (request numbers MV-0001...) updated in the same transaction as the insert.
CREATE TABLE counters (
    name  TEXT    PRIMARY KEY,
    value INTEGER NOT NULL
);
INSERT INTO counters(name, value) VALUES ('request', 0);

CREATE TABLE requests (
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
    -- Closing requires a recorded result (TZ 2.5 / AC-03).
    CHECK (status NOT IN ('implemented', 'declined', 'duplicate') OR (result_text IS NOT NULL AND length(trim(result_text)) > 0 AND closed_at IS NOT NULL)),
    CHECK (status <> 'duplicate' OR duplicate_of IS NOT NULL)
);
CREATE INDEX ix_requests_status_assignee ON requests(status, assignee_id);   -- inbox "assigned to me"
CREATE INDEX ix_requests_department_status ON requests(department_id, status); -- manager inbox by department
CREATE INDEX ix_requests_author ON requests(author_id);                       -- "my requests"
CREATE INDEX ix_requests_first_response_due ON requests(first_response_due);  -- overdue ticker / deadline sort
CREATE INDEX ix_requests_theme ON requests(theme_id);                         -- themes counts
CREATE INDEX ix_requests_created ON requests(created_at);                     -- analytics period filter
CREATE INDEX ix_requests_category ON requests(category_id);                   -- routing preview similar count

CREATE TABLE comments (
    id         INTEGER PRIMARY KEY,
    request_id INTEGER NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    author_id  INTEGER NOT NULL REFERENCES users(id),
    kind       TEXT    NOT NULL CHECK (kind IN ('comment', 'question', 'answer', 'internal')),
    body       TEXT    NOT NULL CHECK (length(body) BETWEEN 1 AND 4000),
    created_at INTEGER NOT NULL
);
CREATE INDEX ix_comments_request ON comments(request_id);

-- Immutable audit trail / timeline. meta carries a small machine key (e.g. overdue kind, routing source).
CREATE TABLE request_events (
    id          INTEGER PRIMARY KEY,
    request_id  INTEGER NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    actor_id    INTEGER REFERENCES users(id),
    kind        TEXT    NOT NULL,
    from_status TEXT,
    to_status   TEXT,
    text        TEXT,
    meta        TEXT,
    created_at  INTEGER NOT NULL
);
CREATE INDEX ix_request_events_request_kind ON request_events(request_id, kind);

CREATE TABLE attachments (
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
CREATE INDEX ix_attachments_request ON attachments(request_id);

CREATE TABLE notifications (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind       TEXT    NOT NULL,
    request_id INTEGER NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    title      TEXT    NOT NULL,
    body       TEXT    NOT NULL,
    read_at    INTEGER,
    created_at INTEGER NOT NULL
);
CREATE INDEX ix_notifications_user_read ON notifications(user_id, read_at);

-- Transactional outbox schema kept for the future webhook/Bitrix24 dispatcher (R1: dispatcher not built).
CREATE TABLE outbox (
    id              INTEGER PRIMARY KEY,
    topic           TEXT    NOT NULL,
    payload_json    TEXT    NOT NULL,
    status          TEXT    NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
    attempts        INTEGER NOT NULL DEFAULT 0,
    next_attempt_at INTEGER NOT NULL,
    created_at      INTEGER NOT NULL
);
