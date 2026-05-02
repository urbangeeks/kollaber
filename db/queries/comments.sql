-- name: CreateComment :one
INSERT INTO comments (event_id, user_id, body)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListCommentsByEvent :many
SELECT * FROM comments
WHERE event_id = $1
ORDER BY created_at ASC;
