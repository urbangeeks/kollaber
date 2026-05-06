-- name: ListOrgMembers :many
SELECT u.id, u.email, om.role, om.created_at
FROM org_members om
JOIN users u ON u.id = om.user_id
WHERE om.org_id = $1
ORDER BY om.created_at ASC;

-- name: UpdateOrgMemberRole :exec
UPDATE org_members SET role = $3 WHERE org_id = $1 AND user_id = $2;

-- name: DeleteOrgMember :exec
DELETE FROM org_members WHERE org_id = $1 AND user_id = $2;

-- name: ListPendingInvites :many
SELECT i.token, i.role, i.created_at, i.expires_at, u.email AS created_by_email
FROM invites i
JOIN users u ON u.id = i.created_by
WHERE i.org_id = $1 AND i.accepted_at IS NULL AND i.expires_at > NOW()
ORDER BY i.created_at DESC;

-- name: RevokeInvite :exec
DELETE FROM invites WHERE token = $1 AND org_id = $2;
