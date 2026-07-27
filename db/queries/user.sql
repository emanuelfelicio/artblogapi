-- name: GetPublicUserProfileByUsername :one
SELECT
    u.id, u.username, u.email, u.display_name, u.bio,
    u.avatar_upload_id, u.banner_upload_id,
    u.is_active, u.created_at, u.updated_at,
    up_a.object_key AS avatar_key,
    up_b.object_key AS banner_key
FROM users u
LEFT JOIN uploads up_a ON up_a.id = u.avatar_upload_id AND up_a.status = 'COMPLETED'
LEFT JOIN uploads up_b ON up_b.id = u.banner_upload_id AND up_b.status = 'COMPLETED'
WHERE u.username = $1 AND u.is_active = true;

-- name: GetMyUserProfileByID :one
SELECT
    u.id, u.username, u.email, u.display_name, u.bio,
    u.avatar_upload_id, u.banner_upload_id,
    u.is_active, u.created_at, u.updated_at,
    up_a.object_key AS avatar_key,
    up_b.object_key AS banner_key
FROM users u
LEFT JOIN uploads up_a ON up_a.id = u.avatar_upload_id AND up_a.status = 'COMPLETED'
LEFT JOIN uploads up_b ON up_b.id = u.banner_upload_id AND up_b.status = 'COMPLETED'
WHERE u.id = $1;

-- name: UpdateUserProfile :one
UPDATE users
SET
    display_name = coalesce($2, display_name),
    bio         = coalesce($3, bio),
    updated_at   = now()
WHERE id = $1
RETURNING id;

-- name: UpdateUserAvatar :exec
UPDATE users SET avatar_upload_id = $2, updated_at = now() WHERE id = $1;

-- name: UpdateUserBanner :exec
UPDATE users SET banner_upload_id = $2, updated_at = now() WHERE id = $1;

-- name: FindCompletedUploadByOwner :one
SELECT id FROM uploads
WHERE id = $1 AND user_id = $2 AND status = 'COMPLETED';