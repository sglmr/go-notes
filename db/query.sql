-- name: GetNote :one
select *
from notes
where id = $1
limit 1;
-- name: ListNotes :many
select *
from notes
where archive != TRUE
order by created_at desc;
-- name: ListAllNotes :many
select *
from notes;
-- name: ListFavoriteNotes :many
select *
from notes
where favorite = TRUE
order by modified_at desc;
-- name: ListArchivedNotes :many
select *
from notes
where archive = TRUE
order by created_at desc;
-- name: CreateNote :one
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
values ($1, $2, $3, $4, $5, $6, NOW(), $7)
returning *;
-- name: UpdateNote :one
update notes
set title = $2,
    note = $3,
    archive = $4,
    favorite = $5,
    created_at = $6,
    tags = $7,
    modified_at = NOW()
where id = $1
returning *;
-- name: UpdateNoteTags :one
update notes
set tags = $2,
    modified_at = NOW()
where id = $1
returning *;
-- name: DeleteNote :exec
delete from notes
where id = $1;
-- name: SearchNotes :many
SELECT *
FROM notes
WHERE (
        @query::text = ''
        OR ('id' || ' ' || title || ' ' || note) ILIKE '%' || @query::text || '%'
    )
    AND (
        @tags::text [] = '{""}' -- This matches the Go []string{""}
        OR @tags::text [] = '{}'
        OR tags @> @tags::text []
    )
    AND (archive = @archived::bool)
    AND (
        favorite = @favorites::bool
        OR @favorites::bool = FALSE
    )
ORDER BY created_at DESC;
-- name: FindNotesWithTags :many
SELECT *
FROM notes
WHERE tags @> $1::text []
ORDER BY created_at DESC;
-- name: GetTagsWithCounts :many
SELECT *
FROM tag_summary
ORDER BY tag_name;
-- name: ImportNote :one
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
returning *;
-- name: RandomNote :one
SELECT *
FROM notes OFFSET floor(
        random() * (
            select count(*)
            from notes
        )
    )
limit 1;
-- name: ArchiveNote :exec
update notes
set archive = TRUE
where id = $1;