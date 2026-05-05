ALTER TABLE events
  ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'success'
  CONSTRAINT events_status_check CHECK (status IN ('success', 'failure', 'in_progress'));

UPDATE events SET status = 'in_progress'
WHERE id = '00000000-0000-0000-0000-000000000101';
