-- name: CreateOrg :one
INSERT INTO orgs (name, slug)
VALUES ($1, $2)
RETURNING *;

-- name: CreateOrgMember :exec
INSERT INTO org_members (org_id, user_id, role)
VALUES ($1, $2, $3);

-- name: GetOrgByUserID :one
SELECT o.* FROM orgs o
JOIN org_members om ON om.org_id = o.id
WHERE om.user_id = $1
LIMIT 1;

-- name: ListOrgsByUserID :many
SELECT o.* FROM orgs o
JOIN org_members om ON om.org_id = o.id
WHERE om.user_id = $1
ORDER BY o.created_at ASC;

-- name: GetOrgMember :one
SELECT * FROM org_members WHERE org_id = $1 AND user_id = $2;
