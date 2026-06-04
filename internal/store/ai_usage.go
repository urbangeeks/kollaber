package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// IncrementAIAgentUsage atomically records one AI agent call for an org in the
// given period ('YYYY-MM'). quota is the org's monthly allowance; pass a
// negative value for unlimited.
//
// When a quota applies the increment is conditional: if the org is already at
// or over the quota the counter is left untouched and allowed is false, so
// rejected calls never consume quota. It returns the post-increment count
// (or the unchanged count when not allowed).
func (q *Queries) IncrementAIAgentUsage(ctx context.Context, orgID pgtype.UUID, period string, quota int) (count int, allowed bool, err error) {
	if quota < 0 {
		err = q.db.QueryRow(ctx, `
			INSERT INTO ai_usage (org_id, period, count) VALUES ($1, $2, 1)
			ON CONFLICT (org_id, period)
			DO UPDATE SET count = ai_usage.count + 1
			RETURNING count`, orgID, period).Scan(&count)
		return count, true, err
	}

	err = q.db.QueryRow(ctx, `
		INSERT INTO ai_usage (org_id, period, count) VALUES ($1, $2, 1)
		ON CONFLICT (org_id, period)
		DO UPDATE SET count = ai_usage.count + 1
		WHERE ai_usage.count < $3
		RETURNING count`, orgID, period, quota).Scan(&count)
	if errors.Is(err, pgx.ErrNoRows) {
		// Conflict row existed but was at/over quota, so DO UPDATE matched no
		// row and nothing was returned. The org is over its allowance.
		return quota, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return count, true, nil
}
