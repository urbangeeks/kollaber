-- Allow the 'rollback' and 'scale' event types that kube-watcher's
-- classifyChange has emitted since it learned to distinguish them from plain
-- deploys. The CHECK last widened in 015 to cover teardown, so those inserts
-- were rejected by the database and surfaced as a 500 from POST /events —
-- silently dropping every rollback and scale the watcher detected.
--
-- The API validator, UI icons, timeline filters, and client schemas all
-- already accept both types; only this constraint was missed.
ALTER TABLE events DROP CONSTRAINT IF EXISTS events_type_check;
ALTER TABLE events ADD CONSTRAINT events_type_check
    CHECK (type IN ('deploy', 'alert', 'note', 'teardown', 'rollback', 'scale'));
