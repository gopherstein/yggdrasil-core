-- Retry and edit keep earlier versions (#447). A message's parent is the
-- message it follows; siblings with one parent are versions of that point.
-- Messages from before have no parent and follow the one before them, so
-- existing chats read as one version at every point.
ALTER TABLE messages ADD COLUMN parent_id TEXT;

CREATE INDEX IF NOT EXISTS messages_conversation ON messages (conversation_id);

-- The version shown after each point; '' is the chat's first message. A
-- point with none shows its latest version.
CREATE TABLE IF NOT EXISTS message_shown (
    conversation_id TEXT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    parent_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    PRIMARY KEY (conversation_id, parent_id)
);
