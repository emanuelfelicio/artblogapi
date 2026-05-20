-- name: CreateUser :one
INSERT INTO users (
    id, username, email, password_hash
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1 LIMIT 1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1 LIMIT 1;

-- name: CheckEmailExists :one
SELECT EXISTS(SELECT 1 FROM users WHERE email = $1);

-- name: CheckUsernameExists :one
SELECT EXISTS(SELECT 1 FROM users WHERE username = $1);

-- name: CheckEmailUsername :one
SELECT
    EXISTS(SELECT 1 FROM users u WHERE u.email = $1) AS email_exists,
    EXISTS(SELECT 1 FROM users u WHERE u.username = $2) AS username_exists;
