-- Mimir: connected knowledge. A source is a file, a folder, or text the user
-- supplied. Chunks are rebuilt from the source on refresh, so changing business
-- data never requires retraining a model.

CREATE TABLE IF NOT EXISTS knowledge_sources (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    path TEXT,
    signature TEXT,
    status TEXT NOT NULL DEFAULT 'ready',
    error TEXT,
    chunk_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    refreshed_at TEXT
);

CREATE VIRTUAL TABLE IF NOT EXISTS knowledge_fts USING fts5(
    title,
    body,
    source_id UNINDEXED,
    ordinal UNINDEXED,
    tokenize = 'unicode61 remove_diacritics 2 tokenchars ''/-'''
);

ALTER TABLE profiles ADD COLUMN knowledge_json TEXT;
