-- The model a scheduled prompt runs. The desktop is closed when the daemon
-- fires the task, so the choice is stored with the automation.

ALTER TABLE automations ADD COLUMN model_id TEXT;
