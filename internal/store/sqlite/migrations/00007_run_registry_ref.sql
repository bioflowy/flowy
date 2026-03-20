-- +goose Up
ALTER TABLE runs ADD COLUMN registry_ref TEXT NOT NULL DEFAULT '';
ALTER TABLE runs ADD COLUMN registry_digest TEXT NOT NULL DEFAULT '';

-- +goose Down
CREATE TABLE runs_old AS
    SELECT id, wdl_path, inputs_json, state, outputs_json, created_at, updated_at, project_id
    FROM runs;
DROP TABLE runs;
ALTER TABLE runs_old RENAME TO runs;
