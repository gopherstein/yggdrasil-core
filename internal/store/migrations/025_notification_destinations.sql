-- Gjallarhorn destinations: email (your own SMTP server) and webhooks.
-- config_json holds what is safe to show (host, port, sender, recipients,
-- URL); passwords and webhook secrets are kept in the secrets directory.
-- categories_json lists the categories a destination receives (empty: all),
-- and min_severity the lowest severity it receives.

CREATE TABLE IF NOT EXISTS notification_destinations (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    config_json TEXT NOT NULL DEFAULT '{}',
    categories_json TEXT,
    min_severity TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- A delivery is retried on its own, without rerunning the work that made the
-- notification: next_attempt_at is when, and status pending or held says it
-- is waiting (held: quiet hours).
ALTER TABLE notification_deliveries ADD COLUMN destination_id TEXT;
ALTER TABLE notification_deliveries ADD COLUMN first_attempt_at TEXT;
ALTER TABLE notification_deliveries ADD COLUMN next_attempt_at TEXT;

CREATE INDEX IF NOT EXISTS notification_deliveries_due ON notification_deliveries(status, next_attempt_at);

-- Repeats of the same notice are counted on the first one.
ALTER TABLE notifications ADD COLUMN repeat_count INTEGER NOT NULL DEFAULT 1;
