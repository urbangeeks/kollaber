package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func (q *Queries) GetEventByIDForOrg(ctx context.Context, eventID, orgID pgtype.UUID) (Event, error) {
	var e Event
	err := q.db.QueryRow(ctx, `
		SELECT e.id, e.type, e.service, e.environment_id, e.timestamp, e.metadata, e.created_at, e.status,
		       e.ai_summary, e.ai_postmortem
		FROM events e
		JOIN environments env ON env.id = e.environment_id
		WHERE e.id = $1 AND env.org_id = $2`,
		eventID, orgID,
	).Scan(&e.ID, &e.Type, &e.Service, &e.EnvironmentID, &e.Timestamp, &e.Metadata, &e.CreatedAt, &e.Status,
		&e.AISummary, &e.AIPostmortem)
	return e, err
}

func (q *Queries) SetEventAISummary(ctx context.Context, eventID pgtype.UUID, summary string) error {
	_, err := q.db.Exec(ctx, `UPDATE events SET ai_summary = $1 WHERE id = $2`, summary, eventID)
	return err
}

func (q *Queries) SetEventAIPostmortem(ctx context.Context, eventID pgtype.UUID, postmortem string) error {
	_, err := q.db.Exec(ctx, `UPDATE events SET ai_postmortem = $1 WHERE id = $2`, postmortem, eventID)
	return err
}

// GetEventsAroundTime returns events in the same environment within ±window of t,
// ordered by timestamp ascending, excluding the event with excludeID.
func (q *Queries) GetEventsAroundTime(ctx context.Context, envID pgtype.UUID, t time.Time, window time.Duration, excludeID pgtype.UUID) ([]Event, error) {
	rows, err := q.db.Query(ctx, `
		SELECT id, type, service, environment_id, timestamp, metadata, created_at, status,
		       ai_summary, ai_postmortem
		FROM events
		WHERE environment_id = $1
		  AND timestamp BETWEEN $2 AND $3
		  AND id != $4
		ORDER BY timestamp ASC
		LIMIT 50`,
		envID,
		t.Add(-window),
		t.Add(window),
		excludeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Type, &e.Service, &e.EnvironmentID, &e.Timestamp, &e.Metadata, &e.CreatedAt, &e.Status,
			&e.AISummary, &e.AIPostmortem); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, rows.Err()
}
