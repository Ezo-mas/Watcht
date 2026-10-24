-- +goose Up
-- Household accounts. Only what's needed so far; M2 (auth) adds the sign-in
-- columns.
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    username TEXT NOT NULL COLLATE NOCASE UNIQUE
        CHECK (length(username) BETWEEN 1 AND 64),
    created_at TEXT NOT NULL
) STRICT;
