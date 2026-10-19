CREATE TABLE meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE users (
    id               INTEGER PRIMARY KEY,
    email            TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash    TEXT NOT NULL,
    timezone         TEXT NOT NULL DEFAULT 'UTC',
    last_briefing_on TEXT,
    last_wrapup_on   TEXT,
    created_at       TEXT NOT NULL
);

CREATE TABLE working_hours (
    user_id   INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    weekday   INTEGER NOT NULL CHECK (weekday BETWEEN 0 AND 6),
    start_min INTEGER NOT NULL,
    end_min   INTEGER NOT NULL,
    PRIMARY KEY (user_id, weekday),
    CHECK (start_min >= 0 AND end_min <= 1440 AND start_min < end_min)
);

CREATE TABLE refresh_tokens (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TEXT NOT NULL,
    revoked_at TEXT
);

CREATE TABLE access_tokens (
    id           INTEGER PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    prefix       TEXT NOT NULL,
    last_used_at TEXT,
    created_at   TEXT NOT NULL,
    revoked_at   TEXT
);

CREATE TABLE namespaces (
    id            INTEGER PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    exclusive     INTEGER NOT NULL DEFAULT 0,
    default_value TEXT,
    UNIQUE (user_id, name)
);

CREATE TABLE namespace_values (
    namespace_id INTEGER NOT NULL REFERENCES namespaces (id) ON DELETE CASCADE,
    value        TEXT NOT NULL,
    position     INTEGER NOT NULL,
    terminal     INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (namespace_id, value)
);

CREATE TABLE tags (
    id       INTEGER PRIMARY KEY,
    user_id  INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name     TEXT NOT NULL,
    color    TEXT,
    pinned   INTEGER NOT NULL DEFAULT 0,
    position INTEGER NOT NULL DEFAULT 0,
    UNIQUE (user_id, name)
);

CREATE TABLE tasks (
    id          INTEGER PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    notes       TEXT NOT NULL DEFAULT '',
    due_date    TEXT,
    remind_at   TEXT,
    reminded_at TEXT,
    closed_at   TEXT,
    deleted_at  TEXT,
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);

CREATE INDEX tasks_user ON tasks (user_id, deleted_at, closed_at);
CREATE INDEX tasks_remind ON tasks (remind_at) WHERE remind_at IS NOT NULL AND reminded_at IS NULL;

CREATE TABLE task_tags (
    task_id INTEGER NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    tag_id  INTEGER NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, tag_id)
);

CREATE INDEX task_tags_tag ON task_tags (tag_id);

CREATE TABLE time_entries (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    task_id    INTEGER NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    started_at TEXT NOT NULL,
    stopped_at TEXT,
    note       TEXT NOT NULL DEFAULT ''
);

CREATE INDEX time_entries_user ON time_entries (user_id, started_at);
CREATE UNIQUE INDEX time_entries_running ON time_entries (task_id) WHERE stopped_at IS NULL;

CREATE TABLE views (
    id       INTEGER PRIMARY KEY,
    user_id  INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name     TEXT NOT NULL,
    query    TEXT NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    builtin  INTEGER NOT NULL DEFAULT 0,
    UNIQUE (user_id, name)
);
