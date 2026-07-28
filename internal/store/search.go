package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

// SearchHit is one match, either an event or a comment on one. Event is always
// populated: a comment hit carries the event it was posted on so the caller can
// show the match in context rather than as a floating quote.
type SearchHit struct {
	// Kind is "event" or "comment".
	Kind  string
	Event Event
	Rank  float32

	// Comment fields, set only when Kind == "comment".
	CommentID        pgtype.UUID
	CommentBody      string
	CommentUserID    pgtype.UUID
	CommentCreatedAt pgtype.Timestamptz
}

// SearchParams scopes a search. EnvironmentID is optional; when nil the search
// covers every environment in the org.
type SearchParams struct {
	OrgID         pgtype.UUID
	Query         string
	EnvironmentID *pgtype.UUID
	Limit         int32
}

// Search runs a full-text query over event text and comment bodies, best match
// first, then most recent.
//
// The query is parsed with websearch_to_tsquery, which accepts what users
// already type into search boxes — quoted phrases, OR, leading minus — and,
// unlike to_tsquery, never raises a syntax error on input it doesn't
// understand. A query that reduces to nothing (all stopwords) simply matches
// no rows.
//
// Both halves scope to the org through environments, since neither events nor
// comments carry org_id.
func (q *Queries) Search(ctx context.Context, arg SearchParams) ([]SearchHit, error) {
	rows, err := q.db.Query(ctx, `
		WITH tsq AS (SELECT websearch_to_tsquery('english', $2) AS query)
		SELECT
			'event' AS kind,
			e.id, e.type, e.service, e.environment_id, e.timestamp, e.metadata, e.created_at,
			e.status, e.ai_summary, e.ai_postmortem,
			ts_rank(e.search_vector, tsq.query) AS rank,
			NULL::uuid AS comment_id,
			NULL::text AS comment_body,
			NULL::uuid AS comment_user_id,
			NULL::timestamptz AS comment_created_at
		FROM events e
		JOIN environments env ON env.id = e.environment_id
		CROSS JOIN tsq
		WHERE env.org_id = $1
		  AND ($3::uuid IS NULL OR e.environment_id = $3::uuid)
		  AND e.search_vector @@ tsq.query

		UNION ALL

		SELECT
			'comment' AS kind,
			e.id, e.type, e.service, e.environment_id, e.timestamp, e.metadata, e.created_at,
			e.status, e.ai_summary, e.ai_postmortem,
			ts_rank(c.search_vector, tsq.query) AS rank,
			c.id, c.body, c.user_id, c.created_at
		FROM comments c
		JOIN events e ON e.id = c.event_id
		JOIN environments env ON env.id = e.environment_id
		CROSS JOIN tsq
		WHERE env.org_id = $1
		  AND ($3::uuid IS NULL OR e.environment_id = $3::uuid)
		  AND c.search_vector @@ tsq.query

		ORDER BY rank DESC, timestamp DESC
		LIMIT $4`,
		arg.OrgID, arg.Query, envArg(arg.EnvironmentID), arg.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SearchHit
	for rows.Next() {
		var h SearchHit
		var body *string
		if err := rows.Scan(
			&h.Kind,
			&h.Event.ID, &h.Event.Type, &h.Event.Service, &h.Event.EnvironmentID, &h.Event.Timestamp,
			&h.Event.Metadata, &h.Event.CreatedAt, &h.Event.Status, &h.Event.AISummary, &h.Event.AIPostmortem,
			&h.Rank,
			&h.CommentID, &body, &h.CommentUserID, &h.CommentCreatedAt,
		); err != nil {
			return nil, err
		}
		if body != nil {
			h.CommentBody = *body
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
