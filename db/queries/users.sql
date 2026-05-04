-- name: CreateUser :one
INSERT INTO users (email, password_hash)
VALUES ($1, $2)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByGitHubID :one
SELECT * FROM users WHERE github_id = $1;

-- name: CreateGitHubUser :one
INSERT INTO users (email, github_id)
VALUES ($1, $2)
RETURNING *;

-- name: LinkGitHubID :one
UPDATE users SET github_id = $1 WHERE email = $2 RETURNING *;
