-- +goose Up
-- Household accounts. Only what's needed so far; M2 (auth) adds the sign-in
-- columns.
--
-- Usernames are ASCII letters, digits, '.', '_' and '-': that makes NOCASE
-- (which only folds ASCII) a complete case-insensitive rule, and rules out
-- look-alike names, whitespace, and control characters.
--
-- AUTOINCREMENT: a deleted account's ID is never given to a new one, so
-- anything still holding the old ID can't end up acting on someone else.
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL COLLATE NOCASE UNIQUE
        CHECK (length(username) BETWEEN 1 AND 64)
        CHECK (username NOT GLOB '*[^A-Za-z0-9._-]*')
        -- length() and GLOB stop at a NUL byte; equal byte and character
        -- counts rule out NULs (and any non-ASCII) after it.
        CHECK (octet_length(username) = length(username)),
    created_at TEXT NOT NULL
        CHECK (created_at IS strftime('%Y-%m-%dT%H:%M:%SZ', created_at))
) STRICT;
