-- A chat portal's limits (#205): messages each visitor may send an hour
-- (0 for no limit), the longest message, and how many of its chats run at
-- once.
ALTER TABLE portals ADD COLUMN hourly_limit INTEGER NOT NULL DEFAULT 30;
ALTER TABLE portals ADD COLUMN max_message INTEGER NOT NULL DEFAULT 2000;
ALTER TABLE portals ADD COLUMN concurrency INTEGER NOT NULL DEFAULT 2;
