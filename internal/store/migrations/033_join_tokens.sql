-- One-line join tokens (#40). The token itself is shown once and never
-- stored: proof_key is derived from its secret and only lets this computer
-- check a joining computer's proof that it holds the token.
CREATE TABLE IF NOT EXISTS join_tokens (
    id TEXT PRIMARY KEY,
    proof_key TEXT NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    used_at TEXT,
    used_by TEXT,
    revoked_at TEXT
);
