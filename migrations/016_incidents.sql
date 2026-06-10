CREATE TABLE IF NOT EXISTS incidents (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    title       TEXT NOT NULL,
    severity    TEXT NOT NULL DEFAULT 'sev3'
                CONSTRAINT incidents_severity_check CHECK (severity IN ('sev1', 'sev2', 'sev3', 'sev4')),
    status      TEXT NOT NULL DEFAULT 'open'
                CONSTRAINT incidents_status_check CHECK (status IN ('open', 'mitigated', 'resolved')),
    owner_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    opened_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_incidents_org_status ON incidents(org_id, status);

-- Link events to an incident (many events -> one incident). Detaching an
-- incident leaves the events intact.
ALTER TABLE events
  ADD COLUMN IF NOT EXISTS incident_id UUID REFERENCES incidents(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_events_incident ON events(incident_id);
