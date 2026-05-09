package store

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
)

// GetOrCreateUserByEmail finds an existing user or creates one with an empty password hash (SSO users).
func (q *Queries) GetOrCreateUserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	err := q.db.QueryRow(ctx, `
		INSERT INTO users (email, password_hash)
		VALUES ($1, '')
		ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		RETURNING id, email, password_hash, github_id, is_admin, created_at`,
		email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.GithubID, &u.IsAdmin, &u.CreatedAt)
	return u, err
}

type SSOConfig struct {
	OrgID        pgtype.UUID
	IssuerURL    string
	ClientID     string
	ClientSecret string
	Domain       string
	Enabled      bool
}

func (q *Queries) GetSSOConfig(ctx context.Context, orgID pgtype.UUID) (SSOConfig, error) {
	var c SSOConfig
	err := q.db.QueryRow(ctx, `
		SELECT org_id, issuer_url, client_id, client_secret, domain, enabled
		FROM sso_configs WHERE org_id = $1`, orgID).
		Scan(&c.OrgID, &c.IssuerURL, &c.ClientID, &c.ClientSecret, &c.Domain, &c.Enabled)
	return c, err
}

func (q *Queries) GetSSOConfigByDomain(ctx context.Context, domain string) (SSOConfig, error) {
	var c SSOConfig
	err := q.db.QueryRow(ctx, `
		SELECT org_id, issuer_url, client_id, client_secret, domain, enabled
		FROM sso_configs WHERE domain = $1 AND enabled = true`, domain).
		Scan(&c.OrgID, &c.IssuerURL, &c.ClientID, &c.ClientSecret, &c.Domain, &c.Enabled)
	return c, err
}

func (q *Queries) GetSSOConfigByOrgSlug(ctx context.Context, slug string) (SSOConfig, error) {
	var c SSOConfig
	err := q.db.QueryRow(ctx, `
		SELECT s.org_id, s.issuer_url, s.client_id, s.client_secret, s.domain, s.enabled
		FROM sso_configs s
		JOIN orgs o ON o.id = s.org_id
		WHERE o.slug = $1 AND s.enabled = true`, slug).
		Scan(&c.OrgID, &c.IssuerURL, &c.ClientID, &c.ClientSecret, &c.Domain, &c.Enabled)
	return c, err
}

type UpsertSSOConfigParams struct {
	OrgID        pgtype.UUID
	IssuerURL    string
	ClientID     string
	ClientSecret string
	Domain       string
	Enabled      bool
}

func (q *Queries) UpsertSSOConfig(ctx context.Context, p UpsertSSOConfigParams) error {
	_, err := q.db.Exec(ctx, `
		INSERT INTO sso_configs (org_id, issuer_url, client_id, client_secret, domain, enabled, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (org_id) DO UPDATE SET
			issuer_url    = EXCLUDED.issuer_url,
			client_id     = EXCLUDED.client_id,
			client_secret = EXCLUDED.client_secret,
			domain        = EXCLUDED.domain,
			enabled       = EXCLUDED.enabled,
			updated_at    = NOW()`,
		p.OrgID, p.IssuerURL, p.ClientID, p.ClientSecret, p.Domain, p.Enabled,
	)
	return err
}
