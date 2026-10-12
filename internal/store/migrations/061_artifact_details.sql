-- How a file was made, as JSON: for a picture or clip, its model, quality,
-- seed, size, and prompt, so it can be made again (#553). Files from before
-- have none.
ALTER TABLE artifacts ADD COLUMN details TEXT;
