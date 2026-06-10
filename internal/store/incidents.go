package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Incident struct {
	ID           pgtype.UUID        `json:"id"`
	OrgID        pgtype.UUID        `json:"org_id"`
	Title        string             `json:"title"`
	Severity     string             `json:"severity"`
	Status       string             `json:"status"`
	OwnerID      pgtype.UUID        `json:"owner_id"`
	OpenedAt     pgtype.Timestamptz `json:"opened_at"`
	ResolvedAt   pgtype.Timestamptz `json:"resolved_at"`
	CreatedAt    pgtype.Timestamptz `json:"created_at"`
	AIPostmortem *string            `json:"ai_postmortem,omitempty"`
}

const incidentColumns = `id, org_id, title, severity, status, owner_id, opened_at, resolved_at, created_at, ai_postmortem`

func scanIncident(row interface {
	Scan(dest ...any) error
}) (Incident, error) {
	var i Incident
	err := row.Scan(&i.ID, &i.OrgID, &i.Title, &i.Severity, &i.Status, &i.OwnerID, &i.OpenedAt, &i.ResolvedAt, &i.CreatedAt, &i.AIPostmortem)
	return i, err
}

type CreateIncidentParams struct {
	OrgID    pgtype.UUID
	Title    string
	Severity string
	OwnerID  pgtype.UUID
}

func (q *Queries) CreateIncident(ctx context.Context, arg CreateIncidentParams) (Incident, error) {
	row := q.db.QueryRow(ctx, `
		INSERT INTO incidents (org_id, title, severity, owner_id)
		VALUES ($1, $2, $3, $4)
		RETURNING `+incidentColumns,
		arg.OrgID, arg.Title, arg.Severity, arg.OwnerID,
	)
	return scanIncident(row)
}

func (q *Queries) GetIncidentByID(ctx context.Context, id, orgID pgtype.UUID) (Incident, error) {
	row := q.db.QueryRow(ctx, `
		SELECT `+incidentColumns+`
		FROM incidents
		WHERE id = $1 AND org_id = $2`,
		id, orgID,
	)
	return scanIncident(row)
}

// ListIncidents returns incidents for an org, newest first. If status is
// non-empty it filters to that status.
func (q *Queries) ListIncidents(ctx context.Context, orgID pgtype.UUID, status string) ([]Incident, error) {
	query := `SELECT ` + incidentColumns + ` FROM incidents WHERE org_id = $1`
	args := []any{orgID}
	if status != "" {
		query += ` AND status = $2`
		args = append(args, status)
	}
	query += ` ORDER BY opened_at DESC`

	rows, err := q.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Incident
	for rows.Next() {
		i, err := scanIncident(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

type UpdateIncidentParams struct {
	ID       pgtype.UUID
	OrgID    pgtype.UUID
	Title    string
	Severity string
	Status   string
	OwnerID  pgtype.UUID
}

// UpdateIncident applies the full set of mutable fields. resolved_at is
// stamped when the incident first transitions to resolved and cleared if it
// is re-opened.
func (q *Queries) UpdateIncident(ctx context.Context, arg UpdateIncidentParams) (Incident, error) {
	row := q.db.QueryRow(ctx, `
		UPDATE incidents
		SET title = $3,
		    severity = $4,
		    status = $5,
		    owner_id = $6,
		    resolved_at = CASE
		        WHEN $5 = 'resolved' AND resolved_at IS NULL THEN NOW()
		        WHEN $5 <> 'resolved' THEN NULL
		        ELSE resolved_at
		    END
		WHERE id = $1 AND org_id = $2
		RETURNING `+incidentColumns,
		arg.ID, arg.OrgID, arg.Title, arg.Severity, arg.Status, arg.OwnerID,
	)
	return scanIncident(row)
}

func (q *Queries) SetIncidentAIPostmortem(ctx context.Context, id pgtype.UUID, postmortem string) error {
	_, err := q.db.Exec(ctx, `UPDATE incidents SET ai_postmortem = $1 WHERE id = $2`, postmortem, id)
	return err
}

// AttachEventsToIncident links the given events to an incident, but only events
// whose environment belongs to the same org. Returns the number of events
// actually attached.
func (q *Queries) AttachEventsToIncident(ctx context.Context, incidentID, orgID pgtype.UUID, eventIDs []uuid.UUID) (int64, error) {
	pgIDs := make([]pgtype.UUID, len(eventIDs))
	for i, id := range eventIDs {
		pgIDs[i] = pgtype.UUID{Bytes: id, Valid: true}
	}
	tag, err := q.db.Exec(ctx, `
		UPDATE events
		SET incident_id = $1
		WHERE id = ANY($2::uuid[])
		  AND environment_id IN (SELECT id FROM environments WHERE org_id = $3)`,
		incidentID, pgIDs, orgID,
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ListIncidentEvents returns the events linked to an incident (org-scoped),
// ordered chronologically. Only the core event columns are populated.
func (q *Queries) ListIncidentEvents(ctx context.Context, incidentID, orgID pgtype.UUID) ([]Event, error) {
	rows, err := q.db.Query(ctx, `
		SELECT e.id, e.type, e.service, e.environment_id, e.timestamp, e.metadata, e.created_at, e.status
		FROM events e
		JOIN environments env ON env.id = e.environment_id
		WHERE e.incident_id = $1 AND env.org_id = $2
		ORDER BY e.timestamp ASC`,
		incidentID, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Type, &e.Service, &e.EnvironmentID, &e.Timestamp, &e.Metadata, &e.CreatedAt, &e.Status); err != nil {
			return nil, err
		}
		items = append(items, e)
	}
	return items, rows.Err()
}
