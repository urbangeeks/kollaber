-- Allow 'teardown' events emitted by kube-watcher --report-deletes.
-- The original CHECK from 001_init.sql only permitted deploy/alert/note, so
-- inserting a teardown event failed at the database with a 500.
ALTER TABLE events DROP CONSTRAINT IF EXISTS events_type_check;
ALTER TABLE events ADD CONSTRAINT events_type_check
    CHECK (type IN ('deploy', 'alert', 'note', 'teardown'));
