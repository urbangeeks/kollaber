-- name: GetNotificationPrefs :one
SELECT notify_on, COALESCE(notification_email, '') AS notification_email
FROM notification_prefs
WHERE user_id = $1 AND org_id = $2;

-- name: UpsertNotificationPrefs :exec
INSERT INTO notification_prefs (user_id, org_id, notify_on, notification_email, updated_at)
VALUES ($1, $2, $3, NULLIF($4, ''), NOW())
ON CONFLICT (user_id, org_id) DO UPDATE
SET notify_on = EXCLUDED.notify_on,
    notification_email = EXCLUDED.notification_email,
    updated_at = NOW();

-- name: GetOrgMembersToNotify :many
SELECT COALESCE(np.notification_email, u.email) AS email FROM users u
JOIN org_members om ON om.user_id = u.id
LEFT JOIN notification_prefs np ON np.user_id = u.id AND np.org_id = om.org_id
WHERE om.org_id = $1 AND np.notify_on @> ARRAY[$2::text];
