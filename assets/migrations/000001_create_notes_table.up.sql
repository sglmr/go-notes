-- Create the notes table
CREATE TABLE notes (
    id TEXT PRIMARY KEY CHECK (id ~ '^n_'),
    title TEXT NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    archive BOOLEAN NOT NULL DEFAULT FALSE,
    favorite BOOLEAN NOT NULL DEFAULT FALSE, 
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    modified_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create indexes
CREATE INDEX notes_archive_idx ON notes (archive);
CREATE INDEX notes_favorite_idx ON notes (favorite);
CREATE INDEX notes_created_at_idx ON notes (created_at desc);
CREATE INDEX notes_modified_at_idx ON notes (modified_at desc);