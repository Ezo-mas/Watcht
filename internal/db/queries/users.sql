-- name: CreateUser :one
INSERT INTO users (username, created_at)
VALUES (sqlc.arg(username), sqlc.arg(created_at))
RETURNING *;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = sqlc.arg(username);

-- name: CountUsers :one
SELECT count(*) FROM users;
