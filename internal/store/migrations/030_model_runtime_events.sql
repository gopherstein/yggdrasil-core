-- Model starts, failed starts, and crashes on this computer, for the
-- runtime observations a shared rating may include (#37). Kept 90 days.
CREATE TABLE IF NOT EXISTS model_runtime_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    model_id TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('start', 'start_failed', 'crash')),
    out_of_memory INTEGER NOT NULL DEFAULT 0,
    at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_model_runtime_events_model ON model_runtime_events(model_id, at);

-- Whether a shared rating includes how the model runs here.
ALTER TABLE model_ratings ADD COLUMN share_observations INTEGER NOT NULL DEFAULT 0;
