package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// FreezeWindow is a declared period during which the org would rather nothing
// changed. EnvironmentID is unset for an org-wide freeze.
type FreezeWindow struct {
	ID            pgtype.UUID
	OrgID         pgtype.UUID
	EnvironmentID *pgtype.UUID
	Reason        string
	StartsAt      pgtype.Timestamptz
	EndsAt        pgtype.Timestamptz
	CreatedBy     *pgtype.UUID
	CreatedAt     pgtype.Timestamptz
}

// Two spellings of the same column list: SELECTs alias the table as f, while
// INSERT ... RETURNING has no alias to qualify against.
const freezeColumns = `
	f.id, f.org_id, f.environment_id, f.reason, f.starts_at, f.ends_at,
	f.created_by, f.created_at`

const freezeColumnsBare = `
	id, org_id, environment_id, reason, starts_at, ends_at,
	created_by, created_at`

func scanFreeze(row interface{ Scan(...any) error }) (FreezeWindow, error) {
	var f FreezeWindow
	err := row.Scan(&f.ID, &f.OrgID, &f.EnvironmentID, &f.Reason,
		&f.StartsAt, &f.EndsAt, &f.CreatedBy, &f.CreatedAt)
	return f, err
}

type CreateFreezeWindowParams struct {
	OrgID         pgtype.UUID
	EnvironmentID *pgtype.UUID
	Reason        string
	StartsAt      time.Time
	EndsAt        time.Time
	CreatedBy     pgtype.UUID
}

func (q *Queries) CreateFreezeWindow(ctx context.Context, arg CreateFreezeWindowParams) (FreezeWindow, error) {
	row := q.db.QueryRow(ctx, `
		INSERT INTO freeze_windows (org_id, environment_id, reason, starts_at, ends_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING`+freezeColumnsBare,
		arg.OrgID, arg.EnvironmentID, arg.Reason, arg.StartsAt, arg.EndsAt, arg.CreatedBy)
	return scanFreeze(row)
}

// ListFreezeWindows returns an org's windows, soonest-starting first among
// those still to come and most recent first among those past — expressed simply
// as newest start first, which puts the active and upcoming ones at the top
// where they matter.
func (q *Queries) ListFreezeWindows(ctx context.Context, orgID pgtype.UUID) ([]FreezeWindow, error) {
	rows, err := q.db.Query(ctx, `
		SELECT`+freezeColumns+`
		FROM freeze_windows f
		WHERE f.org_id = $1
		ORDER BY f.starts_at DESC`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []FreezeWindow
	for rows.Next() {
		f, err := scanFreeze(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// DeleteFreezeWindow removes a window, scoped to the org. found is false when
// the id names a window belonging to another tenant, which the caller reports
// as a 404 either way.
//
// Deleting a window does not unmark the changes that already landed inside it:
// those events recorded what was true when they happened, and rewriting that
// afterwards would make the timeline a record of the current opinion rather
// than of what occurred.
func (q *Queries) DeleteFreezeWindow(ctx context.Context, id, orgID pgtype.UUID) (bool, error) {
	tag, err := q.db.Exec(ctx,
		`DELETE FROM freeze_windows WHERE id = $1 AND org_id = $2`, id, orgID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ActiveFreezeWindow returns the window covering an instant for one
// environment, or false when nothing is frozen.
//
// A window with no environment covers every environment in the org, so the
// filter has to accept both that and an exact match. When several overlap the
// one ending latest wins: that is the one still in force after the others
// lapse, and it is the honest thing to name when telling someone their deploy
// landed in a freeze.
func (q *Queries) ActiveFreezeWindow(ctx context.Context, orgID, environmentID pgtype.UUID, at time.Time) (FreezeWindow, bool, error) {
	row := q.db.QueryRow(ctx, `
		SELECT`+freezeColumns+`
		FROM freeze_windows f
		WHERE f.org_id = $1
		  AND (f.environment_id IS NULL OR f.environment_id = $2)
		  AND f.starts_at <= $3
		  AND f.ends_at > $3
		ORDER BY f.ends_at DESC
		LIMIT 1`,
		orgID, environmentID, at)

	f, err := scanFreeze(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return FreezeWindow{}, false, nil
		}
		return FreezeWindow{}, false, err
	}
	return f, true, nil
}
