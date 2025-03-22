-- name: GetNote :one
select *
from notes
where id = $1
limit 1;
-- name: ListNotes :many
select *
from notes
where archive != TRUE
order by modified_at desc;
-- name: ListFavoriteNotes :many
select *
from notes
where favorite = TRUE
order by modified_at desc;
-- name: ListArchivedNotes :many
select *
from notes
where archive = TRUE
order by modified_at desc;
-- name: ListAllNotes :many
select *
from notes
order by modified_at desc;
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
values ($1, $2, $3, false, $4, $5, NOW(), $6)
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
        (@tags::text[])[1] = ''
        OR tags @> @tags::text []
    )
    AND (archive IS FALSE OR @archived::bool IS TRUE)
    AND (favorite IS TRUE or @favorites::bool IS FALSE)
ORDER BY CASE
        WHEN @query::text = '' THEN 3
        WHEN id ILIKE '%' || @query::text || '%' THEN 0
        WHEN title ILIKE '%' || @query::text || '%' THEN 1
        ELSE 2
    END,
    modified_at DESC;
-- name: FindNotesWithTags :many
SELECT *
FROM notes
WHERE tags @> $1::text []
ORDER BY modified_at DESC;
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
SELECT * FROM notes
OFFSET floor(random() * (select count(*) from notes))
limit 1;
-- name: ArchiveNote :exec
update notes
set archive = TRUE
where id = $1;