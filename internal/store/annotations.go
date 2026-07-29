package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// AnnotationRow is an event plus the name of the environment it belongs to.
// The name is what lets one dashboard panel tell a prod deploy from a staging
// one, so it is worth the join rather than a second lookup per row.
type AnnotationRow struct {
	Event
	EnvironmentName string
}

// ListAnnotationsParams scopes an annotation query. Types is required — the
// caller decides which event types deserve a marker, and an empty slice would
// silently match nothing.
type ListAnnotationsParams struct {
	OrgID         pgtype.UUID
	EnvironmentID *pgtype.UUID
	Types         []string
	Service       string
	From          time.Time
	To            time.Time
	Limit         int32
}

// ListAnnotations returns the events in a window that should render as markers
// on a dashboard, oldest first.
//
// The cap is applied to the newest events and the rows reversed afterwards, so
// a window holding more than Limit keeps the recent end — the part someone
// watching a graph is looking at — rather than stopping partway through the
// oldest and leaving the right-hand side of the panel bare.
func (q *Queries) ListAnnotations(ctx context.Context, arg ListAnnotationsParams) ([]AnnotationRow, error) {
	query := `
		SELECT e.id, e.type, e.service, e.environment_id, e.timestamp,
		       e.metadata, e.created_at, e.status, env.name
		FROM events e
		JOIN environments env ON env.id = e.environment_id
		WHERE env.org_id = $1
		  AND e.timestamp >= $2
		  AND e.timestamp <= $3
		  AND e.type = ANY($4)`
	args := []any{arg.OrgID, arg.From, arg.To, arg.Types}
	n := 5

	if arg.EnvironmentID != nil {
		query += fmt.Sprintf(" AND e.environment_id = $%d", n)
		args = append(args, *arg.EnvironmentID)
		n++
	}
	if arg.Service != "" {
		query += fmt.Sprintf(" AND e.service = $%d", n)
		args = append(args, arg.Service)
		n++
	}

	query += fmt.Sprintf(" ORDER BY e.timestamp DESC LIMIT $%d", n)
	args = append(args, arg.Limit)

	rows, err := q.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AnnotationRow
	for rows.Next() {
		var a AnnotationRow
		if err := rows.Scan(
			&a.ID,
			&a.Type,
			&a.Service,
			&a.EnvironmentID,
			&a.Timestamp,
			&a.Metadata,
			&a.CreatedAt,
			&a.Status,
			&a.EnvironmentName,
		); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}
