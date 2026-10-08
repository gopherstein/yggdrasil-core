-- Signing in (#206). A session is a browser signed in as a person; only
-- the SHA-256 of its cookie is kept. An invite is a one-time link that lets
-- a person choose a username and password, or a new password; only the
-- SHA-256 of its token is kept.
CREATE TABLE IF NOT EXISTS sessions (
    id_hash TEXT PRIMARY KEY,
    person_id TEXT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    user_agent TEXT,
    address TEXT
);
CREATE INDEX IF NOT EXISTS sessions_person ON sessions(person_id);

CREATE TABLE IF NOT EXISTS invites (
    token_hash TEXT PRIMARY KEY,
    person_id TEXT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    -- invite (first sign-in) or reset (a new password).
    kind TEXT NOT NULL CHECK (kind IN ('invite', 'reset')),
    created_by TEXT,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    used_at TEXT
);
CREATE INDEX IF NOT EXISTS invites_person ON invites(person_id);
