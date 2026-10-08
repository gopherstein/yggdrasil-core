-- People and roles (#206). The install's one implicit user becomes the
-- Owner, and what was theirs stays theirs: API keys, chats, memories,
-- files, and automations each name the person they belong to.
CREATE TABLE IF NOT EXISTS people (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    -- Set once the person signs in with a password; the Owner signs in on
    -- this computer without one.
    username TEXT UNIQUE COLLATE NOCASE,
    role TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'visitor')),
    password_hash TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    disabled_at TEXT
);

INSERT OR IGNORE INTO people (id, name, role) VALUES ('owner', 'Owner', 'owner');

ALTER TABLE api_keys ADD COLUMN person_id TEXT NOT NULL DEFAULT 'owner';
ALTER TABLE conversations ADD COLUMN person_id TEXT NOT NULL DEFAULT 'owner';
ALTER TABLE memories ADD COLUMN person_id TEXT NOT NULL DEFAULT 'owner';
ALTER TABLE automations ADD COLUMN person_id TEXT NOT NULL DEFAULT 'owner';
ALTER TABLE artifacts ADD COLUMN person_id TEXT NOT NULL DEFAULT 'owner';

CREATE INDEX IF NOT EXISTS conversations_person ON conversations(person_id, updated_at);
CREATE INDEX IF NOT EXISTS memories_person ON memories(person_id);
CREATE INDEX IF NOT EXISTS automations_person ON automations(person_id);
CREATE INDEX IF NOT EXISTS artifacts_person ON artifacts(person_id);
