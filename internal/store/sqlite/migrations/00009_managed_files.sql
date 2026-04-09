-- +goose Up
CREATE TABLE IF NOT EXISTS managed_files (
    id          TEXT     PRIMARY KEY,
    project_id  TEXT     NOT NULL REFERENCES projects(id),
    run_id      TEXT     REFERENCES runs(id),
    path        TEXT     NOT NULL,
    file_type   TEXT     NOT NULL DEFAULT '',
    label       TEXT     NOT NULL DEFAULT '',
    size        INTEGER  NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_managed_files_project ON managed_files(project_id);
CREATE INDEX IF NOT EXISTS idx_managed_files_run     ON managed_files(run_id);

-- +goose Down
DROP TABLE IF EXISTS managed_files;
