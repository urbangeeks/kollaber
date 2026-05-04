-- name: ListOrgsWithStats :many
SELECT
    o.id,
    o.name,
    o.slug,
    o.created_at,
    COUNT(DISTINCT om.user_id)::int AS member_count,
    COUNT(DISTINCT e.id)::int       AS env_count
FROM orgs o
LEFT JOIN org_members om ON om.org_id = o.id
LEFT JOIN environments e ON e.org_id = o.id
GROUP BY o.id
ORDER BY o.created_at DESC;
