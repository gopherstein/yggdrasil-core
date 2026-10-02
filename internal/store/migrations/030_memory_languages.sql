-- Cross-language memory (multilingual spec §18): the language a memory is
-- written in, detected on this computer, and a vector of its meaning from
-- the installed embedding model, so a question in one language finds a
-- memory written in another. content_hash and model_id tell when a vector
-- is stale.
ALTER TABLE memories ADD COLUMN language TEXT;

CREATE TABLE IF NOT EXISTS memory_vectors (
    memory_id TEXT PRIMARY KEY,
    model_id TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    vector BLOB NOT NULL
);
