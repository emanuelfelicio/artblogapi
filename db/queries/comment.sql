-- name: CreateComment :one
INSERT INTO comments (id, post_id, author_id, content)
VALUES ($1, $2, $3, $4)
RETURNING id, post_id, author_id, content, created_at, updated_at, deleted_at;

-- name: GetPostByIDForComment :one
SELECT id
FROM posts
WHERE id = $1;

-- name: ListCommentsByPost :many
SELECT id, post_id, author_id, content, created_at, updated_at, deleted_at
FROM comments
WHERE post_id = $1 AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: GetCommentByIDForUpdate :one
SELECT id, post_id, author_id, content, created_at, updated_at, deleted_at
FROM comments
WHERE id = $1 AND post_id = $2
FOR UPDATE;

-- name: UpdateComment :one
UPDATE comments
SET content = $3, updated_at = now()
WHERE id = $1 AND post_id = $2 AND deleted_at IS NULL
RETURNING id, post_id, author_id, content, created_at, updated_at, deleted_at;

-- name: SoftDeleteComment :one
UPDATE comments
SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND post_id = $2 AND deleted_at IS NULL
RETURNING id, post_id, author_id, content, created_at, updated_at, deleted_at;
