-- name: CreateUser :one
INSERT INTO users (username, password_hash, is_admin, created_at)
VALUES (sqlc.arg(username), sqlc.arg(password_hash), sqlc.arg(is_admin), sqlc.arg(created_at))
RETURNING *;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = sqlc.arg(username);

-- name: CountUsers :one
SELECT count(*) FROM users;
