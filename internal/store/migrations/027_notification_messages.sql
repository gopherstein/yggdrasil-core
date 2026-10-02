-- A notification's message (multilingual spec §22): its title and body as
-- catalog keys with the values they need, as JSON, so each place that shows
-- it writes it in its own language. title and body keep the English text for
-- logs and older clients.
ALTER TABLE notifications ADD COLUMN message TEXT;
