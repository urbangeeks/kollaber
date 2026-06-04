-- Per-org monthly usage counter for the AI timeline agent, used to enforce a
-- cost cap. period is a UTC calendar month, 'YYYY-MM'.
CREATE TABLE IF NOT EXISTS ai_usage (
    org_id  UUID    NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    period  TEXT    NOT NULL,
    count   INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (org_id, period)
);
