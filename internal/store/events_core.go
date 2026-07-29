package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

// Event is a row of the events table as the application uses it.
//
// Hand-written rather than generated because the table carries columns no
// caller wants back on a read: search_vector is a stored tsvector rebuilt from
// the row, and sqlc will only reuse a table's struct when a query selects every
// column. Generating these would put the full tsvector on the wire for every
// row of the busiest query in the product, so events keeps its own type and its
// queries live here beside the other hand-written ones.
//
// incident_id is likewise omitted: incidents are joined explicitly where they
// matter (see incidents.go) and carrying a mostly-null column on every timeline
// read buys nothing.
type Event struct {
	ID            pgtype.UUID        `json:"id"`
	Type          string             `json:"type"`
	Service       string             `json:"service"`
	EnvironmentID pgtype.UUID        `json:"environment_id"`
	Timestamp     pgtype.Timestamptz `json:"timestamp"`
	Metadata      []byte             `json:"metadata"`
	CreatedAt     pgtype.Timestamptz `json:"created_at"`
	Status        string             `json:"status"`
	AISummary     *string            `json:"ai_summary,omitempty"`
	AIPostmortem  *string            `json:"ai_postmortem,omitempty"`
}

// eventColumns is the projection every query returning an Event selects, in the
// order scanEvent expects.
const eventColumns = `e.id, e.type, e.service, e.environment_id, e.timestamp,
	e.metadata, e.created_at, e.status`

func scanEvent(row interface{ Scan(...any) error }) (Event, error) {
	var e Event
	err := row.Scan(&e.ID, &e.Type, &e.Service, &e.EnvironmentID,
		&e.Timestamp, &e.Metadata, &e.CreatedAt, &e.Status)
	return e, err
}

func scanEvents(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
},
) ([]Event, error) {
	var out []Event
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

type CreateEventParams struct {
	Type          string      `json:"type"`
	Service       string      `json:"service"`
	EnvironmentID pgtype.UUID `json:"environment_id"`
	Metadata      []byte      `json:"metadata"`
	Status        string      `json:"status"`
}

// CreateEvent inserts an event at the current time.
func (q *Queries) CreateEvent(ctx context.Context, arg CreateEventParams) (Event, error) {
	row := q.db.QueryRow(ctx, `
		INSERT INTO events (type, service, environment_id, metadata, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, type, service, environment_id, timestamp, metadata, created_at, status`,
		arg.Type, arg.Service, arg.EnvironmentID, arg.Metadata, arg.Status)
	return scanEvent(row)
}

type ListEventsByEnvironmentParams struct {
	EnvironmentID pgtype.UUID `json:"environment_id"`
	OrgID         pgtype.UUID `json:"org_id"`
	Limit         int32       `json:"limit"`
	Offset        int32       `json:"offset"`
}

// ListEventsByEnvironment returns one environment's events, newest first.
//
// The join onto environments is the tenancy check: events carry no org_id, so
// filtering on environment_id alone would serve another org's timeline to
// anyone who could guess a uuid.
func (q *Queries) ListEventsByEnvironment(ctx context.Context, arg ListEventsByEnvironmentParams) ([]Event, error) {
	rows, err := q.db.Query(ctx, `
		SELECT `+eventColumns+`
		FROM events e
		JOIN environments env ON env.id = e.environment_id
		WHERE e.environment_id = $1 AND env.org_id = $2
		ORDER BY e.timestamp DESC
		LIMIT $3 OFFSET $4`,
		arg.EnvironmentID, arg.OrgID, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}

type ListEventsByOrgParams struct {
	OrgID  pgtype.UUID `json:"org_id"`
	Limit  int32       `json:"limit"`
	Offset int32       `json:"offset"`
}

// ListEventsByOrg returns every environment's events for one org, newest first.
func (q *Queries) ListEventsByOrg(ctx context.Context, arg ListEventsByOrgParams) ([]Event, error) {
	rows, err := q.db.Query(ctx, `
		SELECT `+eventColumns+`
		FROM events e
		JOIN environments env ON env.id = e.environment_id
		WHERE env.org_id = $1
		ORDER BY e.timestamp DESC
		LIMIT $2 OFFSET $3`,
		arg.OrgID, arg.Limit, arg.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEvents(rows)
}
