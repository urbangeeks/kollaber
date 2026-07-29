-- Promote a comment to a decision.
--
-- "We're rolling back" and "accepting this risk until Q3" are the highest-value
-- rows in the comments table and are currently indistinguishable from "looking
-- into it". Marking them is what turns a thread into the answer to "what did we
-- decide about it?" six months later.
--
-- decided_by and decided_at record who promoted the comment, which is not
-- necessarily who wrote it: the decision is the author's, the curation is
-- somebody's later act of noticing it mattered.
ALTER TABLE comments
  ADD COLUMN IF NOT EXISTS is_decision BOOLEAN     NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS decided_by  UUID        REFERENCES users(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS decided_at  TIMESTAMPTZ;

-- Partial index: the decisions view reads only the marked rows, and they are a
-- small fraction of a busy org's comments.
CREATE INDEX IF NOT EXISTS idx_comments_decisions
  ON comments (created_at DESC)
  WHERE is_decision;
