-- name: CreateUser :one
INSERT INTO users (id, email, password_hash)
VALUES (sqlc.arg(id), sqlc.arg(email), sqlc.arg(password_hash))
RETURNING id, email, password_hash, created_at, updated_at;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, created_at, updated_at
FROM users
WHERE lower(email) = lower(sqlc.arg(email));

-- name: GetUserByID :one
SELECT id, email, password_hash, created_at, updated_at
FROM users
WHERE id = sqlc.arg(id);
