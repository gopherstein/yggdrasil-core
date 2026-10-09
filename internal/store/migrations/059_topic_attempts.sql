-- Off-topic attempts (#345): messages an Enforce profile held, or whose
-- answer it replaced, kept as run records are.
CREATE TABLE IF NOT EXISTS topic_attempts (
    id TEXT PRIMARY KEY,
    at TEXT NOT NULL,
    profile_id TEXT NOT NULL,
    label TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT '',
    portal_id TEXT NOT NULL DEFAULT '',
    key_id TEXT NOT NULL DEFAULT '',
    person_id TEXT NOT NULL DEFAULT '',
    conversation_id TEXT NOT NULL DEFAULT '',
    message TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS topic_attempts_profile ON topic_attempts (profile_id, at);
