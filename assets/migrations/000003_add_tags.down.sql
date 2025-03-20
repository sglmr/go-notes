-- Create an index on the tags column
DROP INDEX IF EXISTS note_tags_array_idx;
-- Add tags column
ALTER TABLE notes DROP COLUMN IF EXISTS tags;