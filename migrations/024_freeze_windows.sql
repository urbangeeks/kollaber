-- Change freeze windows: Black Friday, quarter end, the week the payments
-- migration lands.
--
-- Kollaber does not block anything. A freeze is a declaration the org made
-- about itself, and a deploy that lands inside one is recorded as having done
-- so — the CLI exits non-zero so CI can decide, and the timeline says plainly
-- that this shipped during a freeze. Blocking would put us on the critical path
-- of every deploy, which is a promise an observability-adjacent tool should not
-- make.
--
-- environment_id NULL means every environment in the org, which is what a
-- company-wide Black Friday freeze actually is. A row naming one environment
-- freezes only that one.
CREATE TABLE IF NOT EXISTS freeze_windows (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id         UUID        NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    environment_id UUID        REFERENCES environments(id) ON DELETE CASCADE,
    reason         TEXT        NOT NULL,
    starts_at      TIMESTAMPTZ NOT NULL,
    ends_at        TIMESTAMPTZ NOT NULL,
    created_by     UUID        REFERENCES users(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT freeze_windows_period_check CHECK (ends_at > starts_at)
);

-- Every write path asks the same question on every change event: "is this org
-- and environment frozen right now?"
CREATE INDEX IF NOT EXISTS idx_freeze_windows_lookup
  ON freeze_windows (org_id, starts_at, ends_at);
