ALTER TABLE orgs
  ADD COLUMN IF NOT EXISTS plan                    TEXT        NOT NULL DEFAULT 'free',
  ADD COLUMN IF NOT EXISTS stripe_customer_id      TEXT,
  ADD COLUMN IF NOT EXISTS stripe_subscription_id  TEXT,
  ADD COLUMN IF NOT EXISTS subscription_status     TEXT        NOT NULL DEFAULT 'active',
  ADD COLUMN IF NOT EXISTS trial_ends_at           TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_orgs_stripe_customer_id     ON orgs(stripe_customer_id);
CREATE INDEX IF NOT EXISTS idx_orgs_stripe_subscription_id ON orgs(stripe_subscription_id);
