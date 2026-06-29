-- DORA metrics support.
--
-- The four keys are derived entirely from existing data:
--   * Deployment frequency  -> count of `deploy` events
--   * Change failure rate    -> `deploy` events with status = 'failure'
--   * Lead time for changes  -> deploy.timestamp - metadata commit time (best effort)
--   * Time to restore        -> incidents.resolved_at - incidents.opened_at
--
-- These indexes keep the windowed aggregations cheap. Events are scoped to an
-- org through environments, so the deploy index leads with type+timestamp and
-- the environment join filters down from there.

CREATE INDEX IF NOT EXISTS idx_events_type_ts ON events(type, timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_incidents_org_resolved
    ON incidents(org_id, resolved_at)
    WHERE status = 'resolved' AND resolved_at IS NOT NULL;
