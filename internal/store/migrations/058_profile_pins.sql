-- Pinning profiles to people and roles (#345). A person's own list, when
-- set, replaces their role's: NULL is the role's, [] is any profile.
ALTER TABLE people ADD COLUMN profiles_json TEXT;

CREATE TABLE IF NOT EXISTS role_profiles (
    role TEXT PRIMARY KEY,
    profiles_json TEXT NOT NULL
);
