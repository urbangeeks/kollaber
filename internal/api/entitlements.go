package api

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/urbangeeks/kollaber/internal/billing"
	"github.com/urbangeeks/kollaber/internal/store"
)

func checkSeatLimit(ctx context.Context, q *store.Queries, orgID pgtype.UUID) error {
	b, err := q.GetOrgBilling(ctx, orgID)
	if err != nil {
		return nil // fail open if billing can't be read
	}
	ent := billing.EntitlementsFor(b.Plan)
	if ent.MaxMembers == -1 {
		return nil
	}
	count, err := q.CountBillableMembers(ctx, orgID)
	if err != nil {
		return nil
	}
	if count >= ent.MaxMembers {
		return fmt.Errorf("seat limit reached (%d/%d) — upgrade your plan to add more members", count, ent.MaxMembers)
	}
	return nil
}

func checkEnvLimit(ctx context.Context, q *store.Queries, orgID pgtype.UUID) error {
	b, err := q.GetOrgBilling(ctx, orgID)
	if err != nil {
		return nil
	}
	ent := billing.EntitlementsFor(b.Plan)
	if ent.MaxEnvironments == -1 {
		return nil
	}
	count, err := q.CountEnvironments(ctx, orgID)
	if err != nil {
		return nil
	}
	if count >= ent.MaxEnvironments {
		return fmt.Errorf("environment limit reached (%d/%d) — upgrade your plan to add more", count, ent.MaxEnvironments)
	}
	return nil
}
