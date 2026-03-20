-- +goose Up
CREATE TABLE IF NOT EXISTS provenance (
    id                  TEXT     PRIMARY KEY,
    run_id              TEXT     NOT NULL REFERENCES runs(id),
    user_id             TEXT     NOT NULL REFERENCES users(id),
    registry_entry_id   TEXT     NOT NULL REFERENCES registry_entries(id),
    registry_version_id TEXT     NOT NULL REFERENCES registry_versions(id),
    registry_ref        TEXT     NOT NULL DEFAULT '',
    registry_digest     TEXT     NOT NULL DEFAULT '',
    created_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at          DATETIME NOT NULL DEFAULT (datetime('now')),
    UNIQUE (run_id)
);
CREATE INDEX IF NOT EXISTS idx_provenance_run      ON provenance(run_id);
CREATE INDEX IF NOT EXISTS idx_provenance_entry    ON provenance(registry_entry_id);
CREATE INDEX IF NOT EXISTS idx_provenance_user     ON provenance(user_id);

-- +goose Down
DROP TABLE IF EXISTS provenance;
