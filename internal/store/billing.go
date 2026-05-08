package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

type OrgBilling struct {
	Plan                   string
	StripeCustomerID       string
	StripeSubscriptionID   string
	SubscriptionStatus     string
	TrialEndsAt            pgtype.Timestamptz
}

func (q *Queries) GetOrgBilling(ctx context.Context, orgID pgtype.UUID) (OrgBilling, error) {
	var b OrgBilling
	err := q.db.QueryRow(ctx, `
		SELECT
			plan,
			COALESCE(stripe_customer_id, ''),
			COALESCE(stripe_subscription_id, ''),
			subscription_status,
			trial_ends_at
		FROM orgs WHERE id = $1`, orgID).
		Scan(&b.Plan, &b.StripeCustomerID, &b.StripeSubscriptionID, &b.SubscriptionStatus, &b.TrialEndsAt)
	return b, err
}

func (q *Queries) GetOrgBillingByStripeCustomerID(ctx context.Context, customerID string) (pgtype.UUID, OrgBilling, error) {
	var id pgtype.UUID
	var b OrgBilling
	err := q.db.QueryRow(ctx, `
		SELECT id, plan,
			COALESCE(stripe_customer_id, ''),
			COALESCE(stripe_subscription_id, ''),
			subscription_status,
			trial_ends_at
		FROM orgs WHERE stripe_customer_id = $1`, customerID).
		Scan(&id, &b.Plan, &b.StripeCustomerID, &b.StripeSubscriptionID, &b.SubscriptionStatus, &b.TrialEndsAt)
	return id, b, err
}

func (q *Queries) GetOrgBillingByStripeSubscriptionID(ctx context.Context, subID string) (pgtype.UUID, OrgBilling, error) {
	var id pgtype.UUID
	var b OrgBilling
	err := q.db.QueryRow(ctx, `
		SELECT id, plan,
			COALESCE(stripe_customer_id, ''),
			COALESCE(stripe_subscription_id, ''),
			subscription_status,
			trial_ends_at
		FROM orgs WHERE stripe_subscription_id = $1`, subID).
		Scan(&id, &b.Plan, &b.StripeCustomerID, &b.StripeSubscriptionID, &b.SubscriptionStatus, &b.TrialEndsAt)
	return id, b, err
}

func (q *Queries) SetOrgStripeCustomer(ctx context.Context, orgID pgtype.UUID, customerID string) error {
	_, err := q.db.Exec(ctx,
		`UPDATE orgs SET stripe_customer_id = $2 WHERE id = $1`,
		orgID, customerID)
	return err
}

func (q *Queries) SetOrgSubscription(ctx context.Context, orgID pgtype.UUID, plan, subscriptionID, status string) error {
	_, err := q.db.Exec(ctx, `
		UPDATE orgs
		SET plan = $2, stripe_subscription_id = $3, subscription_status = $4
		WHERE id = $1`,
		orgID, plan, subscriptionID, status)
	return err
}

func (q *Queries) SetOrgPlan(ctx context.Context, orgID pgtype.UUID, plan string) error {
	_, err := q.db.Exec(ctx, `UPDATE orgs SET plan = $2 WHERE id = $1`, orgID, plan)
	return err
}

func (q *Queries) SetOrgSubscriptionStatus(ctx context.Context, orgID pgtype.UUID, status string) error {
	_, err := q.db.Exec(ctx, `UPDATE orgs SET subscription_status = $2 WHERE id = $1`, orgID, status)
	return err
}

// CountBillableMembers returns the number of members in roles that count toward seat billing
// (owner, admin, member — not viewer).
func (q *Queries) CountBillableMembers(ctx context.Context, orgID pgtype.UUID) (int, error) {
	var n int
	err := q.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM org_members
		WHERE org_id = $1 AND role IN ('owner', 'admin', 'member')`, orgID).Scan(&n)
	return n, err
}

// CountEnvironments returns the number of environments for an org.
func (q *Queries) CountEnvironments(ctx context.Context, orgID pgtype.UUID) (int, error) {
	var n int
	err := q.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM environments WHERE org_id = $1`, orgID).Scan(&n)
	return n, err
}

// GetOrgName returns the name of an org.
func (q *Queries) GetOrgName(ctx context.Context, orgID pgtype.UUID) (string, error) {
	var name string
	err := q.db.QueryRow(ctx, `SELECT name FROM orgs WHERE id = $1`, orgID).Scan(&name)
	return name, err
}
