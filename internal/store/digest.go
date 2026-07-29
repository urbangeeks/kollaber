package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// DigestEnvironment is one environment's week: what shipped, what broke, what
// fired. Counted per type rather than as a total because "12 events" says
// nothing a person can act on, while "9 deploys, 1 rollback" says quite a lot.
type DigestEnvironment struct {
	Name          string
	Deploys       int64
	FailedDeploys int64
	Rollbacks     int64
	Alerts        int64
	Total         int64
}

// DigestThread is an event that drew discussion during the week. These are the
// part of the digest worth reading: a busy comment thread is where the team
// worked something out, and it is the piece least likely to be remembered.
type DigestThread struct {
	EventID         pgtype.UUID
	Type            string
	Service         string
	EnvironmentName string
	Comments        int64
	Timestamp       pgtype.Timestamptz
}

// DigestOrg names an org with at least one member subscribed to the digest.
type DigestOrg struct {
	ID   pgtype.UUID
	Name string
}

// ClaimWeeklyDigest reserves the right to send one org's digest for one week.
//
// It reports true exactly once per (org, week) across every replica and every
// rerun: the insert either takes the row or conflicts with the replica that got
// there first. Callers must claim before sending, never after, or two pods
// racing on the same tick would both mail the org.
func (q *Queries) ClaimWeeklyDigest(ctx context.Context, orgID pgtype.UUID, weekStart time.Time) (bool, error) {
	var claimed bool
	err := q.db.QueryRow(ctx, `
		INSERT INTO digest_sends (org_id, week_start)
		VALUES ($1, $2)
		ON CONFLICT (org_id, week_start) DO NOTHING
		RETURNING true`,
		orgID, weekStart,
	).Scan(&claimed)
	if err != nil {
		// No row came back, so another replica holds the claim. That is the
		// ordinary outcome on every replica but one, not an error.
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return claimed, nil
}

// ReleaseWeeklyDigest drops a claim so a later tick can try again.
//
// Used when the send itself failed. Holding a claim for a digest that was never
// delivered would cost the org the whole week, and the send is a single call to
// the mail provider — if it reports failure, nothing went out.
func (q *Queries) ReleaseWeeklyDigest(ctx context.Context, orgID pgtype.UUID, weekStart time.Time) error {
	_, err := q.db.Exec(ctx,
		`DELETE FROM digest_sends WHERE org_id = $1 AND week_start = $2`, orgID, weekStart)
	return err
}

// RecordDigestRecipients notes how many addresses a delivered digest reached.
func (q *Queries) RecordDigestRecipients(ctx context.Context, orgID pgtype.UUID, weekStart time.Time, n int) error {
	_, err := q.db.Exec(ctx,
		`UPDATE digest_sends SET recipients = $3 WHERE org_id = $1 AND week_start = $2`,
		orgID, weekStart, n)
	return err
}

// ListDigestOrgs returns the orgs with at least one member subscribed.
//
// Filtering here rather than per org keeps an install full of orgs that never
// opted in down to one cheap query per tick.
func (q *Queries) ListDigestOrgs(ctx context.Context) ([]DigestOrg, error) {
	rows, err := q.db.Query(ctx, `
		SELECT DISTINCT o.id, o.name
		FROM orgs o
		JOIN notification_prefs np ON np.org_id = o.id
		WHERE np.notify_on @> ARRAY['digest']::text[]
		ORDER BY o.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DigestOrg
	for rows.Next() {
		var o DigestOrg
		if err := rows.Scan(&o.ID, &o.Name); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ListDigestRecipients returns the addresses subscribed to one org's digest.
//
// Prefers the notification_email override over the login address, so someone
// who signed up with a personal address and routes mail elsewhere gets it where
// they asked. Membership is re-checked here: a preference row outlives the
// membership it was written for, and a former member must not keep receiving
// the org's weekly summary.
func (q *Queries) ListDigestRecipients(ctx context.Context, orgID pgtype.UUID) ([]string, error) {
	rows, err := q.db.Query(ctx, `
		SELECT DISTINCT COALESCE(NULLIF(np.notification_email, ''), u.email)
		FROM notification_prefs np
		JOIN org_members om ON om.org_id = np.org_id AND om.user_id = np.user_id
		JOIN users u ON u.id = np.user_id
		WHERE np.org_id = $1
		  AND np.notify_on @> ARRAY['digest']::text[]`,
		orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		out = append(out, email)
	}
	return out, rows.Err()
}

// DigestEnvironments returns each environment's counts for the window, busiest
// first. Environments with no activity are included so a quiet week reads as
// "nothing happened here" rather than as a missing section.
func (q *Queries) DigestEnvironments(ctx context.Context, orgID pgtype.UUID, from, to time.Time) ([]DigestEnvironment, error) {
	rows, err := q.db.Query(ctx, `
		SELECT env.name,
		       COUNT(e.id) FILTER (WHERE e.type = 'deploy')                          AS deploys,
		       COUNT(e.id) FILTER (WHERE e.type = 'deploy' AND e.status = 'failure') AS failed,
		       COUNT(e.id) FILTER (WHERE e.type = 'rollback')                        AS rollbacks,
		       COUNT(e.id) FILTER (WHERE e.type = 'alert')                           AS alerts,
		       COUNT(e.id)                                                           AS total
		FROM environments env
		LEFT JOIN events e
		       ON e.environment_id = env.id
		      AND e.timestamp >= $2
		      AND e.timestamp < $3
		WHERE env.org_id = $1
		GROUP BY env.id, env.name
		ORDER BY total DESC, env.name`,
		orgID, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DigestEnvironment
	for rows.Next() {
		var d DigestEnvironment
		if err := rows.Scan(&d.Name, &d.Deploys, &d.FailedDeploys, &d.Rollbacks, &d.Alerts, &d.Total); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// DigestIncidents counts incidents opened and resolved during the window.
// Incidents carry org_id directly and have no environment, so they are reported
// once for the org rather than split across the environment sections.
func (q *Queries) DigestIncidents(ctx context.Context, orgID pgtype.UUID, from, to time.Time) (opened, resolved int64, err error) {
	err = q.db.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE opened_at >= $2 AND opened_at < $3),
		       COUNT(*) FILTER (WHERE resolved_at >= $2 AND resolved_at < $3)
		FROM incidents
		WHERE org_id = $1`,
		orgID, from, to).Scan(&opened, &resolved)
	return opened, resolved, err
}

// DigestThreads returns the events that drew the most discussion in the window,
// busiest first.
//
// Ranked by comments written during the week rather than the event's own age,
// so a months-old event that the team argued about on Tuesday still surfaces —
// which is exactly the conversation someone would otherwise miss.
func (q *Queries) DigestThreads(ctx context.Context, orgID pgtype.UUID, from, to time.Time, limit int32) ([]DigestThread, error) {
	rows, err := q.db.Query(ctx, `
		SELECT e.id, e.type, e.service, env.name, COUNT(c.id) AS comments, e.timestamp
		FROM comments c
		JOIN events e ON e.id = c.event_id
		JOIN environments env ON env.id = e.environment_id
		WHERE env.org_id = $1
		  AND c.created_at >= $2
		  AND c.created_at < $3
		GROUP BY e.id, e.type, e.service, env.name, e.timestamp
		ORDER BY comments DESC, e.timestamp DESC
		LIMIT $4`,
		orgID, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DigestThread
	for rows.Next() {
		var t DigestThread
		if err := rows.Scan(&t.EventID, &t.Type, &t.Service, &t.EnvironmentName, &t.Comments, &t.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
