-- Invitations to a chat portal (#205): a one-time link that makes its
-- browser the named visitor an Admin invited. SQLite can't change a CHECK,
-- so the table is made again with the new kind.
CREATE TABLE invites_new (
    token_hash TEXT PRIMARY KEY,
    person_id TEXT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    -- invite (first sign-in), reset (a new password), or portal (a chat
    -- portal's invited visitor).
    kind TEXT NOT NULL CHECK (kind IN ('invite', 'reset', 'portal')),
    created_by TEXT,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    used_at TEXT
);
INSERT INTO invites_new (token_hash, person_id, kind, created_by, created_at, expires_at, used_at)
    SELECT token_hash, person_id, kind, created_by, created_at, expires_at, used_at FROM invites;
DROP TABLE invites;
ALTER TABLE invites_new RENAME TO invites;
CREATE INDEX IF NOT EXISTS invites_person ON invites(person_id);
-- A portal's visitor an Admin invited by name, as opposed to an anonymous
-- one.
ALTER TABLE people ADD COLUMN invited INTEGER NOT NULL DEFAULT 0;
