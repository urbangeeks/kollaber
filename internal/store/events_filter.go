package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type ListEventsFilterParams struct {
	OrgID         pgtype.UUID
	EnvironmentID *pgtype.UUID
	Type          string
	Service       string
	Status        string
	Before        *time.Time
	Limit         int32
	Offset        int32
}

func (q *Queries) ListEventsFiltered(ctx context.Context, arg ListEventsFilterParams) ([]Event, error) {
	query := `SELECT e.id, e.type, e.service, e.environment_id, e.timestamp, e.metadata, e.created_at, e.status
		FROM events e
		JOIN environments env ON env.id = e.environment_id
		WHERE env.org_id = $1`
	args := []any{arg.OrgID}
	n := 2

	if arg.EnvironmentID != nil {
		query += fmt.Sprintf(" AND e.environment_id = $%d", n)
		args = append(args, *arg.EnvironmentID)
		n++
	}
	if arg.Type != "" {
		query += fmt.Sprintf(" AND e.type = $%d", n)
		args = append(args, arg.Type)
		n++
	}
	if arg.Service != "" {
		query += fmt.Sprintf(" AND e.service = $%d", n)
		args = append(args, arg.Service)
		n++
	}
	if arg.Status != "" {
		query += fmt.Sprintf(" AND e.status = $%d", n)
		args = append(args, arg.Status)
		n++
	}
	if arg.Before != nil {
		query += fmt.Sprintf(" AND e.timestamp < $%d", n)
		args = append(args, *arg.Before)
		n++
	}

	query += fmt.Sprintf(" ORDER BY e.timestamp DESC LIMIT $%d OFFSET $%d", n, n+1)
	args = append(args, arg.Limit, arg.Offset)

	rows, err := q.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(
			&e.ID,
			&e.Type,
			&e.Service,
			&e.EnvironmentID,
			&e.Timestamp,
			&e.Metadata,
			&e.CreatedAt,
			&e.Status,
		); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, rows.Err()
}
