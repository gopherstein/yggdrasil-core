-- A file that belongs to no chat, such as one an automation run or an API
-- call made, or an upload never sent, expires (#191). Attaching it to a chat
-- clears expires_at. Files already unfiled get the same week from now.
ALTER TABLE artifacts ADD COLUMN expires_at TEXT;
UPDATE artifacts SET expires_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now', '+7 days') WHERE conversation_id IS NULL;
CREATE INDEX artifacts_expires ON artifacts(expires_at) WHERE expires_at IS NOT NULL;
