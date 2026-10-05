-- name: LikePost :exec
INSERT INTO post_likes (post_id, user_id)
VALUES ($1, $2)
ON CONFLICT (post_id, user_id) DO NOTHING;

-- name: UnlikePost :exec
DELETE FROM post_likes
WHERE post_id = $1 AND user_id = $2;
