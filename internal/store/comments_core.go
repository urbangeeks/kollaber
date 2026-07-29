package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

// Comment is a row of the comments table as the application uses it.
//
// Hand-written for the same reason as Event: the table carries a stored
// tsvector, and sqlc reuses a table's struct only when a query selects every
// column. The decision fields live on CommentDetail (see decisions.go), which
// is what the endpoints that care about them return.
type Comment struct {
	ID        pgtype.UUID        `json:"id"`
	EventID   pgtype.UUID        `json:"event_id"`
	UserID    pgtype.UUID        `json:"user_id"`
	Body      string             `json:"body"`
	CreatedAt pgtype.Timestamptz `json:"created_at"`
}

const commentColumns = `id, event_id, user_id, body, created_at`

func scanComment(row interface{ Scan(...any) error }) (Comment, error) {
	var c Comment
	err := row.Scan(&c.ID, &c.EventID, &c.UserID, &c.Body, &c.CreatedAt)
	return c, err
}

type CreateCommentParams struct {
	EventID pgtype.UUID `json:"event_id"`
	UserID  pgtype.UUID `json:"user_id"`
	Body    string      `json:"body"`
}

func (q *Queries) CreateComment(ctx context.Context, arg CreateCommentParams) (Comment, error) {
	row := q.db.QueryRow(ctx, `
		INSERT INTO comments (event_id, user_id, body)
		VALUES ($1, $2, $3)
		RETURNING `+commentColumns,
		arg.EventID, arg.UserID, arg.Body)
	return scanComment(row)
}

// ListCommentsByEvent returns an event's comments oldest first.
//
// Unscoped by design: the callers that serve a comment thread to a person use
// ListCommentsForEvent, which takes an org. This one feeds the AI summary and
// postmortem builders, which have already resolved the event through its
// environment before they get here.
func (q *Queries) ListCommentsByEvent(ctx context.Context, eventID pgtype.UUID) ([]Comment, error) {
	rows, err := q.db.Query(ctx, `
		SELECT `+commentColumns+`
		FROM comments
		WHERE event_id = $1
		ORDER BY created_at ASC`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Comment
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
