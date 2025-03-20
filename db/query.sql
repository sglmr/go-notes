-- name: GetNote :one
select *
from notes
where id = $1
limit 1;
-- name: ListNotes :many
select *
from notes
where archive is FALSE
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
        modified_at
    )
values ($1, $2, $3, false, $4, $5, NOW())
returning *;
-- name: UpdateNote :one
update notes
set title = $2,
    note = $3,
    archive = $4,
    favorite = $5,
    created_at = $6,
    modified_at = NOW()
where id = $1
returning *;
-- name: DeleteNote :exec
delete from notes
where id = $1;
-- name: SearchNotes :many
SELECT *
FROM notes
WHERE ('id' || ' ' || title || ' ' || note) ILIKE '%' || @query::text || '%'
ORDER BY CASE
        WHEN id ILIKE '%' || @query::text || '%' THEN 0
        WHEN title ILIKE '%' || @query::text || '%' THEN 1
        ELSE 2
    END,
    modified_at DESC;