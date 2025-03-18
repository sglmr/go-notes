-- Enable the pg_trgm extension
create extension if not exists pg_trgm;

-- Create GIN indexes on both columns
create index if not exists notes_combined_trgm_idx on notes using GIN ((title || ' ' || note) gin_trgm_ops);