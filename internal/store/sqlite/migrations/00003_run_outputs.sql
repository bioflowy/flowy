-- +goose Up
-- Adds outputs_json column to the runs table.
-- Stores JSON-encoded workflow/task outputs so handleOutputs can read from DB
-- instead of requiring a local outputs.json file (which does not exist in S3 mode).
ALTER TABLE runs ADD COLUMN outputs_json TEXT;

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; no-op for rollback.
