-- Train Your Own AI. A specialized AI is a base model, optional trained
-- adapter revisions, system instructions, and connected knowledge sources.

CREATE TABLE IF NOT EXISTS specialized_ais (
    id TEXT PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    goal TEXT NOT NULL DEFAULT '',
    instructions TEXT NOT NULL DEFAULT '',
    base_model_id TEXT NOT NULL DEFAULT '',
    preset TEXT NOT NULL DEFAULT 'balanced',
    advanced_json TEXT,
    knowledge_json TEXT,
    deployed_revision INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS training_materials (
    id TEXT PRIMARY KEY,
    ai_id TEXT NOT NULL REFERENCES specialized_ais(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    filename TEXT,
    use TEXT NOT NULL,
    recommendation_json TEXT NOT NULL,
    warning TEXT,
    knowledge_source_id TEXT,
    example_count INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS training_examples (
    id TEXT PRIMARY KEY,
    ai_id TEXT NOT NULL REFERENCES specialized_ais(id) ON DELETE CASCADE,
    material_id TEXT REFERENCES training_materials(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    messages_json TEXT NOT NULL,
    excluded INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_training_examples_ai ON training_examples(ai_id, position);

CREATE TABLE IF NOT EXISTS training_jobs (
    id TEXT PRIMARY KEY,
    ai_id TEXT NOT NULL REFERENCES specialized_ais(id) ON DELETE CASCADE,
    revision INTEGER NOT NULL,
    node_id TEXT NOT NULL,
    node_name TEXT,
    backend TEXT NOT NULL,
    state TEXT NOT NULL,
    progress_json TEXT,
    hyper_json TEXT NOT NULL,
    error TEXT,
    created_at TEXT NOT NULL,
    started_at TEXT,
    finished_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_training_jobs_ai ON training_jobs(ai_id, created_at);

CREATE TABLE IF NOT EXISTS ai_revisions (
    ai_id TEXT NOT NULL REFERENCES specialized_ais(id) ON DELETE CASCADE,
    revision INTEGER NOT NULL,
    job_id TEXT NOT NULL,
    base_model_id TEXT NOT NULL,
    backend TEXT NOT NULL,
    hyper_json TEXT NOT NULL,
    example_count INTEGER NOT NULL,
    examples_hash TEXT NOT NULL,
    adapter_path TEXT NOT NULL,
    final_train_loss REAL,
    final_val_loss REAL,
    created_at TEXT NOT NULL,
    PRIMARY KEY (ai_id, revision)
);

CREATE TABLE IF NOT EXISTS eval_prompts (
    id TEXT PRIMARY KEY,
    ai_id TEXT NOT NULL REFERENCES specialized_ais(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    prompt TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS eval_runs (
    id TEXT PRIMARY KEY,
    ai_id TEXT NOT NULL REFERENCES specialized_ais(id) ON DELETE CASCADE,
    revision INTEGER NOT NULL,
    status TEXT NOT NULL,
    results_json TEXT,
    error TEXT,
    created_at TEXT NOT NULL,
    finished_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_eval_runs_ai ON eval_runs(ai_id, revision, created_at);
