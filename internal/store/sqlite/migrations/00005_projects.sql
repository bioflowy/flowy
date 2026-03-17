-- +goose Up
-- Creates the projects and project_members tables for project workspace support.
-- Adds project_id column to runs for associating runs with a project.
-- Existing run data is cleared because project_id enforcement is required at the
-- application layer and there is no valid project to migrate old runs to.

CREATE TABLE IF NOT EXISTS projects (
    id          TEXT     PRIMARY KEY,
    slug        TEXT     NOT NULL UNIQUE,
    description TEXT     NOT NULL DEFAULT '',
    is_active   INTEGER  NOT NULL DEFAULT 1,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_projects_slug ON projects(slug);

CREATE TABLE IF NOT EXISTS project_members (
    project_id TEXT     NOT NULL REFERENCES projects(id),
    user_id    TEXT     NOT NULL REFERENCES users(id),
    role       TEXT     NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at DATETIME NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (project_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_project_members_user ON project_members(user_id);

-- Clear existing run data before adding the project_id column.
-- project_id is nullable in SQL but enforced non-empty at the application layer.
DELETE FROM tasks;
DELETE FROM runs;

ALTER TABLE runs ADD COLUMN project_id TEXT REFERENCES projects(id);

-- +goose Down
-- SQLite does not support DROP COLUMN directly in older versions, so we recreate
-- the runs table without the project_id column.
CREATE TABLE runs_old AS SELECT id, wdl_path, inputs_json, state, outputs_json, created_at, updated_at FROM runs;
DROP TABLE runs;
ALTER TABLE runs_old RENAME TO runs;

DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS projects;
