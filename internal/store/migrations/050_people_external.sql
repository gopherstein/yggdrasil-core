-- The name a trusted reverse proxy signs a person in as (#206), such as
-- an email address; one person per name.
ALTER TABLE people ADD COLUMN external_id TEXT;
CREATE UNIQUE INDEX IF NOT EXISTS idx_people_external ON people(external_id) WHERE external_id IS NOT NULL;
