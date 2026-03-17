-- +goose Up
-- Creates the users table for JWT-based authentication and the server_config
-- key-value table for storing auto-generated secrets (JWTSecret, DaemonToken).

CREATE TABLE IF NOT EXISTS users (
    id            TEXT     PRIMARY KEY,
    username      TEXT     NOT NULL UNIQUE,
    password_hash TEXT     NOT NULL,
    display_name  TEXT     NOT NULL DEFAULT '',
    email         TEXT     NOT NULL DEFAULT '',
    global_role   TEXT     NOT NULL DEFAULT 'user',
    is_active     INTEGER  NOT NULL DEFAULT 1,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    updated_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);

CREATE TABLE IF NOT EXISTS server_config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS server_config;
DROP TABLE IF EXISTS users;
