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

-- name: GetNextProcessingJob :one
WITH next_job AS (
  SELECT uploads.id
  FROM uploads
  WHERE uploads.status = 'PROCESSING'
    AND uploads.retry_count < $1
    AND (uploads.next_retry_at IS NULL OR uploads.next_retry_at <= now())
    AND (uploads.heartbeat_at IS NULL OR uploads.heartbeat_at < now() - $2::interval)
  ORDER BY uploads.created_at ASC
  LIMIT 1
  FOR UPDATE SKIP LOCKED
)
UPDATE uploads u
SET heartbeat_at = now(),
    updated_at   = now()
FROM next_job nj
WHERE u.id = nj.id
RETURNING u.id, u.user_id, u.purpose, u.file_size, u.content_type, u.object_key, u.retry_count;



-- name: HeartbeatUploadProcessing :exec
UPDATE uploads
SET heartbeat_at = now(),
    updated_at   = now()
WHERE id = $1;

-- name: IncrementUploadRetry :exec
UPDATE uploads
SET retry_count   = retry_count + 1,
    next_retry_at = now() + $2::interval,
    heartbeat_at  = NULL,
    updated_at    = now()
WHERE id = $1;

-- name: UpdateUploadCompletion :exec
UPDATE uploads
SET object_key   = $2,
    content_type = $3,
    status       = $4,
    heartbeat_at = NULL,
    updated_at   = now()
WHERE id = $1;

