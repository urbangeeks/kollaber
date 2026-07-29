package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// CommentDetail is a comment with its decision fields and the author's email.
//
// Separate from the generated Comment because that one predates the decision
// columns, and because every query here joins through events to environments to
// scope by org — comments carry no org_id of their own.
type CommentDetail struct {
	ID          pgtype.UUID
	EventID     pgtype.UUID
	UserID      pgtype.UUID
	Body        string
	CreatedAt   pgtype.Timestamptz
	AuthorEmail string
	IsDecision  bool
	DecidedBy   *pgtype.UUID
	DecidedAt   pgtype.Timestamptz
}

// Decision is a marked comment together with the event it was written on. The
// event context is the point: "we're rolling back" means nothing without the
// deploy it was said about.
type Decision struct {
	CommentDetail
	EventType       string
	EventService    string
	EventTimestamp  pgtype.Timestamptz
	EnvironmentID   pgtype.UUID
	EnvironmentName string
}

const commentDetailColumns = `
	c.id, c.event_id, c.user_id, c.body, c.created_at,
	u.email, c.is_decision, c.decided_by, c.decided_at`

func scanCommentDetail(row interface{ Scan(...any) error }) (CommentDetail, error) {
	var c CommentDetail
	err := row.Scan(&c.ID, &c.EventID, &c.UserID, &c.Body, &c.CreatedAt,
		&c.AuthorEmail, &c.IsDecision, &c.DecidedBy, &c.DecidedAt)
	return c, err
}

// ListCommentsForEvent returns an event's comments oldest first, scoped to the
// org that owns the event's environment.
//
// The org scope is not optional. Comments hang off events, which hang off
// environments, which is the only thing tying either to a tenant — a query that
// filters on event_id alone hands one org's discussion to anyone who can guess
// a uuid.
func (q *Queries) ListCommentsForEvent(ctx context.Context, eventID, orgID pgtype.UUID) ([]CommentDetail, error) {
	rows, err := q.db.Query(ctx, `
		SELECT`+commentDetailColumns+`
		FROM comments c
		JOIN events e ON e.id = c.event_id
		JOIN environments env ON env.id = e.environment_id
		JOIN users u ON u.id = c.user_id
		WHERE c.event_id = $1 AND env.org_id = $2
		ORDER BY c.created_at ASC`,
		eventID, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CommentDetail
	for rows.Next() {
		c, err := scanCommentDetail(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetCommentDecision marks or unmarks a comment as a decision and returns the
// updated row. found is false when the comment does not exist or belongs to
// another org, which the caller reports as a 404 either way — telling someone
// their guess named a real comment in a different tenant is itself a leak.
//
// decidedBy records who promoted the comment, which need not be its author: the
// decision is the author's, the curation is somebody's later act of noticing it
// mattered. Unmarking clears both, so a re-marked comment does not carry a
// stale attribution.
func (q *Queries) SetCommentDecision(ctx context.Context, commentID, orgID, userID pgtype.UUID, decided bool) (CommentDetail, bool, error) {
	row := q.db.QueryRow(ctx, `
		WITH scoped AS (
			SELECT c.id
			FROM comments c
			JOIN events e ON e.id = c.event_id
			JOIN environments env ON env.id = e.environment_id
			WHERE c.id = $1 AND env.org_id = $2
		)
		UPDATE comments c
		SET is_decision = $3,
		    decided_by  = CASE WHEN $3 THEN $4::uuid ELSE NULL END,
		    decided_at  = CASE WHEN $3 THEN NOW() ELSE NULL END
		FROM scoped s, users u
		WHERE c.id = s.id AND u.id = c.user_id
		RETURNING`+commentDetailColumns,
		commentID, orgID, decided, userID)

	c, err := scanCommentDetail(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CommentDetail{}, false, nil
		}
		return CommentDetail{}, false, err
	}
	return c, true, nil
}

// ListDecisionsParams scopes a decisions query. EnvironmentID is optional; when
// nil the log covers every environment in the org.
type ListDecisionsParams struct {
	OrgID         pgtype.UUID
	EnvironmentID *pgtype.UUID
	Limit         int32
	Offset        int32
}

// ListDecisions returns the org's marked comments, newest first.
//
// Ordered by when the comment was written rather than when it was marked: the
// log is a record of what was decided and when, and someone tidying up old
// threads on a Friday should not reshuffle the history.
func (q *Queries) ListDecisions(ctx context.Context, arg ListDecisionsParams) ([]Decision, error) {
	query := `
		SELECT` + commentDetailColumns + `,
		       e.type, e.service, e.timestamp, env.id, env.name
		FROM comments c
		JOIN events e ON e.id = c.event_id
		JOIN environments env ON env.id = e.environment_id
		JOIN users u ON u.id = c.user_id
		WHERE env.org_id = $1 AND c.is_decision`
	args := []any{arg.OrgID}
	n := 2

	if arg.EnvironmentID != nil {
		query += fmt.Sprintf(" AND e.environment_id = $%d", n)
		args = append(args, *arg.EnvironmentID)
		n++
	}

	query += fmt.Sprintf(" ORDER BY c.created_at DESC LIMIT $%d OFFSET $%d", n, n+1)
	args = append(args, arg.Limit, arg.Offset)

	rows, err := q.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Decision
	for rows.Next() {
		var d Decision
		if err := rows.Scan(
			&d.ID, &d.EventID, &d.UserID, &d.Body, &d.CreatedAt,
			&d.AuthorEmail, &d.IsDecision, &d.DecidedBy, &d.DecidedAt,
			&d.EventType, &d.EventService, &d.EventTimestamp,
			&d.EnvironmentID, &d.EnvironmentName,
		); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// CountDecisions reports how many decisions the org has, so a paged view can
// say whether there is more to fetch.
func (q *Queries) CountDecisions(ctx context.Context, orgID pgtype.UUID, environmentID *pgtype.UUID) (int64, error) {
	query := `
		SELECT COUNT(*)
		FROM comments c
		JOIN events e ON e.id = c.event_id
		JOIN environments env ON env.id = e.environment_id
		WHERE env.org_id = $1 AND c.is_decision`
	args := []any{orgID}
	if environmentID != nil {
		query += " AND e.environment_id = $2"
		args = append(args, *environmentID)
	}

	var n int64
	err := q.db.QueryRow(ctx, query, args...).Scan(&n)
	return n, err
}
