-- Expand org_members role constraint to 4-tier hierarchy
ALTER TABLE org_members DROP CONSTRAINT IF EXISTS org_members_role_check;
ALTER TABLE org_members ADD CONSTRAINT org_members_role_check
    CHECK (role IN ('owner', 'admin', 'member', 'viewer'));

-- Store intended role on invites so acceptors get the right role
ALTER TABLE invites ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'member'
    CHECK (role IN ('admin', 'member', 'viewer'));
