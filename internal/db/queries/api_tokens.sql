-- name: CreateAPIToken :one
INSERT INTO api_tokens (user_id, name, token_hash, created_at)
VALUES (sqlc.arg(user_id), sqlc.arg(name), sqlc.arg(token_hash), sqlc.arg(created_at))
RETURNING id, name, created_at;

-- name: ListAPITokens :many
SELECT id, name, created_at FROM api_tokens
WHERE user_id = sqlc.arg(user_id)
ORDER BY created_at, id;

-- name: DeleteAPIToken :execrows
DELETE FROM api_tokens WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- GetAPITokenUser returns the user a token acts as. The user's admin flag
-- is read fresh, like a session's.
-- name: GetAPITokenUser :one
SELECT users.id, users.username, users.is_admin
FROM api_tokens
JOIN users ON users.id = api_tokens.user_id
WHERE api_tokens.token_hash = sqlc.arg(token_hash);
