-- name: CreateEvent :one
INSERT INTO events (type, service, environment_id, metadata, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListEventsByEnvironment :many
SELECT e.* FROM events e
JOIN environments env ON env.id = e.environment_id
WHERE e.environment_id = $1 AND env.org_id = $2
ORDER BY e.timestamp DESC
LIMIT $3 OFFSET $4;

-- name: ListEventsByOrg :many
SELECT e.* FROM events e
JOIN environments env ON env.id = e.environment_id
WHERE env.org_id = $1
ORDER BY e.timestamp DESC
LIMIT $2 OFFSET $3;

-- name: ListServicesByEnvironment :many
SELECT DISTINCT e.service FROM events e
JOIN environments env ON env.id = e.environment_id
WHERE e.environment_id = $1 AND env.org_id = $2
ORDER BY e.service;
