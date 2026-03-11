-- +goose Up
-- Creates the core schema for flowy workflow run and task persistence.
-- shard_index and last_heartbeat_at are included from day one — retrofitting
-- these columns in SQLite (which lacks ALTER TABLE ADD COLUMN with defaults on
-- existing rows) would be painful.

CREATE TABLE IF NOT EXISTS runs (
    id          TEXT     PRIMARY KEY,
    wdl_path    TEXT     NOT NULL,
    inputs_json TEXT     NOT NULL,
    state       TEXT     NOT NULL DEFAULT 'PENDING',
    created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS tasks (
    id                 INTEGER  PRIMARY KEY AUTOINCREMENT,
    run_id             TEXT     NOT NULL REFERENCES runs(id),
    task_name          TEXT     NOT NULL,
    state              TEXT     NOT NULL DEFAULT 'PENDING',
    shard_index        INTEGER  NOT NULL DEFAULT 0,
    daemon_id          TEXT,
    last_heartbeat_at  DATETIME,
    created_at         DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at         DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_tasks_run_id ON tasks(run_id);
CREATE INDEX IF NOT EXISTS idx_tasks_state  ON tasks(state);

-- +goose Down
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS runs;
