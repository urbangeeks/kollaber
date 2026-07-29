package store

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// ServiceVersion is what one service was running at a point in time, and the
// event that put it there.
//
// Version is a pointer because it is genuinely unknown when the deploy that
// landed carried no version metadata. Reporting the previous version instead
// would be a lie — that build is not what is running.
type ServiceVersion struct {
	Service    string
	Version    *string
	EventID    pgtype.UUID
	EventType  string
	DeployedAt pgtype.Timestamptz
}

// versionExpr is the metadata key chain a version is read from, most canonical
// first. Every ingestion path names it differently: the CLI and GitHub Actions
// write `version`, the Kubernetes watcher also writes `image_tag`, Argo CD
// writes `revision`, Atlantis writes `head_commit`, and a rollback names where
// it went with `to`.
const versionExpr = `COALESCE(
	e.metadata->>'version',
	e.metadata->>'image_tag',
	e.metadata->>'revision',
	e.metadata->>'head_commit',
	e.metadata->>'to'
)`

// ServiceVersionsAt returns what each service in an environment was running at
// the given instant, alphabetically by service.
//
// Only successful deploys and rollbacks count. A failed deploy did not change
// what is running — treating it as the current version is how an inventory
// tells you a build is in production that never got there — and an in-progress
// one has not landed yet either.
//
// DISTINCT ON takes the newest qualifying event per service, so a service that
// has not been deployed since before the window still reports its last known
// version rather than dropping out of the inventory.
func (q *Queries) ServiceVersionsAt(ctx context.Context, orgID, environmentID pgtype.UUID, at time.Time) ([]ServiceVersion, error) {
	rows, err := q.db.Query(ctx, `
		SELECT DISTINCT ON (e.service)
		       e.service,
		       `+versionExpr+` AS version,
		       e.id, e.type, e.timestamp
		FROM events e
		JOIN environments env ON env.id = e.environment_id
		WHERE env.org_id = $1
		  AND e.environment_id = $2
		  AND e.timestamp <= $3
		  AND e.type IN ('deploy', 'rollback')
		  AND e.status = 'success'
		ORDER BY e.service, e.timestamp DESC`,
		orgID, environmentID, at)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ServiceVersion
	for rows.Next() {
		var s ServiceVersion
		if err := rows.Scan(&s.Service, &s.Version, &s.EventID, &s.EventType, &s.DeployedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
