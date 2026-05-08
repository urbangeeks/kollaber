package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

func (q *Queries) GetOrgTeamsWebhook(ctx context.Context, orgID pgtype.UUID) (string, error) {
	var url string
	err := q.db.QueryRow(ctx,
		`SELECT COALESCE(teams_webhook_url, '') FROM orgs WHERE id = $1`,
		orgID,
	).Scan(&url)
	return url, err
}

func (q *Queries) SetOrgTeamsWebhook(ctx context.Context, orgID pgtype.UUID, webhookURL string) error {
	_, err := q.db.Exec(ctx,
		`UPDATE orgs SET teams_webhook_url = NULLIF($2, '') WHERE id = $1`,
		orgID, webhookURL,
	)
	return err
}
