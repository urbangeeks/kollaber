package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

func (q *Queries) GetEventByIDForOrg(ctx context.Context, eventID, orgID pgtype.UUID) (Event, error) {
	var e Event
	err := q.db.QueryRow(ctx, `
		SELECT e.id, e.type, e.service, e.environment_id, e.timestamp, e.metadata, e.created_at, e.status
		FROM events e
		JOIN environments env ON env.id = e.environment_id
		WHERE e.id = $1 AND env.org_id = $2`,
		eventID, orgID,
	).Scan(&e.ID, &e.Type, &e.Service, &e.EnvironmentID, &e.Timestamp, &e.Metadata, &e.CreatedAt, &e.Status)
	return e, err
}
