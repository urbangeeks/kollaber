package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

func (q *Queries) GetOrgSlackWebhook(ctx context.Context, orgID pgtype.UUID) (string, error) {
	var url string
	err := q.db.QueryRow(ctx,
		`SELECT COALESCE(slack_webhook_url, '') FROM orgs WHERE id = $1`,
		orgID,
	).Scan(&url)
	return url, err
}

func (q *Queries) SetOrgSlackWebhook(ctx context.Context, orgID pgtype.UUID, webhookURL string) error {
	_, err := q.db.Exec(ctx,
		`UPDATE orgs SET slack_webhook_url = NULLIF($2, '') WHERE id = $1`,
		orgID, webhookURL,
	)
	return err
}
