-- Newest occurrence first when reading an automation's run history.
CREATE INDEX IF NOT EXISTS automation_runs_history_idx
    ON automation_runs (automation_id, occurrence_at DESC);
