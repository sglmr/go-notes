// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0
// source: query.sql

package db

import (
	"context"
	"time"
)

const createNote = `-- name: CreateNote :one
insert into notes (
        id,
        title,
        note,
        archive,
        favorite,
        created_at,
        modified_at,
        tags
    )
values ($1, $2, $3, false, $4, $5, NOW(), $6)
returning id, title, note, archive, favorite, created_at, modified_at, tags
`

type CreateNoteParams struct {
	ID        string
	Title     string
	Note      string
	Favorite  bool
	CreatedAt time.Time
	Tags      []string
}

func (q *Queries) CreateNote(ctx context.Context, arg CreateNoteParams) (Note, error) {
	row := q.db.QueryRow(ctx, createNote,
		arg.ID,
		arg.Title,
		arg.Note,
		arg.Favorite,
		arg.CreatedAt,
		arg.Tags,
	)
	var i Note
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Note,
		&i.Archive,
		&i.Favorite,
		&i.CreatedAt,
		&i.ModifiedAt,
		&i.Tags,
	)
	return i, err
}

const deleteNote = `-- name: DeleteNote :exec
delete from notes
where id = $1
`

func (q *Queries) DeleteNote(ctx context.Context, id string) error {
	_, err := q.db.Exec(ctx, deleteNote, id)
	return err
}

const findNotesWithTags = `-- name: FindNotesWithTags :many
SELECT id, title, note, archive, favorite, created_at, modified_at, tags
FROM notes
WHERE tags @> $1::text []
ORDER BY modified_at DESC
`

