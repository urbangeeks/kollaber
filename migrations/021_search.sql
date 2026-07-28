-- Full-text search over the timeline. "Didn't we hit this before?" is the
-- question operational memory exists to answer, and until now the only way to
-- ask it was to scroll.
--
-- Both vectors are GENERATED ... STORED rather than trigger-maintained: the
-- expressions are immutable, so Postgres keeps them in step with the row for
-- free and they can never drift the way a forgotten trigger would.

-- Event text is the service and type, plus the *values* inside metadata.
-- jsonb_to_tsvector with a '["string","numeric"]' filter deliberately skips
-- JSON keys: indexing them would make a search for "version" match every
-- deploy ever shipped, since that is a key name on all of them.
ALTER TABLE events ADD COLUMN IF NOT EXISTS search_vector tsvector
    GENERATED ALWAYS AS (
        to_tsvector('english', coalesce(service, '') || ' ' || coalesce(type, '')) ||
        jsonb_to_tsvector('english', coalesce(metadata, '{}'::jsonb), '["string","numeric"]')
    ) STORED;

CREATE INDEX IF NOT EXISTS idx_events_search ON events USING GIN (search_vector);

-- Comments are the higher-value half: the metadata says what shipped, the
-- comments say what we concluded about it.
ALTER TABLE comments ADD COLUMN IF NOT EXISTS search_vector tsvector
    GENERATED ALWAYS AS (to_tsvector('english', body)) STORED;

CREATE INDEX IF NOT EXISTS idx_comments_search ON comments USING GIN (search_vector);

-- Search always orders by rank then recency, and always scopes to an org via
-- environments. This supports the recency half of that ordering once the GIN
-- index has narrowed the candidate set.
CREATE INDEX IF NOT EXISTS idx_events_env_timestamp ON events (environment_id, timestamp DESC);
