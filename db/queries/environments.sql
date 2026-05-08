-- name: GetEnvironmentByID :one
SELECT * FROM environments WHERE id = $1;

-- name: GetEnvStatsByOrg :many
SELECT
  e.environment_id,
  COUNT(*) FILTER (WHERE e.type = 'deploy')::int AS deploys,
  COUNT(*) FILTER (WHERE e.type = 'alert')::int  AS alerts,
  COUNT(*) FILTER (WHERE e.type = 'note')::int   AS notes,
  MAX(e.timestamp) AS last_event_at
FROM events e
JOIN environments env ON env.id = e.environment_id
WHERE env.org_id = $1
GROUP BY e.environment_id;

-- name: ListEnvironments :many
SELECT * FROM environments
WHERE org_id = $1
ORDER BY created_at ASC;

-- name: CreateEnvironment :one
INSERT INTO environments (org_id, name, cluster_name)
VALUES ($1, $2, $3)
RETURNING *;

-- name: DeleteEnvironment :exec
DELETE FROM environments
WHERE id = $1 AND org_id = $2;

-- name: UpdateEnvironment :one
UPDATE environments
SET name = $3, cluster_name = $4
WHERE id = $1 AND org_id = $2
RETURNING *;
