package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

// DORAMetrics holds the raw aggregates behind the four DORA keys for one org
// over a time window. Ratings (Elite/High/Medium/Low) are computed in the API
// layer so the store stays a thin data accessor.
//
// Events are scoped to the org through environments. Incidents carry org_id
// directly and have no environment, so the environment filter (when set)
// narrows the deploy-derived metrics only — MTTR always covers the whole org.
type DORAMetrics struct {
	// Deployment frequency / change failure rate (from deploy events).
	TotalDeploys  int64
	FailedDeploys int64

	// Lead time for changes (best effort, from commit metadata on deploys).
	LeadTimeSamples int64
	LeadTimeSeconds float64

	// Time to restore (from resolved incidents).
	ResolvedIncidents int64
	RestoreSeconds    float64
}

// DeployTrendPoint is one day's deploy count, used to draw a sparkline of
// deployment frequency over the window. Failed counts the subset of that day's
// deploys with a failure status, so the UI can flag days that shipped a bad
// change.
type DeployTrendPoint struct {
	Day     pgtype.Timestamptz `json:"day"`
	Deploys int64              `json:"deploys"`
	Failed  int64              `json:"failed"`
}

// DORAParams scopes a metrics query. EnvironmentID is optional; when nil the
// deploy-derived metrics cover every environment in the org.
type DORAParams struct {
	OrgID         pgtype.UUID
	EnvironmentID *pgtype.UUID
	Since         pgtype.Timestamptz
}

// DORA returns the aggregates for the four keys in a single round trip per
// source table (one query over events, one over incidents).
func (q *Queries) DORA(ctx context.Context, arg DORAParams) (DORAMetrics, error) {
	var m DORAMetrics

	// Deploy-derived metrics. The lead-time term only counts deploys that carry
	// a parseable ISO commit timestamp in metadata; the regex guard keeps a
	// malformed value from aborting the whole aggregate with a cast error.
	row := q.db.QueryRow(ctx, `
		WITH deploys AS (
			SELECT
				e.timestamp,
				e.status,
				CASE
					WHEN commit_raw ~ '^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}'
					THEN commit_raw::timestamptz
				END AS committed_at
			FROM events e
			JOIN environments env ON env.id = e.environment_id
			CROSS JOIN LATERAL (
				SELECT COALESCE(
					e.metadata->>'committed_at',
					e.metadata->>'commit_time',
					e.metadata->>'commit_timestamp'
				) AS commit_raw
			) c
			WHERE env.org_id = $1
			  AND e.type = 'deploy'
			  AND e.timestamp >= $2
			  AND ($3::uuid IS NULL OR e.environment_id = $3::uuid)
		)
		SELECT
			COUNT(*),
			COUNT(*) FILTER (WHERE status = 'failure'),
			COUNT(*) FILTER (WHERE committed_at IS NOT NULL AND timestamp >= committed_at),
			COALESCE(EXTRACT(EPOCH FROM AVG(timestamp - committed_at)
				FILTER (WHERE committed_at IS NOT NULL AND timestamp >= committed_at)), 0)
		FROM deploys`,
		arg.OrgID, arg.Since, envArg(arg.EnvironmentID),
	)
	if err := row.Scan(&m.TotalDeploys, &m.FailedDeploys, &m.LeadTimeSamples, &m.LeadTimeSeconds); err != nil {
		return m, err
	}

	// Time to restore, from incidents resolved within the window.
	row = q.db.QueryRow(ctx, `
		SELECT
			COUNT(*),
			COALESCE(EXTRACT(EPOCH FROM AVG(resolved_at - opened_at)), 0)
		FROM incidents
		WHERE org_id = $1
		  AND status = 'resolved'
		  AND resolved_at IS NOT NULL
		  AND resolved_at >= $2`,
		arg.OrgID, arg.Since,
	)
	if err := row.Scan(&m.ResolvedIncidents, &m.RestoreSeconds); err != nil {
		return m, err
	}

	return m, nil
}

// DeployTrend returns per-day deploy counts across the window, oldest first.
// Days with no deploys are omitted; the API layer fills gaps if it needs a
// dense series.
func (q *Queries) DeployTrend(ctx context.Context, arg DORAParams) ([]DeployTrendPoint, error) {
	rows, err := q.db.Query(ctx, `
		SELECT
			date_trunc('day', e.timestamp) AS day,
			COUNT(*),
			COUNT(*) FILTER (WHERE e.status = 'failure')
		FROM events e
		JOIN environments env ON env.id = e.environment_id
		WHERE env.org_id = $1
		  AND e.type = 'deploy'
		  AND e.timestamp >= $2
		  AND ($3::uuid IS NULL OR e.environment_id = $3::uuid)
		GROUP BY 1
		ORDER BY 1`,
		arg.OrgID, arg.Since, envArg(arg.EnvironmentID),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DeployTrendPoint
	for rows.Next() {
		var p DeployTrendPoint
		if err := rows.Scan(&p.Day, &p.Deploys, &p.Failed); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// envArg unwraps an optional environment filter into a driver-friendly value:
// a typed nil UUID when unset so the `$3::uuid IS NULL` guards short-circuit.
func envArg(env *pgtype.UUID) any {
	if env == nil || !env.Valid {
		return nil
	}
	return *env
}
