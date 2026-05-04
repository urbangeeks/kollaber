-- name: ListEnvironments :many
SELECT * FROM environments
WHERE org_id = $1
ORDER BY created_at ASC;

-- name: CreateEnvironment :one
INSERT INTO environments (org_id, name, cluster_name)
VALUES ($1, $2, $3)
RETURNING *;
