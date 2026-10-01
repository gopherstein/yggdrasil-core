-- Daemon-owned scheduled prompts. automation_runs holds one row per occurrence.
-- Later slices claim those rows; this migration only stores them.

CREATE TABLE IF NOT EXISTS automations (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1 CHECK (enabled IN (0, 1)),
    schedule_json TEXT NOT NULL,
    time_zone TEXT NOT NULL,
    prompt TEXT NOT NULL,
    profile_id TEXT,
    tools_json TEXT NOT NULL DEFAULT '[]',
    notification_json TEXT NOT NULL DEFAULT '{"mode":"always"}',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    next_run_at TEXT,
    last_run_at TEXT
);

CREATE TABLE IF NOT EXISTS automation_runs (
    id TEXT PRIMARY KEY,
    automation_id TEXT NOT NULL REFERENCES automations(id) ON DELETE CASCADE,
    occurrence_at TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    claimed_at TEXT,
    lease_until TEXT,
    started_at TEXT,
    finished_at TEXT,
    result TEXT,
    error TEXT,
    notification_sent INTEGER NOT NULL DEFAULT 0 CHECK (notification_sent IN (0, 1)),
    model_id TEXT,
    node_id TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (automation_id, occurrence_at)
);

CREATE INDEX IF NOT EXISTS automations_due_idx ON automations (enabled, next_run_at);
