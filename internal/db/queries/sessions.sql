-- name: CreateSession :one
INSERT INTO sessions (token_hash, user_id, created_at, expires_at)
VALUES (sqlc.arg(token_hash), sqlc.arg(user_id), sqlc.arg(created_at), sqlc.arg(expires_at))
RETURNING id;

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

-- DeleteExpiredSessions returns the number of rows deleted, for logging.
-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at <= sqlc.arg(now);

-- DeleteUserSessions ends every session of a user except keep_id (pass 0
-- to end them all).
-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = sqlc.arg(user_id) AND id != sqlc.arg(keep_id);
