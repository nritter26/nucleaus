PRAGMA foreign_keys = ON;

-- Enum-like category labels for snippets. Managed at /labels.
CREATE TABLE IF NOT EXISTS labels (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE COLLATE NOCASE,
    slug TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS snippets (
    id         INTEGER PRIMARY KEY,
    title      TEXT    NOT NULL,
    code       TEXT    NOT NULL DEFAULT '',
    notes      TEXT    NOT NULL DEFAULT '',
    lang       TEXT    NOT NULL DEFAULT '',
    label_id   INTEGER REFERENCES labels(id) ON DELETE SET NULL,
    source     TEXT    NOT NULL DEFAULT 'custom',
    created_at TEXT    NOT NULL,
    updated_at TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_snippets_label ON snippets (label_id);
CREATE INDEX IF NOT EXISTS idx_snippets_source ON snippets (source);
