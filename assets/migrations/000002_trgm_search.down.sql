-- Drop GIN indexes
drop index if exists notes_combined_trgm_idx;

-- Disable the pg_trgm extension
drop extension if exists pg_trgm;