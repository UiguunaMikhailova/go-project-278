-- name: GetLink :one
SELECT * FROM links WHERE id = $1;

-- name: GetLinkByShortName :one
SELECT * FROM links WHERE short_name = $1;

-- name: CreateLink :one
INSERT INTO links (original_url, short_name)
VALUES ($1, $2)
RETURNING *;

-- name: UpdateLink :one
UPDATE links
SET original_url = $2, short_name = $3
WHERE id = $1
RETURNING *;

-- name: DeleteLink :execrows
DELETE FROM links WHERE id = $1;

-- name: ListLinksPage :many
SELECT * FROM links ORDER BY id LIMIT $1 OFFSET $2;

-- name: CountLinks :one
SELECT count(*) FROM links;

-- name: CreateLinkVisit :one
INSERT INTO link_visits (link_id, ip, user_agent, referer, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: ListLinkVisitsPage :many
SELECT * FROM link_visits ORDER BY id LIMIT $1 OFFSET $2;

-- name: CountLinkVisits :one
SELECT count(*) FROM link_visits;
