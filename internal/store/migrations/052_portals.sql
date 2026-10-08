-- Chat portals (#205): a branded chat page for the people an Admin serves,
-- with its own profile, tools, memory, and access. A portal's anonymous
-- visitors are guest people who belong to it (people.portal_id), so their
-- chats are private to each of them like anyone's.
CREATE TABLE IF NOT EXISTS portals (
    id            TEXT PRIMARY KEY,
    slug          TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    profile_id    TEXT NOT NULL DEFAULT '',
    tools         TEXT NOT NULL DEFAULT 'none',
    memory        INTEGER NOT NULL DEFAULT 0,
    language      TEXT NOT NULL DEFAULT '',
    access        TEXT NOT NULL DEFAULT 'passcode',
    passcode_hash TEXT,
    branding      TEXT NOT NULL DEFAULT '{}',
    enabled       INTEGER NOT NULL DEFAULT 1,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
);
ALTER TABLE people ADD COLUMN portal_id TEXT;
CREATE INDEX IF NOT EXISTS idx_people_portal ON people(portal_id);
