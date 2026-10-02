-- The language a person used a model in when they rated it (multilingual
-- spec §23), such as es; '' when they did not say.
ALTER TABLE model_ratings ADD COLUMN language TEXT NOT NULL DEFAULT '';
