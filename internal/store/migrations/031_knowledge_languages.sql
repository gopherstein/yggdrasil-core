-- The languages a knowledge source is written in (multilingual spec §19),
-- detected on this computer when it is indexed: JSON, each language with how
-- many passages are in it, most first.
ALTER TABLE knowledge_sources ADD COLUMN languages_json TEXT;
