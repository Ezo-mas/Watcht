-- name: CreateUser :one
INSERT INTO users (username, password_hash, is_admin, created_at)
VALUES (sqlc.arg(username), sqlc.arg(password_hash), sqlc.arg(is_admin), sqlc.arg(created_at))
RETURNING *;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = sqlc.arg(username);

-- name: CountUsers :one
SELECT count(*) FROM users;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = sqlc.arg(id);

-- name: ListUsers :many
SELECT * FROM users ORDER BY username;

-- name: DeleteUser :execrows
DELETE FROM users WHERE id = sqlc.arg(id);

-- name: SetPasswordHash :execrows
UPDATE users SET password_hash = sqlc.arg(password_hash) WHERE id = sqlc.arg(id);

-- ReplacePasswordHash changes a password only if it is still the one the
-- caller verified, so a concurrent change or reset can't be overwritten.
-- name: ReplacePasswordHash :execrows
UPDATE users SET password_hash = sqlc.arg(new_hash)
WHERE id = sqlc.arg(id) AND password_hash = sqlc.arg(old_hash);
