-- +goose Up
-- Adds inputs_json and outputs_json columns to the tasks table.
-- These columns support API-based input/output delivery: the server stores
-- task inputs here so daemons can retrieve them via TaskClaimResponse, and
-- daemons report outputs via TaskResultRequest which the server persists here.
ALTER TABLE tasks ADD COLUMN inputs_json TEXT;
ALTER TABLE tasks ADD COLUMN outputs_json TEXT;

-- +goose Down
-- SQLite does not support DROP COLUMN in older versions; no-op for rollback.
-- In practice this migration is forward-only for the project's SQLite schema.
