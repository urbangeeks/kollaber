CREATE TABLE IF NOT EXISTS sso_configs (
    org_id        UUID        PRIMARY KEY REFERENCES orgs(id) ON DELETE CASCADE,
    issuer_url    TEXT        NOT NULL,
    client_id     TEXT        NOT NULL,
    client_secret TEXT        NOT NULL,
    domain        TEXT        NOT NULL,
    enabled       BOOLEAN     NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS sso_configs_domain ON sso_configs (domain) WHERE enabled = true;
