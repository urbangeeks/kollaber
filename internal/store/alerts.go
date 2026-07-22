package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreateEventAtParams mirrors CreateEventParams with an explicit event time.
type CreateEventAtParams struct {
	Type          string
	Service       string
	EnvironmentID pgtype.UUID
	Metadata      []byte
	Status        string
	Timestamp     pgtype.Timestamptz
}

// CreateEventAt inserts an event at a caller-supplied timestamp instead of
// letting the column default to NOW(). Webhook sources that batch or retry
// deliver events well after they occurred, and a timeline that orders them by
// arrival rather than occurrence would misreport what happened first.
func (q *Queries) CreateEventAt(ctx context.Context, arg CreateEventAtParams) (Event, error) {
	var e Event
	err := q.db.QueryRow(ctx, `
		INSERT INTO events (type, service, environment_id, metadata, status, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, type, service, environment_id, timestamp, metadata, created_at, status,
		          ai_summary, ai_postmortem`,
		arg.Type, arg.Service, arg.EnvironmentID, arg.Metadata, arg.Status, arg.Timestamp,
	).Scan(&e.ID, &e.Type, &e.Service, &e.EnvironmentID, &e.Timestamp, &e.Metadata, &e.CreatedAt, &e.Status,
		&e.AISummary, &e.AIPostmortem)
	return e, err
}

// LatestAlertStatusByFingerprint returns the status of the most recent alert
// event in envID carrying the given Alertmanager fingerprint.
//
// Alertmanager re-sends a firing alert on every repeat_interval, so ingesting
// each delivery unconditionally would bury the timeline under duplicates. The
// caller compares the returned status against the incoming one and skips the
// write when they match — a firing->resolved transition still lands, because
// the status differs.
//
// found is false when no event with that fingerprint exists yet.
func (q *Queries) LatestAlertStatusByFingerprint(ctx context.Context, envID pgtype.UUID, fingerprint string) (status string, found bool, err error) {
	err = q.db.QueryRow(ctx, `
		SELECT status
		FROM events
		WHERE environment_id = $1
		  AND type = 'alert'
		  AND metadata->>'fingerprint' = $2
		ORDER BY timestamp DESC
		LIMIT 1`,
		envID, fingerprint,
	).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return status, true, nil
}
