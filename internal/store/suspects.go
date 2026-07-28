package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// ChangeEventTypes are the event types that represent something changing in the
// system, and so can plausibly explain an alert. Notes are human annotations
// and alerts are the thing being explained, so neither is ever a candidate.
//
// This is deliberately a subset of ValidEventTypes rather than a derivation of
// it: a new event type should have to opt in to being blamed for outages.
var ChangeEventTypes = []string{"deploy", "rollback", "scale", "teardown"}

// ListChangesBeforeParams scopes a suspect-change lookup. Window is measured
// backwards from Before, and Limit caps the candidate set handed to ranking.
type ListChangesBeforeParams struct {
	OrgID         pgtype.UUID
	EnvironmentID pgtype.UUID
	Before        time.Time
	Window        time.Duration
	Limit         int32
	// ExcludeID keeps the triggering event out of its own candidate list. Leave
	// it zero (Valid: false) to exclude nothing.
	ExcludeID pgtype.UUID
}

// ListChangesBefore returns the change events in one environment that landed in
// the window immediately before arg.Before, newest first.
//
// Only changes that precede the alert are returned. A deploy that landed after
// an alert fired cannot have caused it, and including them would produce
// confident-looking nonsense in the UI — the ranking layer has no way to tell
// the difference once ordering is lost.
func (q *Queries) ListChangesBefore(ctx context.Context, arg ListChangesBeforeParams) ([]Event, error) {
	rows, err := q.db.Query(ctx, `
		SELECT e.id, e.type, e.service, e.environment_id, e.timestamp, e.metadata, e.created_at, e.status,
		       e.ai_summary, e.ai_postmortem
		FROM events e
		JOIN environments env ON env.id = e.environment_id
		WHERE env.org_id = $1
		  AND e.environment_id = $2
		  AND e.type = ANY($3)
		  AND e.timestamp <= $4
		  AND e.timestamp >= $5
		  AND ($6::uuid IS NULL OR e.id != $6::uuid)
		ORDER BY e.timestamp DESC
		LIMIT $7`,
		arg.OrgID,
		arg.EnvironmentID,
		ChangeEventTypes,
		arg.Before,
		arg.Before.Add(-arg.Window),
		arg.ExcludeID,
		arg.Limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Type, &e.Service, &e.EnvironmentID, &e.Timestamp, &e.Metadata,
			&e.CreatedAt, &e.Status, &e.AISummary, &e.AIPostmortem); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
