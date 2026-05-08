-- name: GetNotificationPrefs :one
SELECT notify_on FROM notification_prefs
WHERE user_id = $1 AND org_id = $2;

-- name: UpsertNotificationPrefs :exec
INSERT INTO notification_prefs (user_id, org_id, notify_on, updated_at)
VALUES ($1, $2, $3, NOW())
ON CONFLICT (user_id, org_id) DO UPDATE
SET notify_on = EXCLUDED.notify_on, updated_at = NOW();

-- name: GetOrgMembersToNotify :many
SELECT u.email FROM users u
JOIN org_members om ON om.user_id = u.id
LEFT JOIN notification_prefs np ON np.user_id = u.id AND np.org_id = om.org_id
WHERE om.org_id = $1 AND np.notify_on @> ARRAY[$2::text];
