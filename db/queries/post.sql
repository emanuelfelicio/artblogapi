-- name: CreatePost :one
INSERT INTO posts (id, author_id, title, content)
VALUES ($1, $2, $3, $4)
RETURNING id, author_id, title, content, created_at, updated_at;

-- name: GetPostByIDForUpdate :one
SELECT id, author_id, title, content, created_at, updated_at
FROM posts
WHERE id = $1
FOR UPDATE;

-- name: GetPostWithImagesByID :many
SELECT p.id, p.author_id, p.title, p.content, p.created_at, p.updated_at,
       pi.upload_id, pi.position, pi.created_at AS image_created_at,
       u.object_key, u.content_type
FROM posts p
LEFT JOIN post_images pi ON p.id = pi.post_id
LEFT JOIN uploads u ON pi.upload_id = u.id
WHERE p.id = $1
ORDER BY pi.position ASC;

-- name: UpdatePost :one
UPDATE posts
SET title = COALESCE(sqlc.narg('title'), title),
    content = COALESCE(sqlc.narg('content'), content),
    updated_at = now()
WHERE id = $1
RETURNING id, author_id, title, content, created_at, updated_at;

-- name: DeletePost :exec
DELETE FROM posts
WHERE id = $1;

-- name: BatchInsertPostImages :exec
INSERT INTO post_images (post_id, upload_id, position)
SELECT $1, unnest($2::uuid[]), unnest($3::smallint[]);

-- name: DeletePostImages :exec
DELETE FROM post_images
WHERE post_id = $1 AND upload_id = ANY($2::uuid[]);

-- name: UpdatePostImagePositions :exec
UPDATE post_images AS pi
SET position = u.pos
FROM (SELECT unnest($2::uuid[]) AS upload_id, unnest($3::smallint[]) AS pos) AS u
WHERE pi.post_id = $1 AND pi.upload_id = u.upload_id;

-- name: DeletePostImagesByPostID :many
DELETE FROM post_images
WHERE post_id = $1
RETURNING upload_id;

-- name: GetPostImagesByPostIDs :many
SELECT pi.post_id, pi.upload_id, pi.position, pi.created_at,
       u.object_key, u.content_type
FROM post_images pi
JOIN uploads u ON pi.upload_id = u.id
WHERE pi.post_id = ANY($1::uuid[])
ORDER BY pi.post_id, pi.position ASC;

-- name: ListRecentPosts :many
SELECT id, author_id, title, content, created_at, updated_at
FROM posts
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListPostsByAuthor :many
SELECT id, author_id, title, content, created_at, updated_at
FROM posts
WHERE author_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;
