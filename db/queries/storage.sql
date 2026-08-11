-- name: CreateUpload :one
INSERT INTO uploads (id, user_id, object_key, status, purpose, file_size, content_type)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id;

-- name: GetUploadByID :one
SELECT id, user_id, object_key, status, purpose, file_size, content_type, failure_reason, created_at, updated_at
FROM uploads
WHERE id = $1;

-- name: UpdateUploadStatus :exec
UPDATE uploads
SET status = $2, updated_at = now()
WHERE id = $1;

-- name: UpdateUploadObjectKeyAndStatus :exec
UPDATE uploads
SET object_key = $2, status = $3, updated_at = now()
WHERE id = $1;

-- name: RejectUpload :exec
UPDATE uploads
SET status = 'REJECTED', failure_reason = $2, updated_at = now()
WHERE id = $1;

-- name: GetUploadForUpdate :one
SELECT id, user_id, status, purpose, object_key
FROM uploads
WHERE id = $1 FOR UPDATE;

-- name: GetUserForUpdate :one
SELECT id, avatar_upload_id, banner_upload_id
FROM users
WHERE id = $1 FOR UPDATE;
