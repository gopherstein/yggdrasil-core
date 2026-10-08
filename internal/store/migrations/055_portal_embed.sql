-- The websites that may show a chat portal in a frame (#205), as a JSON
-- list of origins such as https://shop.example.com.
ALTER TABLE portals ADD COLUMN embed_origins TEXT NOT NULL DEFAULT '[]';
