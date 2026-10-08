-- How long a chat portal keeps its visitors' conversations (#205), in
-- days; 0 keeps them.
ALTER TABLE portals ADD COLUMN retention_days INTEGER NOT NULL DEFAULT 30;
