-- Bounded retries stay on the occurrence. A terminal failure is visible on the automation.
ALTER TABLE automation_runs ADD COLUMN attempt INTEGER NOT NULL DEFAULT 1;
ALTER TABLE automation_runs ADD COLUMN retry_at TEXT;

ALTER TABLE automations ADD COLUMN consecutive_failures INTEGER NOT NULL DEFAULT 0;
ALTER TABLE automations ADD COLUMN last_error TEXT;
