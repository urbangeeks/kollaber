-- Alertmanager de-duplication lookup: the ingest path asks for the most recent
-- alert event in an environment carrying a given fingerprint on every webhook
-- delivery, and Alertmanager re-delivers firing alerts on each repeat_interval.
-- Partial so it only covers rows that actually carry a fingerprint.
CREATE INDEX IF NOT EXISTS idx_events_alert_fingerprint
    ON events (environment_id, (metadata->>'fingerprint'), timestamp DESC)
    WHERE type = 'alert' AND metadata->>'fingerprint' IS NOT NULL;
