-- Add tags column
ALTER TABLE notes
ADD COLUMN IF NOT EXISTS tags TEXT [] NOT NULL DEFAULT '{}';
-- Create an index on the tags column
CREATE INDEX IF NOT EXISTS note_tags_array_idx ON notes USING GIN (tags);