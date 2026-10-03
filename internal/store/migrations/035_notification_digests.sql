-- A destination can get a daily digest instead of each notice (#111):
-- {"at": "HH:MM", "time_zone": "Area/City"}, or NULL to send each notice.
ALTER TABLE notification_destinations ADD COLUMN digest_json TEXT;
