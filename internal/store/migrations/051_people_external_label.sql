-- How a person signed in elsewhere is shown (#206): their name at a
-- trusted proxy, or their email at an OpenID Connect provider, whose
-- external_id is an opaque subject.
ALTER TABLE people ADD COLUMN external_label TEXT;
