-- +goose Up
-- Creates registry tables for WDL content-addressed registry support.
-- registry_entries: named slots with kind classification (tool/workflow).
-- registry_versions: immutable content-addressed WDL blobs.
-- registry_tags: mutable name pointers to specific versions.

CREATE TABLE IF NOT EXISTS registry_entries (
    id         TEXT     PRIMARY KEY,
    project_id TEXT     NOT NULL REFERENCES projects(id),
    name       TEXT     NOT NULL,
    kind       TEXT     NOT NULL,
    is_active  INTEGER  NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE (project_id, name)
);
CREATE INDEX IF NOT EXISTS idx_registry_entries_project ON registry_entries(project_id);

CREATE TABLE IF NOT EXISTS registry_versions (
    id          TEXT     PRIMARY KEY,
    entry_id    TEXT     NOT NULL REFERENCES registry_entries(id),
    digest      TEXT     NOT NULL,
    wdl_content TEXT     NOT NULL,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE (entry_id, digest)
);
CREATE INDEX IF NOT EXISTS idx_registry_versions_entry ON registry_versions(entry_id);
CREATE INDEX IF NOT EXISTS idx_registry_versions_digest ON registry_versions(digest);

CREATE TABLE IF NOT EXISTS registry_tags (
    id         TEXT     PRIMARY KEY,
    entry_id   TEXT     NOT NULL REFERENCES registry_entries(id),
    name       TEXT     NOT NULL,
    version_id TEXT     NOT NULL REFERENCES registry_versions(id),
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE (entry_id, name)
);
CREATE INDEX IF NOT EXISTS idx_registry_tags_entry ON registry_tags(entry_id);

-- +goose Down
DROP TABLE IF EXISTS registry_tags;
DROP TABLE IF EXISTS registry_versions;
DROP TABLE IF EXISTS registry_entries;
