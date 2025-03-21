-- Drop the tag summary view
DROP VIEW IF EXISTS tag_summary;
-- Drop the index on the tags column
DROP INDEX IF EXISTS note_tags_array_idx;
-- Drop the tags column
ALTER TABLE IF EXISTS notes DROP COLUMN IF EXISTS tags;