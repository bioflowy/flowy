-- +goose Up
ALTER TABLE managed_files ADD COLUMN source_type TEXT NOT NULL DEFAULT '';

UPDATE managed_files
SET source_type = CASE
    WHEN run_id IS NULL THEN 'import'
    ELSE 'workflow'
END
WHERE source_type = '';

-- +goose Down
ALTER TABLE managed_files DROP COLUMN source_type;
