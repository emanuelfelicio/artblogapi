-- name: CreateSession :exec
INSERT INTO sessions (
    id, user_id, expires_at, created_at, revoked, user_agent, ip, device_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
);

-- name: FindSessionByID :one
SELECT * FROM sessions
WHERE id = $1 LIMIT 1;

-- name: FindSessionByIDForUpdate :one
SELECT * FROM sessions
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: RevokeSessionByID :exec
UPDATE sessions
SET revoked = TRUE
WHERE id = $1;

-- name: RevokeAllSessionsByUserID :exec
UPDATE sessions
SET revoked = TRUE
WHERE user_id = $1;
