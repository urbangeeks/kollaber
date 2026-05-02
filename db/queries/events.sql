-- name: CreateEvent :one
INSERT INTO events (type, service, environment_id, metadata)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListEvents :many
SELECT * FROM events
ORDER BY timestamp DESC
LIMIT $1 OFFSET $2;

-- name: ListEventsByEnvironment :many
SELECT * FROM events
WHERE environment_id = $1
ORDER BY timestamp DESC
LIMIT $2 OFFSET $3;
