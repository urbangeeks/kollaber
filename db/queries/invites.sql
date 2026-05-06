-- name: CreateInvite :one
INSERT INTO invites (org_id, created_by, token, role)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetInviteByToken :one
SELECT i.id, i.org_id, i.token, i.role, i.expires_at, i.accepted_at, i.created_at, o.name AS org_name
FROM invites i
JOIN orgs o ON o.id = i.org_id
WHERE i.token = $1;

-- name: AcceptInvite :exec
UPDATE invites SET accepted_at = NOW() WHERE token = $1;
