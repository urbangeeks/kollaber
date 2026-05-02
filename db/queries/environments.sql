-- name: ListEnvironments :many
SELECT * FROM environments ORDER BY created_at ASC;

-- name: CreateEnvironment :one
INSERT INTO environments (name, cluster_name)
VALUES ($1, $2)
RETURNING *;
