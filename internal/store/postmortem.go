package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// CommentWithAuthor is a comment plus the email of whoever wrote it. Postmortems
// need the name against the words: "we decided to roll back" is worth much less
// six months later than "alice decided to roll back".
type CommentWithAuthor struct {
	Comment
	AuthorEmail string
}

// ListCommentsInRangeParams scopes a postmortem's discussion lookup.
type ListCommentsInRangeParams struct {
	OrgID         pgtype.UUID
	EnvironmentID pgtype.UUID
	From          time.Time
	To            time.Time
	Limit         int32
}

// ListCommentsInRange returns every comment on events that occurred in the
// window, oldest first, with each author's email.
//
// The window filters on the *event's* timestamp, not the comment's. A
// postmortem covers what happened between two times; a comment added a week
// later reflecting on that outage belongs in the document, and filtering on
// comment time would drop exactly the considered analysis worth keeping.
func (q *Queries) ListCommentsInRange(ctx context.Context, arg ListCommentsInRangeParams) ([]CommentWithAuthor, error) {
	rows, err := q.db.Query(ctx, `
		SELECT c.id, c.event_id, c.user_id, c.body, c.created_at, u.email
		FROM comments c
		JOIN events e ON e.id = c.event_id
		JOIN environments env ON env.id = e.environment_id
		JOIN users u ON u.id = c.user_id
		WHERE env.org_id = $1
		  AND e.environment_id = $2
		  AND e.timestamp >= $3
		  AND e.timestamp <= $4
		ORDER BY e.timestamp ASC, c.created_at ASC
		LIMIT $5`,
		arg.OrgID, arg.EnvironmentID, arg.From, arg.To, arg.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CommentWithAuthor
	for rows.Next() {
		var c CommentWithAuthor
		if err := rows.Scan(&c.ID, &c.EventID, &c.UserID, &c.Body, &c.CreatedAt, &c.AuthorEmail); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
