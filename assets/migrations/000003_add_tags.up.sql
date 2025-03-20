-- Add tags column
ALTER TABLE notes
ADD COLUMN IF NOT EXISTS tags TEXT [] NOT NULL DEFAULT '{}';
-- Create an index on the tags column
CREATE INDEX IF NOT EXISTS note_tags_array_idx ON notes USING GIN (tags);
-- Create a view to summarize the tags and counts
CREATE OR REPLACE VIEW tag_summary AS
SELECT unnest(tags) AS tag_name,
    COUNT(*) AS note_count
FROM notes
WHERE tags IS NOT NULL
    AND array_length(tags, 1) > 0
GROUP BY tag_name
ORDER BY note_count DESC,
    tag_name;
/* Someday later... look into materialized views if
 tag_summary has low performance */