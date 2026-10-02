-- Community model ratings (#37). A person's own rating of each model stays
-- here; remote_key is set while it is shared with the ratings service.
CREATE TABLE IF NOT EXISTS model_ratings (
    model_id TEXT PRIMARY KEY,
    stars INTEGER NOT NULL CHECK (stars BETWEEN 1 AND 5),
    tags TEXT NOT NULL DEFAULT '[]',
    remote_key TEXT,
    shared_at TEXT,
    updated_at TEXT NOT NULL
);

-- Models the person asked not to be asked about again.
CREATE TABLE IF NOT EXISTS model_rating_prompts (
    model_id TEXT PRIMARY KEY,
    dismissed_at TEXT NOT NULL
);

-- The last public ratings summary, so ratings show while offline.
CREATE TABLE IF NOT EXISTS ratings_summary (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    body TEXT NOT NULL,
    source TEXT NOT NULL,
    fetched_at TEXT NOT NULL
);
