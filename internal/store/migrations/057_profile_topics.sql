-- A profile's topic controls (#345): what the assistant stays on, what it
-- never discusses, the reply to anything else, and how strictly.
ALTER TABLE profiles ADD COLUMN topics_json TEXT;