func (q *Queries) FindNotesWithTags(ctx context.Context, dollar_1 []string) ([]Note, error) {
	rows, err := q.db.Query(ctx, findNotesWithTags, dollar_1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Note
	for rows.Next() {
		var i Note
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.Note,
			&i.Archive,
			&i.Favorite,
			&i.CreatedAt,
			&i.ModifiedAt,
			&i.Tags,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getNote = `-- name: GetNote :one
select id, title, note, archive, favorite, created_at, modified_at, tags
from notes
where id = $1
limit 1
`

func (q *Queries) GetNote(ctx context.Context, id string) (Note, error) {
	row := q.db.QueryRow(ctx, getNote, id)
	var i Note
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Note,
		&i.Archive,
		&i.Favorite,
		&i.CreatedAt,
		&i.ModifiedAt,
		&i.Tags,
	)
	return i, err
}

const getTagsWithCounts = `-- name: GetTagsWithCounts :many
SELECT tag_name, note_count
FROM tag_summary
ORDER BY tag_name
`

func (q *Queries) GetTagsWithCounts(ctx context.Context) ([]TagSummary, error) {
	rows, err := q.db.Query(ctx, getTagsWithCounts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []TagSummary
	for rows.Next() {
		var i TagSummary
		if err := rows.Scan(&i.TagName, &i.NoteCount); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const importNote = `-- name: ImportNote :one
insert into notes (
        id,
        title,
        note,
        archive,
        favorite,
        created_at,
        modified_at,
        tags
    )
values ($1, $2, $3, $4, $5, $6, $7, $8)
returning id, title, note, archive, favorite, created_at, modified_at, tags
`

type ImportNoteParams struct {
	ID         string
	Title      string
	Note       string
	Archive    bool
	Favorite   bool
	CreatedAt  time.Time
	ModifiedAt time.Time
	Tags       []string
}

func (q *Queries) ImportNote(ctx context.Context, arg ImportNoteParams) (Note, error) {
	row := q.db.QueryRow(ctx, importNote,
		arg.ID,
		arg.Title,
		arg.Note,
		arg.Archive,
		arg.Favorite,
		arg.CreatedAt,
		arg.ModifiedAt,
		arg.Tags,
	)
	var i Note
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Note,
		&i.Archive,
		&i.Favorite,
		&i.CreatedAt,
		&i.ModifiedAt,
		&i.Tags,
	)
	return i, err
}

const listAllNotes = `-- name: ListAllNotes :many
select id, title, note, archive, favorite, created_at, modified_at, tags
from notes
order by modified_at desc
`

func (q *Queries) ListAllNotes(ctx context.Context) ([]Note, error) {
	rows, err := q.db.Query(ctx, listAllNotes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Note
	for rows.Next() {
		var i Note
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.Note,
			&i.Archive,
			&i.Favorite,
			&i.CreatedAt,
			&i.ModifiedAt,
			&i.Tags,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listArchivedNotes = `-- name: ListArchivedNotes :many
select id, title, note, archive, favorite, created_at, modified_at, tags
from notes
where archive = TRUE
order by modified_at desc
`

func (q *Queries) ListArchivedNotes(ctx context.Context) ([]Note, error) {
	rows, err := q.db.Query(ctx, listArchivedNotes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Note
	for rows.Next() {
		var i Note
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.Note,
			&i.Archive,
			&i.Favorite,
			&i.CreatedAt,
			&i.ModifiedAt,
			&i.Tags,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listFavoriteNotes = `-- name: ListFavoriteNotes :many
select id, title, note, archive, favorite, created_at, modified_at, tags
from notes
where favorite = TRUE
order by modified_at desc
`

func (q *Queries) ListFavoriteNotes(ctx context.Context) ([]Note, error) {
	rows, err := q.db.Query(ctx, listFavoriteNotes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Note
	for rows.Next() {
		var i Note
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.Note,
			&i.Archive,
			&i.Favorite,
			&i.CreatedAt,
			&i.ModifiedAt,
			&i.Tags,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const listNotes = `-- name: ListNotes :many
select id, title, note, archive, favorite, created_at, modified_at, tags
from notes
where archive is FALSE
order by modified_at desc
`

func (q *Queries) ListNotes(ctx context.Context) ([]Note, error) {
	rows, err := q.db.Query(ctx, listNotes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Note
	for rows.Next() {
		var i Note
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.Note,
			&i.Archive,
			&i.Favorite,
			&i.CreatedAt,
			&i.ModifiedAt,
			&i.Tags,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const randomNote = `-- name: RandomNote :one
SELECT id, title, note, archive, favorite, created_at, modified_at, tags FROM notes
OFFSET floor(random() * (select count(*) from notes))
limit 1
`

func (q *Queries) RandomNote(ctx context.Context) (Note, error) {
	row := q.db.QueryRow(ctx, randomNote)
	var i Note
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Note,
		&i.Archive,
		&i.Favorite,
		&i.CreatedAt,
		&i.ModifiedAt,
		&i.Tags,
	)
	return i, err
}

const searchNotes = `-- name: SearchNotes :many
SELECT id, title, note, archive, favorite, created_at, modified_at, tags
FROM notes
WHERE (
        $1::text = ''
        OR ('id' || ' ' || title || ' ' || note) ILIKE '%' || $1::text || '%'
    )
    AND (
        ($2::text[])[1] = ''
        OR tags @> $2::text []
    )
    AND (archive IS FALSE OR $3::bool IS TRUE)
    AND (favorite IS TRUE or $4::bool IS FALSE)
ORDER BY CASE
        WHEN $1::text = '' THEN 3
        WHEN id ILIKE '%' || $1::text || '%' THEN 0
        WHEN title ILIKE '%' || $1::text || '%' THEN 1
        ELSE 2
    END,
    modified_at DESC
`

type SearchNotesParams struct {
	Query     string
	Tags      []string
	Archived  bool
	Favorites bool
}

func (q *Queries) SearchNotes(ctx context.Context, arg SearchNotesParams) ([]Note, error) {
	rows, err := q.db.Query(ctx, searchNotes,
		arg.Query,
		arg.Tags,
		arg.Archived,
		arg.Favorites,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Note
	for rows.Next() {
		var i Note
		if err := rows.Scan(
			&i.ID,
			&i.Title,
			&i.Note,
			&i.Archive,
			&i.Favorite,
			&i.CreatedAt,
			&i.ModifiedAt,
			&i.Tags,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const updateNote = `-- name: UpdateNote :one
update notes
set title = $2,
    note = $3,
    archive = $4,
    favorite = $5,
    created_at = $6,
    tags = $7,
    modified_at = NOW()
where id = $1
returning id, title, note, archive, favorite, created_at, modified_at, tags
`

type UpdateNoteParams struct {
	ID        string
	Title     string
	Note      string
	Archive   bool
	Favorite  bool
	CreatedAt time.Time
	Tags      []string
}

func (q *Queries) UpdateNote(ctx context.Context, arg UpdateNoteParams) (Note, error) {
	row := q.db.QueryRow(ctx, updateNote,
		arg.ID,
		arg.Title,
		arg.Note,
		arg.Archive,
		arg.Favorite,
		arg.CreatedAt,
		arg.Tags,
	)
	var i Note
	err := row.Scan(
		&i.ID,
		&i.Title,
		&i.Note,
		&i.Archive,
		&i.Favorite,
		&i.CreatedAt,
		&i.ModifiedAt,
		&i.Tags,
	)
	return i, err
}
