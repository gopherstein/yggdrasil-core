-- The built-in example AI is marked so the UI can explain each step.

ALTER TABLE specialized_ais ADD COLUMN is_example INTEGER NOT NULL DEFAULT 0;
