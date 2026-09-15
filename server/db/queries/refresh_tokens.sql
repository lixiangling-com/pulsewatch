-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at)
VALUES (sqlc.arg(id), sqlc.arg(user_id), sqlc.arg(token_hash), sqlc.arg(expires_at))
RETURNING id, user_id, token_hash, expires_at, revoked_at, created_at;

-- name: GetRefreshTokenForUpdate :one
SELECT id, user_id, token_hash, expires_at, revoked_at, created_at
FROM refresh_tokens
WHERE token_hash = sqlc.arg(token_hash)
FOR UPDATE;

-- name: RevokeRefreshToken :execrows
UPDATE refresh_tokens
SET revoked_at = COALESCE(revoked_at, now())
WHERE id = sqlc.arg(id) AND revoked_at IS NULL;
