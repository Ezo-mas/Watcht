-- name: CreateSession :exec
INSERT INTO sessions (token_hash, user_id, created_at, expires_at)
VALUES (sqlc.arg(token_hash), sqlc.arg(user_id), sqlc.arg(created_at), sqlc.arg(expires_at));

-- GetSession returns an unexpired session and its user. Timestamps share
-- one fixed layout, so comparing them as text compares the times.
-- name: GetSession :one
SELECT
    sessions.id,
    sessions.expires_at,
    users.id AS user_id,
    users.username,
    users.is_admin
FROM sessions
JOIN users ON users.id = sessions.user_id
WHERE sessions.token_hash = sqlc.arg(token_hash)
    AND sessions.expires_at > sqlc.arg(now);

-- name: ExtendSession :exec
UPDATE sessions SET expires_at = sqlc.arg(expires_at) WHERE id = sqlc.arg(id);

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = sqlc.arg(token_hash);

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at <= sqlc.arg(now);
